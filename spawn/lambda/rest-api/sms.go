package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	spawnclient "github.com/scttfrdmn/spore-host/spawn/pkg/aws"
)

const pendingTable = "spore-sms-pending"
const pendingTTLMinutes = 15

// PendingNotification tracks what we're waiting for a reply on.
type PendingNotification struct {
	Phone      string            // user's phone number
	Project    string            // "spore", "prism", etc.
	InstanceID string
	Region     string
	EventType  string            // "ttl_warning", "idle_stopped", etc.
	Options    map[string]string // "1" -> "extend:1h", "2" -> "extend:2h", etc.
	ExpiresAt  int64             // unix timestamp
}

// StorePending saves a pending notification so incoming replies can be matched.
func StorePending(ctx context.Context, n PendingNotification) error {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return err
	}
	client := dynamodb.NewFromConfig(cfg)

	optJSON := encodeOptions(n.Options)
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(pendingTable),
		Item: map[string]dynamodbtypes.AttributeValue{
			"phone":       &dynamodbtypes.AttributeValueMemberS{Value: n.Phone},
			"project":     &dynamodbtypes.AttributeValueMemberS{Value: n.Project},
			"instance_id": &dynamodbtypes.AttributeValueMemberS{Value: n.InstanceID},
			"region":      &dynamodbtypes.AttributeValueMemberS{Value: n.Region},
			"event_type":  &dynamodbtypes.AttributeValueMemberS{Value: n.EventType},
			"options":     &dynamodbtypes.AttributeValueMemberS{Value: optJSON},
			"expires_at":  &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", n.ExpiresAt)},
			"ttl":         &dynamodbtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", n.ExpiresAt)},
		},
	})
	return err
}

// BuildSMSMessage formats the outbound SMS for an event type with a numbered menu.
func BuildSMSMessage(instanceName, eventType string, extraInfo map[string]string) (string, map[string]string) {
	options := map[string]string{}
	var msg strings.Builder

	switch eventType {
	case "ttl_warning":
		remaining := extraInfo["remaining"]
		fmt.Fprintf(&msg, "%s terminates in %s.\n\n", instanceName, remaining)
		fmt.Fprintf(&msg, "1 \u00b7 Extend 1h\n2 \u00b7 Extend 2h\n4 \u00b7 Extend 4h\n0 \u00b7 Dismiss")
		options["1"] = "extend:1h"
		options["2"] = "extend:2h"
		options["4"] = "extend:4h"
		options["0"] = "dismiss"

	case "idle_warning":
		idle := extraInfo["idle_duration"]
		remaining := extraInfo["remaining"]
		fmt.Fprintf(&msg, "%s idle %s, stops in %s.\n\n", instanceName, idle, remaining)
		fmt.Fprintf(&msg, "1 \u00b7 Keep running (reset timer)\n0 \u00b7 Dismiss")
		options["1"] = "keep"
		options["0"] = "dismiss"

	case "idle_stopped", "idle_hibernated":
		cost := extraInfo["cost"]
		verb := "stopped"
		if eventType == "idle_hibernated" {
			verb = "hibernated"
		}
		fmt.Fprintf(&msg, "%s %s (idle). Cost so far: %s\n\n", instanceName, verb, cost)
		fmt.Fprintf(&msg, "1 \u00b7 Wake instance\n0 \u00b7 Dismiss")
		options["1"] = "start"
		options["0"] = "dismiss"

	case "ttl_expired":
		cost := extraInfo["cost"]
		fmt.Fprintf(&msg, "%s terminated (TTL). Cumulative cost: %s", instanceName, cost)
		// no reply options

	case "completion":
		cost := extraInfo["cost"]
		fmt.Fprintf(&msg, "%s job done. Cost: %s\n\n", instanceName, cost)
		fmt.Fprintf(&msg, "1 \u00b7 Get status\n0 \u00b7 Dismiss")
		options["1"] = "status"
		options["0"] = "dismiss"

	case "spot_interrupt":
		fmt.Fprintf(&msg, "%s Spot interruption \u2014 ~2 min remaining.", instanceName)
		// no reply options

	default:
		fmt.Fprintf(&msg, "%s: %s", instanceName, eventType)
	}

	return msg.String(), options
}

// handleSMSIncoming processes a Twilio webhook for an inbound SMS reply.
func handleSMSIncoming(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	// Validate Twilio signature
	authToken := os.Getenv("TWILIO_AUTH_TOKEN")
	if authToken != "" {
		if !validateTwilioSignature(req, authToken) {
			return errResp(http.StatusForbidden, "invalid Twilio signature"), nil
		}
	}

	// Parse form body (Twilio sends application/x-www-form-urlencoded)
	params, err := url.ParseQuery(req.Body)
	if err != nil {
		return errResp(http.StatusBadRequest, "invalid body"), nil
	}

	from := params.Get("From")   // user's phone number
	body := strings.TrimSpace(params.Get("Body"))

	if from == "" || body == "" {
		return twilioResp("")
	}

	// Look up pending notification
	pending, err := fetchPending(ctx, from)
	if err != nil || pending == nil {
		return twilioResp("No pending notification found. Use the spore.host CLI or Slack bot to manage your instances.")
	}

	// Match reply to option
	action, ok := pending.Options[body]
	if !ok {
		opts := buildOptionsHint(pending.Options)
		return twilioResp(fmt.Sprintf("Unrecognised reply %q. Valid: %s", body, opts))
	}

	if action == "dismiss" {
		clearPending(ctx, from)
		return twilioResp("Dismissed.")
	}

	// Execute the action
	reply, err := executeAction(ctx, pending, action)
	if err != nil {
		return twilioResp(fmt.Sprintf("Error: %v", err))
	}

	clearPending(ctx, from)
	return twilioResp(reply)
}

func executeAction(ctx context.Context, p *PendingNotification, action string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(p.Region))
	if err != nil {
		return "", fmt.Errorf("AWS config: %w", err)
	}
	client := spawnclient.NewClientFromConfig(cfg)

	switch {
	case action == "start":
		if err := client.StartInstance(ctx, p.Region, p.InstanceID); err != nil {
			return "", err
		}
		return fmt.Sprintf("Waking instance. Use `spawn connect` to reconnect when it's running."), nil

	case action == "status":
		state, err := client.GetInstanceState(ctx, p.Region, p.InstanceID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Instance %s: %s", p.InstanceID[:12], state), nil

	case strings.HasPrefix(action, "extend:"):
		dur := strings.TrimPrefix(action, "extend:")
		if err := client.UpdateInstanceTags(ctx, p.Region, p.InstanceID, map[string]string{
			"spawn:ttl": dur,
		}); err != nil {
			return "", err
		}
		return fmt.Sprintf("TTL extended by %s.", dur), nil

	case action == "keep":
		// Reset idle timer by updating a tag spored checks
		if err := client.UpdateInstanceTags(ctx, p.Region, p.InstanceID, map[string]string{
			"spawn:idle-reset": time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return "", err
		}
		return "Idle timer reset. Instance will keep running.", nil

	default:
		return "", fmt.Errorf("unknown action %q", action)
	}
}

// handleNotificationRegister saves or removes a phone number for a user.
func handleNotificationRegister(ctx context.Context, method string, req events.APIGatewayV2HTTPRequest, p *Principal) (events.APIGatewayV2HTTPResponse, error) {
	var body struct {
		Phone     string `json:"phone"`
		UserKey   string `json:"user_key"`
	}
	if err := parseJSON(req.Body, &body); err != nil {
		return errResp(http.StatusBadRequest, "invalid JSON body"), nil
	}
	if body.Phone == "" || body.UserKey == "" {
		return errResp(http.StatusBadRequest, "phone and user_key required"), nil
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return errResp(http.StatusInternalServerError, "AWS config error"), nil
	}
	table := os.Getenv("REGISTRY_TABLE")
	if table == "" {
		table = "spore-bot-registry"
	}
	client := dynamodb.NewFromConfig(cfg)

	if method == "DELETE" {
		_, err = client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(table),
			Key: map[string]dynamodbtypes.AttributeValue{
				"user_key": &dynamodbtypes.AttributeValueMemberS{Value: body.UserKey},
				"nickname": &dynamodbtypes.AttributeValueMemberS{Value: "_phone"},
			},
			UpdateExpression: aws.String("REMOVE phone"),
		})
	} else {
		_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(table),
			Item: map[string]dynamodbtypes.AttributeValue{
				"user_key": &dynamodbtypes.AttributeValueMemberS{Value: body.UserKey},
				"nickname": &dynamodbtypes.AttributeValueMemberS{Value: "_phone"},
				"phone":    &dynamodbtypes.AttributeValueMemberS{Value: body.Phone},
				"project":  &dynamodbtypes.AttributeValueMemberS{Value: p.Project},
			},
		})
	}
	if err != nil {
		return errResp(http.StatusInternalServerError, fmt.Sprintf("DynamoDB: %v", err)), nil
	}
	verb := "registered"
	if method == "DELETE" {
		verb = "deregistered"
	}
	return jsonResp(http.StatusOK, map[string]string{"status": verb, "phone": body.Phone}), nil
}

// twilioResp returns a TwiML response with a message.
func twilioResp(msg string) (events.APIGatewayV2HTTPResponse, error) {
	var body string
	if msg == "" {
		body = `<?xml version="1.0" encoding="UTF-8"?><Response></Response>`
	} else {
		body = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><Response><Message>%s</Message></Response>`, msg)
	}
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/xml"},
		Body:       body,
	}, nil
}

func validateTwilioSignature(req events.APIGatewayV2HTTPRequest, authToken string) bool {
	// Reconstruct URL from request context
	urlStr := "https://" + req.RequestContext.DomainName + req.RequestContext.HTTP.Path

	params, _ := url.ParseQuery(req.Body)
	paramStr := ""
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var sb strings.Builder
		for _, k := range keys {
			sb.WriteString(k)
			sb.WriteString(params.Get(k))
		}
		paramStr = sb.String()
	}

	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(urlStr + paramStr))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	sig := req.Headers["x-twilio-signature"]
	return hmac.Equal([]byte(expected), []byte(sig))
}

func fetchPending(ctx context.Context, phone string) (*PendingNotification, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return nil, err
	}
	client := dynamodb.NewFromConfig(cfg)
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(pendingTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"phone": &dynamodbtypes.AttributeValueMemberS{Value: phone},
		},
	})
	if err != nil || out.Item == nil {
		return nil, nil
	}
	get := func(k string) string {
		if v, ok := out.Item[k].(*dynamodbtypes.AttributeValueMemberS); ok {
			return v.Value
		}
		return ""
	}
	return &PendingNotification{
		Phone:      phone,
		Project:    get("project"),
		InstanceID: get("instance_id"),
		Region:     get("region"),
		EventType:  get("event_type"),
		Options:    decodeOptions(get("options")),
	}, nil
}

func clearPending(ctx context.Context, phone string) {
	cfg, _ := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	client := dynamodb.NewFromConfig(cfg)
	_, _ = client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(pendingTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"phone": &dynamodbtypes.AttributeValueMemberS{Value: phone},
		},
	})
}

func encodeOptions(opts map[string]string) string {
	var parts []string
	for k, v := range opts {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func decodeOptions(s string) map[string]string {
	m := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
	}
	return m
}

func buildOptionsHint(opts map[string]string) string {
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
