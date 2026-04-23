package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// adminRequest is the unified request body for all /admin/* endpoints.
// Fields used depend on the endpoint; unused fields are ignored.
type adminRequest struct {
	// shared
	Platform    string `json:"platform"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Nickname    string `json:"nickname"`

	// workspace-add
	WorkspaceName       string   `json:"workspace_name"`
	BotToken            string   `json:"bot_token"`
	SigningSecret       string   `json:"signing_secret"`
	AllowedChannels     []string `json:"allowed_channels,omitempty"`
	ConnectCodeTTLHours int      `json:"connect_code_ttl_hours,omitempty"`

	// register
	InstanceID     string   `json:"instance_id"`
	RoleARN        string   `json:"role_arn"`
	DNSName        string   `json:"dns_name,omitempty"`
	TagPrefix      string   `json:"tag_prefix,omitempty"`
	AllowedActions []string `json:"allowed_actions"`

	// set-enabled
	Enabled bool `json:"enabled"`
}

// callerAccountID extracts the AWS account ID from an IAM ARN.
// ARN format: arn:aws:iam::123456789012:user/alice  or  arn:aws:sts::123456789012:assumed-role/...
func callerAccountID(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}

// handleAdmin routes /admin/* requests. callerARN is extracted from the
// API Gateway request context (populated by AWS when AuthType: AWS_IAM).
func handleAdmin(ctx context.Context, reg *Registry, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	callerARN := request.RequestContext.Authorizer.IAM.UserARN
	if callerARN == "" {
		return adminError(403, "IAM identity required"), nil
	}

	path := strings.TrimRight(request.RawPath, "/")
	method := request.RequestContext.HTTP.Method

	var body adminRequest
	if request.Body != "" {
		if err := json.Unmarshal([]byte(request.Body), &body); err != nil {
			return adminError(400, "invalid request body"), nil
		}
	}

	// Populate tag prefix from header if not in body (prism sends X-Prism-Tag-Prefix)
	if body.TagPrefix == "" {
		if tp := request.Headers["x-prism-tag-prefix"]; tp != "" {
			body.TagPrefix = tp
		}
	}
	if body.TagPrefix == "" {
		body.TagPrefix = "spawn"
	}

	switch {
	case path == "/admin/workspace-add" && method == "POST":
		return adminWorkspaceAdd(ctx, reg, body, callerARN)
	case path == "/admin/workspace-list" && method == "GET":
		return adminWorkspaceList(ctx, reg, request.QueryStringParameters, callerARN)
	case path == "/admin/register" && method == "POST":
		return adminRegister(ctx, reg, body, callerARN)
	case path == "/admin/set-enabled" && method == "POST":
		return adminSetEnabled(ctx, reg, body, callerARN)
	case path == "/admin/deregister" && method == "POST":
		return adminDeregister(ctx, reg, body, callerARN)
	case path == "/admin/list" && method == "GET":
		return adminList(ctx, reg, request.QueryStringParameters, callerARN)
	default:
		return adminError(404, fmt.Sprintf("unknown admin route: %s %s", method, path)), nil
	}
}

func adminWorkspaceAdd(ctx context.Context, reg *Registry, r adminRequest, callerARN string) (events.APIGatewayProxyResponse, error) {
	if r.Platform == "" || r.WorkspaceID == "" || r.BotToken == "" || r.SigningSecret == "" {
		return adminError(400, "platform, workspace_id, bot_token, and signing_secret are required"), nil
	}

	ws := &WorkspaceConfig{
		WorkspaceKey:        workspaceKey(r.Platform, r.WorkspaceID),
		Platform:            r.Platform,
		BotToken:            r.BotToken,
		SigningSecret:       r.SigningSecret,
		WorkspaceName:       r.WorkspaceName,
		AllowedChannels:     r.AllowedChannels,
		ConnectCodeTTLHours: r.ConnectCodeTTLHours,
		InstalledBy:         callerARN,
		InstalledAt:         time.Now().UTC().Format(time.RFC3339),
	}
	if err := reg.PutWorkspace(ctx, ws); err != nil {
		return adminError(500, fmt.Sprintf("store workspace: %v", err)), nil
	}

	return adminOK(map[string]string{
		"workspace_key": ws.WorkspaceKey,
		"workspace_name": ws.WorkspaceName,
		"installed_by":  callerARN,
	})
}

func adminWorkspaceList(ctx context.Context, reg *Registry, params map[string]string, callerARN string) (events.APIGatewayProxyResponse, error) {
	platform := params["platform"]
	workspaceID := params["workspace_id"]
	if platform == "" || workspaceID == "" {
		return adminError(400, "platform and workspace_id query params are required"), nil
	}

	ws, err := reg.GetWorkspace(ctx, platform, workspaceID)
	if err != nil {
		return adminError(404, "workspace not found"), nil
	}

	// Verify the caller's account installed this workspace.
	if callerAccountID(ws.InstalledBy) != callerAccountID(callerARN) {
		return adminError(403, "workspace not found"), nil // intentionally vague
	}

	// Return workspace metadata only — never return bot_token or signing_secret
	return adminOK(map[string]interface{}{
		"workspace_key":  ws.WorkspaceKey,
		"platform":       ws.Platform,
		"workspace_name": ws.WorkspaceName,
		"installed_by":   ws.InstalledBy,
		"installed_at":   ws.InstalledAt,
		"allowed_channels": ws.AllowedChannels,
		"connect_code_ttl_hours": ws.ConnectCodeTTLHours,
		"has_incoming_webhook": ws.IncomingWebhookURL != "",
		"token_rotation": ws.TokenRotation,
	})
}

func adminRegister(ctx context.Context, reg *Registry, r adminRequest, callerARN string) (events.APIGatewayProxyResponse, error) {
	if r.Platform == "" || r.WorkspaceID == "" || r.UserID == "" ||
		r.InstanceID == "" || r.Nickname == "" || r.RoleARN == "" {
		return adminError(400, "platform, workspace_id, user_id, instance_id, nickname, and role_arn are required"), nil
	}
	if err := verifyWorkspaceOwner(ctx, reg, r.Platform, r.WorkspaceID, callerARN); err != nil {
		return adminError(403, err.Error()), nil
	}
	if len(r.AllowedActions) == 0 {
		r.AllowedActions = []string{"status"}
	}

	registration := &BotRegistration{
		UserKey:        userKey(r.Platform, r.WorkspaceID, r.UserID),
		Nickname:       r.Nickname,
		InstanceID:     r.InstanceID,
		RoleARN:        r.RoleARN,
		DNSName:        r.DNSName,
		TagPrefix:      r.TagPrefix,
		AllowedActions: r.AllowedActions,
		RegisteredBy:   callerARN,
		Platform:       r.Platform,
		Enabled:        false, // must be explicitly enabled
	}
	if err := reg.PutRegistration(ctx, registration); err != nil {
		return adminError(500, fmt.Sprintf("store registration: %v", err)), nil
	}

	return adminOK(map[string]interface{}{
		"user_key":        registration.UserKey,
		"nickname":        registration.Nickname,
		"instance_id":     registration.InstanceID,
		"allowed_actions": registration.AllowedActions,
		"enabled":         registration.Enabled,
		"registered_by":   callerARN,
		"note":            "Registration created. Run set-enabled with enabled:true to grant access.",
	})
}

func adminSetEnabled(ctx context.Context, reg *Registry, r adminRequest, callerARN string) (events.APIGatewayProxyResponse, error) {
	if r.Platform == "" || r.WorkspaceID == "" || r.UserID == "" || r.Nickname == "" {
		return adminError(400, "platform, workspace_id, user_id, and nickname are required"), nil
	}
	if err := verifyWorkspaceOwner(ctx, reg, r.Platform, r.WorkspaceID, callerARN); err != nil {
		return adminError(403, err.Error()), nil
	}

	if err := reg.SetEnabled(ctx, r.Platform, r.WorkspaceID, r.UserID, r.Nickname, r.Enabled); err != nil {
		return adminError(500, fmt.Sprintf("set enabled: %v", err)), nil
	}

	state := "disabled"
	if r.Enabled {
		state = "enabled"
	}
	return adminOK(map[string]interface{}{
		"user_key": userKey(r.Platform, r.WorkspaceID, r.UserID),
		"nickname": r.Nickname,
		"enabled":  r.Enabled,
		"state":    state,
	})
}

func adminDeregister(ctx context.Context, reg *Registry, r adminRequest, callerARN string) (events.APIGatewayProxyResponse, error) {
	if r.Platform == "" || r.WorkspaceID == "" || r.UserID == "" || r.Nickname == "" {
		return adminError(400, "platform, workspace_id, user_id, and nickname are required"), nil
	}
	if err := verifyWorkspaceOwner(ctx, reg, r.Platform, r.WorkspaceID, callerARN); err != nil {
		return adminError(403, err.Error()), nil
	}

	if err := reg.DeleteRegistration(ctx, r.Platform, r.WorkspaceID, r.UserID, r.Nickname); err != nil {
		return adminError(500, fmt.Sprintf("delete registration: %v", err)), nil
	}

	return adminOK(map[string]string{"deleted": userKey(r.Platform, r.WorkspaceID, r.UserID) + "#" + r.Nickname})
}

func adminList(ctx context.Context, reg *Registry, params map[string]string, callerARN string) (events.APIGatewayProxyResponse, error) {
	platform := params["platform"]
	workspaceID := params["workspace_id"]
	userID := params["user_id"]

	if platform == "" || workspaceID == "" {
		return adminError(400, "platform and workspace_id query params are required"), nil
	}
	if err := verifyWorkspaceOwner(ctx, reg, platform, workspaceID, callerARN); err != nil {
		return adminError(403, err.Error()), nil
	}

	var regs []BotRegistration
	var err error
	if userID != "" {
		regs, err = reg.ListUserInstances(ctx, platform, workspaceID, userID)
	} else {
		regs, err = reg.ListWorkspaceRegistrations(ctx, platform, workspaceID)
	}
	if err != nil {
		return adminError(500, fmt.Sprintf("list registrations: %v", err)), nil
	}

	return adminOK(map[string]interface{}{
		"registrations": regs,
		"count":         len(regs),
	})
}

// verifyWorkspaceOwner checks that the caller's AWS account installed the workspace.
// Returns an error if the workspace doesn't exist or belongs to a different account.
// Returns a deliberately vague "workspace not found" for the account mismatch case
// to avoid confirming that a workspace exists to unauthorised callers.
func verifyWorkspaceOwner(ctx context.Context, reg *Registry, platform, workspaceID, callerARN string) error {
	ws, err := reg.GetWorkspace(ctx, platform, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found")
	}
	if callerAccountID(ws.InstalledBy) != callerAccountID(callerARN) {
		return fmt.Errorf("workspace not found")
	}
	return nil
}

func adminOK(payload interface{}) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(payload)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}, nil
}

func adminError(code int, msg string) events.APIGatewayProxyResponse {
	body, _ := json.Marshal(map[string]string{"error": msg})
	return events.APIGatewayProxyResponse{
		StatusCode: code,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}
}
