package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	lambdasvc "github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

var (
	cfg          aws.Config
	reg          *Registry
	lambdaClient *lambdasvc.Client
	functionName string
	httpClient   = &http.Client{Timeout: 15 * time.Second}
)

func init() {
	ctx := context.Background()
	var err error
	cfg, err = awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	reg = newRegistry(cfg)
	lambdaClient = lambdasvc.NewFromConfig(cfg)
	functionName = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
}

// handler routes between webhook (Phase 1) and async action (Phase 2).
// Phase 1 receives APIGatewayProxyRequest events from API Gateway.
// Phase 2 receives BotAction JSON from async self-invocation.
func handler(ctx context.Context, rawEvent json.RawMessage) (interface{}, error) {
	// Try to parse as API Gateway event first (Phase 1)
	var apiReq events.APIGatewayProxyRequest
	if err := json.Unmarshal(rawEvent, &apiReq); err == nil && apiReq.HTTPMethod != "" {
		return handleWebhook(ctx, cfg, reg, apiReq)
	}

	// Otherwise it's a BotAction payload (Phase 2)
	return nil, handleAsyncAction(ctx, cfg, reg, rawEvent)
}

// invokeAsync kicks off Phase 2 as an async Lambda self-invocation.
func invokeAsync(ctx context.Context, action *BotAction) error {
	if functionName == "" {
		return fmt.Errorf("function name not set")
	}
	payload, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("marshal action: %w", err)
	}
	_, err = lambdaClient.Invoke(ctx, &lambdasvc.InvokeInput{
		FunctionName:   aws.String(functionName),
		InvocationType: lambdatypes.InvocationTypeEvent,
		Payload:        payload,
	})
	return err
}

// httpPost is a shared helper for posting JSON responses back to Slack/Teams.
func httpPost(url, contentType string, body []byte) error {
	resp, err := httpClient.Post(url, contentType, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("post returned %d", resp.StatusCode)
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func main() {
	lambda.Start(handler)
}
