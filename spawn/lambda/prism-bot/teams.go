package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// TeamsActivity represents an incoming Teams outgoing webhook payload.
type TeamsActivity struct {
	Type string `json:"type"`
	Text string `json:"text"`
	From struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"from"`
	ServiceURL string `json:"serviceUrl"`
	// Teams doesn't have workspace IDs in the same way — use tenant ID
	ChannelData struct {
		Tenant struct {
			ID string `json:"id"`
		} `json:"tenant"`
	} `json:"channelData"`
}

// verifyTeamsSignature validates Teams outgoing webhook HMAC-SHA256 signature.
// Teams sends the signature in the Authorization header as "HMAC <base64>".
func verifyTeamsSignature(sharedSecret, body, authHeader string) error {
	if !strings.HasPrefix(authHeader, "HMAC ") {
		return fmt.Errorf("missing HMAC authorization")
	}
	sigB64 := strings.TrimPrefix(authHeader, "HMAC ")
	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(sharedSecret))
	mac.Write([]byte(body))
	expected := mac.Sum(nil)

	if !hmac.Equal(expected, sigBytes) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// parseTeamsCommand parses a Teams activity into a SlashCommand-like struct.
func parseTeamsActivity(body string) (*SlashCommand, string, error) {
	var activity TeamsActivity
	if err := json.Unmarshal([]byte(body), &activity); err != nil {
		return nil, "", fmt.Errorf("parse Teams activity: %w", err)
	}

	// Teams sends text like "@BotName /prism stop rstudio"
	text := strings.TrimSpace(activity.Text)
	// Strip @mention prefix if present
	if idx := strings.Index(text, ">"); idx >= 0 {
		text = strings.TrimSpace(text[idx+1:])
	}
	// Strip leading slash command name
	parts := strings.Fields(text)
	cmd := ""
	args := ""
	if len(parts) > 0 {
		cmd = parts[0] // e.g. "/prism" or "prism"
		if len(parts) > 1 {
			args = strings.Join(parts[1:], " ")
		}
	}

	sc := &SlashCommand{
		Command:     cmd,
		Text:        args,
		UserID:      activity.From.ID,
		WorkspaceID: activity.ChannelData.Tenant.ID,
		ResponseURL: activity.ServiceURL,
	}
	return sc, activity.ServiceURL, nil
}

// postTeamsResponse sends a response back to Teams via the service URL.
func postTeamsResponse(serviceURL, text string) error {
	msg := map[string]interface{}{
		"type": "message",
		"text": text,
	}
	data, _ := json.Marshal(msg)
	return httpPost(serviceURL, "application/json", data)
}
