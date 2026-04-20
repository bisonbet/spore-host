package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// BotRegistration maps a chat user to an EC2 instance they can control.
type BotRegistration struct {
	// PK: {platform}#{workspace-id}#{user-id}
	UserKey string `dynamodbav:"user_key"`
	// SK: {nickname}
	Nickname       string   `dynamodbav:"nickname"`
	InstanceID     string   `dynamodbav:"instance_id"`
	AWSAccountID   string   `dynamodbav:"aws_account_id"`
	RoleARN        string   `dynamodbav:"role_arn"`
	DNSName        string   `dynamodbav:"dns_name,omitempty"`
	TagPrefix      string   `dynamodbav:"tag_prefix"`
	AllowedActions []string `dynamodbav:"allowed_actions"`
	RegisteredBy   string   `dynamodbav:"registered_by"`
	Platform       string   `dynamodbav:"platform"`
	CreatedAt      string   `dynamodbav:"created_at"`
}

// WorkspaceConfig stores per-workspace OAuth tokens (bot token + signing secret).
type WorkspaceConfig struct {
	// PK: {platform}#{workspace-id}
	WorkspaceKey  string `dynamodbav:"workspace_key"`
	BotToken      string `dynamodbav:"bot_token"`
	SigningSecret string `dynamodbav:"signing_secret"`
	Platform      string `dynamodbav:"platform"`
	WorkspaceName string `dynamodbav:"workspace_name"`
	InstalledBy   string `dynamodbav:"installed_by"`
	InstalledAt   string `dynamodbav:"installed_at"`
}

// userKey builds the DynamoDB PK for a user: "{platform}#{workspace}#{user}".
func userKey(platform, workspaceID, userID string) string {
	return strings.Join([]string{platform, workspaceID, userID}, "#")
}

// workspaceKey builds the DynamoDB PK for a workspace: "{platform}#{workspace}".
func workspaceKey(platform, workspaceID string) string {
	return platform + "#" + workspaceID
}

// Registry handles DynamoDB operations for bot registrations and workspaces.
type Registry struct {
	client          *dynamodb.Client
	registryTable   string
	workspacesTable string
}

func newRegistry(cfg aws.Config) *Registry {
	return &Registry{
		client:          dynamodb.NewFromConfig(cfg),
		registryTable:   getEnv("BOT_REGISTRY_TABLE", "spore-bot-registry"),
		workspacesTable: getEnv("BOT_WORKSPACES_TABLE", "spore-bot-workspaces"),
	}
}

// GetWorkspace retrieves signing secret and bot token for a workspace.
func (r *Registry) GetWorkspace(ctx context.Context, platform, workspaceID string) (*WorkspaceConfig, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.workspacesTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"workspace_key": &dynamodbtypes.AttributeValueMemberS{Value: workspaceKey(platform, workspaceID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	if result.Item == nil {
		return nil, fmt.Errorf("workspace %s/%s not registered", platform, workspaceID)
	}
	var ws WorkspaceConfig
	if err := attributevalue.UnmarshalMap(result.Item, &ws); err != nil {
		return nil, fmt.Errorf("unmarshal workspace: %w", err)
	}
	return &ws, nil
}

// ListUserInstances returns all registered instances for a user.
func (r *Registry) ListUserInstances(ctx context.Context, platform, workspaceID, userID string) ([]BotRegistration, error) {
	key := userKey(platform, workspaceID, userID)
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.registryTable),
		KeyConditionExpression: aws.String("user_key = :uk"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":uk": &dynamodbtypes.AttributeValueMemberS{Value: key},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	regs := make([]BotRegistration, 0, len(result.Items))
	for _, item := range result.Items {
		var reg BotRegistration
		if err := attributevalue.UnmarshalMap(item, &reg); err != nil {
			continue
		}
		regs = append(regs, reg)
	}
	return regs, nil
}

// GetInstance retrieves a specific registered instance by nickname.
func (r *Registry) GetInstance(ctx context.Context, platform, workspaceID, userID, nickname string) (*BotRegistration, error) {
	key := userKey(platform, workspaceID, userID)
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.registryTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"user_key": &dynamodbtypes.AttributeValueMemberS{Value: key},
			"nickname": &dynamodbtypes.AttributeValueMemberS{Value: nickname},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}
	if result.Item == nil {
		return nil, nil
	}
	var reg BotRegistration
	if err := attributevalue.UnmarshalMap(result.Item, &reg); err != nil {
		return nil, fmt.Errorf("unmarshal registration: %w", err)
	}
	return &reg, nil
}

// PutRegistration stores a new instance registration.
func (r *Registry) PutRegistration(ctx context.Context, reg *BotRegistration) error {
	if reg.CreatedAt == "" {
		reg.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	item, err := attributevalue.MarshalMap(reg)
	if err != nil {
		return fmt.Errorf("marshal registration: %w", err)
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.registryTable),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("put registration: %w", err)
	}
	return nil
}

// DeleteRegistration removes an instance registration.
func (r *Registry) DeleteRegistration(ctx context.Context, platform, workspaceID, userID, nickname string) error {
	key := userKey(platform, workspaceID, userID)
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.registryTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"user_key": &dynamodbtypes.AttributeValueMemberS{Value: key},
			"nickname": &dynamodbtypes.AttributeValueMemberS{Value: nickname},
		},
	})
	if err != nil {
		return fmt.Errorf("delete registration: %w", err)
	}
	return nil
}

// isActionAllowed checks if an action is in the allowed list.
func isActionAllowed(reg *BotRegistration, action string) bool {
	for _, a := range reg.AllowedActions {
		if a == action {
			return true
		}
	}
	return false
}
