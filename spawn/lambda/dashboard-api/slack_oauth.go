package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	slackAuthorizeURL = "https://slack.com/oauth/v2/authorize"
	slackAccessURL    = "https://slack.com/api/oauth.v2.access"
	slackOAuthScopes  = "commands,chat:write,users:read,users:read.email"
)

// handleSlackOAuthRedirect redirects the user to Slack's OAuth authorization page.
// GET /api/slack/oauth
func handleSlackOAuthRedirect(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	clientID := os.Getenv("SLACK_CLIENT_ID")
	if clientID == "" {
		return errorResponse(500, "Slack OAuth not configured"), nil
	}
	redirectURI := slackRedirectURI(request)

	authURL := fmt.Sprintf("%s?client_id=%s&scope=%s&redirect_uri=%s",
		slackAuthorizeURL,
		url.QueryEscape(clientID),
		url.QueryEscape(slackOAuthScopes),
		url.QueryEscape(redirectURI),
	)

	return events.APIGatewayProxyResponse{
		StatusCode: 302,
		Headers: map[string]string{
			"Location":                        authURL,
			"Access-Control-Allow-Origin":     "*",
			"Access-Control-Allow-Headers":    "Content-Type",
		},
	}, nil
}

// handleSlackOAuthCallback exchanges the Slack OAuth code for a bot token,
// stores the workspace credentials in spore-bot-workspaces DynamoDB, and
// redirects the user to the dashboard with a success indicator.
// GET /api/slack/oauth/callback?code=...
func handleSlackOAuthCallback(ctx context.Context, cfg aws.Config, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	clientID := os.Getenv("SLACK_CLIENT_ID")
	clientSecret := os.Getenv("SLACK_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return errorResponse(500, "Slack OAuth not configured"), nil
	}

	// Check for error from Slack (user denied)
	if errParam := request.QueryStringParameters["error"]; errParam != "" {
		return redirectToDashboard("error=" + url.QueryEscape(errParam)), nil
	}

	code := request.QueryStringParameters["code"]
	if code == "" {
		return errorResponse(400, "Missing OAuth code"), nil
	}

	// Exchange code for bot token
	token, err := exchangeSlackCode(ctx, clientID, clientSecret, code, slackRedirectURI(request))
	if err != nil {
		return errorResponse(500, fmt.Sprintf("OAuth exchange failed: %v", err)), nil
	}

	// Store workspace in spore-bot-workspaces DynamoDB
	workspacesTable := getEnvOrDefault("SPORE_BOT_WORKSPACES_TABLE", "spore-bot-workspaces")
	if err := storeSlackWorkspace(ctx, cfg, workspacesTable, token); err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to store workspace: %v", err)), nil
	}

	// Redirect to dashboard with success
	return redirectToDashboard(fmt.Sprintf("bot=connected&workspace=%s&workspace_name=%s",
		url.QueryEscape(token.Team.ID),
		url.QueryEscape(token.Team.Name),
	)), nil
}

// slackOAuthTokenResponse is the response from Slack's oauth.v2.access endpoint.
type slackOAuthTokenResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	AccessToken string `json:"access_token"`
	BotToken    string `json:"-"` // set after parsing
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
	AuthedUser struct {
		ID string `json:"id"`
	} `json:"authed_user"`
	Bot struct {
		BotAccessToken string `json:"bot_access_token"`
	} `json:"bot"`
	// The bot token is in different fields depending on the OAuth version
	// For OAuth v2, it's under the top-level access_token when scopes include bot
}

func exchangeSlackCode(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*slackOAuthTokenResponse, error) {
	resp, err := http.PostForm(slackAccessURL, url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"redirect_uri":  {redirectURI},
	})
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var token slackOAuthTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !token.OK {
		return nil, fmt.Errorf("Slack API error: %s", token.Error)
	}

	// Bot token is the top-level access_token for workspace apps (OAuth v2)
	token.BotToken = token.AccessToken
	return &token, nil
}

// storeSlackWorkspace writes the workspace credentials to DynamoDB.
// The signing secret is NOT available from the OAuth flow — it must be set
// separately via `spawn bot workspace-add --signing-secret ...` or updated after.
// The bot token is sufficient for most operations; signing secret is for webhook verification.
func storeSlackWorkspace(ctx context.Context, cfg aws.Config, tableName string, token *slackOAuthTokenResponse) error {
	client := dynamodb.NewFromConfig(cfg)
	workspaceKey := "slack#" + token.Team.ID

	// Check if workspace already exists (preserve signing secret if set)
	existing, _ := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"workspace_key": &dynamodbtypes.AttributeValueMemberS{Value: workspaceKey},
		},
	})

	// The signing secret is app-level (same for all workspaces using this Slack app).
	// It is stored as SLACK_SIGNING_SECRET in the Lambda env — users never need to know it.
	// Prefer: env var > existing stored value > empty (bot token-only install)
	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")
	if signingSecret == "" && existing != nil && existing.Item != nil {
		if v, ok := existing.Item["signing_secret"].(*dynamodbtypes.AttributeValueMemberS); ok {
			signingSecret = v.Value
		}
	}

	ws := map[string]interface{}{
		"workspace_key":  workspaceKey,
		"bot_token":      token.BotToken,
		"signing_secret": signingSecret,
		"platform":       "slack",
		"workspace_name": token.Team.Name,
		"installed_by":   "oauth:" + token.AuthedUser.ID,
		"installed_at":   time.Now().UTC().Format(time.RFC3339),
	}
	item, err := attributevalue.MarshalMap(ws)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	return err
}

func slackRedirectURI(request events.APIGatewayProxyRequest) string {
	// Allow override via env var for local testing
	if override := os.Getenv("SLACK_REDIRECT_URI"); override != "" {
		return override
	}
	// Construct from request host
	host := request.Headers["Host"]
	if host == "" {
		host = request.Headers["host"]
	}
	if host == "" {
		host = "api.spore.host"
	}
	return "https://" + host + "/api/slack/oauth/callback"
}

func redirectToDashboard(params string) events.APIGatewayProxyResponse {
	dashboardURL := os.Getenv("DASHBOARD_URL")
	if dashboardURL == "" {
		dashboardURL = "https://spore.host/dashboard.html"
	}
	if params != "" {
		if strings.Contains(dashboardURL, "?") {
			dashboardURL += "&" + params
		} else {
			dashboardURL += "?" + params
		}
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 302,
		Headers: map[string]string{
			"Location":                    dashboardURL,
			"Access-Control-Allow-Origin": "*",
		},
	}
}
