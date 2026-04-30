package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	spawnclient "github.com/scttfrdmn/spore-host/spawn/pkg/aws"
)

func handleListInstances(ctx context.Context, cfg aws.Config, req events.APIGatewayV2HTTPRequest, p *Principal) (events.APIGatewayV2HTTPResponse, error) {
	region := req.QueryStringParameters["region"]
	state := req.QueryStringParameters["state"]
	if state == "" {
		state = "running"
	}

	client := spawnclient.NewClientFromConfig(cfg)
	instances, err := client.ListInstances(ctx, region, state)
	if err != nil {
		return errResp(http.StatusInternalServerError, fmt.Sprintf("list instances: %v", err)), nil
	}

	type instanceOut struct {
		InstanceID       string            `json:"instance_id"`
		Name             string            `json:"name"`
		InstanceType     string            `json:"instance_type"`
		State            string            `json:"state"`
		Region           string            `json:"region"`
		AvailabilityZone string            `json:"availability_zone,omitempty"`
		PublicIP         string            `json:"public_ip,omitempty"`
		DNS              string            `json:"dns,omitempty"`
		LaunchTime       time.Time         `json:"launch_time"`
		TTL              string            `json:"ttl,omitempty"`
		IdleTimeout      string            `json:"idle_timeout,omitempty"`
		Tags             map[string]string `json:"tags,omitempty"`
	}

	out := make([]instanceOut, 0, len(instances))
	for _, inst := range instances {
		dns := inst.Tags["spawn:dns-name"]
		out = append(out, instanceOut{
			InstanceID:       inst.InstanceID,
			Name:             inst.Name,
			InstanceType:     inst.InstanceType,
			State:            inst.State,
			Region:           inst.Region,
			AvailabilityZone: inst.AvailabilityZone,
			PublicIP:         inst.PublicIP,
			DNS:              dns,
			LaunchTime:       inst.LaunchTime,
			TTL:              inst.TTL,
			IdleTimeout:      inst.IdleTimeout,
		})
	}
	return jsonResp(http.StatusOK, map[string]any{"instances": out, "count": len(out)}), nil
}

func handleGetInstance(ctx context.Context, cfg aws.Config, id string, p *Principal) (events.APIGatewayV2HTTPResponse, error) {
	client := spawnclient.NewClientFromConfig(cfg)
	instances, err := client.ListInstances(ctx, "", "")
	if err != nil {
		return errResp(http.StatusInternalServerError, fmt.Sprintf("list: %v", err)), nil
	}
	for _, inst := range instances {
		if inst.InstanceID == id || strings.EqualFold(inst.Name, id) {
			return jsonResp(http.StatusOK, inst), nil
		}
	}
	return errResp(http.StatusNotFound, fmt.Sprintf("instance %q not found", id)), nil
}

func handleInstanceAction(ctx context.Context, cfg aws.Config, id, action string, req events.APIGatewayV2HTTPRequest, p *Principal) (events.APIGatewayV2HTTPResponse, error) {
	client := spawnclient.NewClientFromConfig(cfg)

	// Resolve instance
	instances, err := client.ListInstances(ctx, "", "")
	if err != nil {
		return errResp(http.StatusInternalServerError, "list instances failed"), nil
	}
	var target *spawnclient.InstanceInfo
	for i := range instances {
		if instances[i].InstanceID == id || strings.EqualFold(instances[i].Name, id) {
			target = &instances[i]
			break
		}
	}
	if target == nil {
		return errResp(http.StatusNotFound, fmt.Sprintf("instance %q not found", id)), nil
	}

	switch action {
	case "stop":
		if err := client.StopInstance(ctx, target.Region, target.InstanceID, false); err != nil {
			return errResp(http.StatusInternalServerError, fmt.Sprintf("stop failed: %v", err)), nil
		}
		return jsonResp(http.StatusOK, map[string]string{"status": "stopped", "instance_id": target.InstanceID}), nil

	case "hibernate":
		if err := client.StopInstance(ctx, target.Region, target.InstanceID, true); err != nil {
			return errResp(http.StatusInternalServerError, fmt.Sprintf("hibernate failed: %v", err)), nil
		}
		return jsonResp(http.StatusOK, map[string]string{"status": "hibernating", "instance_id": target.InstanceID}), nil

	case "start":
		if err := client.StartInstance(ctx, target.Region, target.InstanceID); err != nil {
			return errResp(http.StatusInternalServerError, fmt.Sprintf("start failed: %v", err)), nil
		}
		return jsonResp(http.StatusOK, map[string]string{"status": "starting", "instance_id": target.InstanceID}), nil

	case "terminate":
		if err := client.Terminate(ctx, target.Region, target.InstanceID); err != nil {
			return errResp(http.StatusInternalServerError, fmt.Sprintf("terminate failed: %v", err)), nil
		}
		return jsonResp(http.StatusOK, map[string]string{"status": "terminating", "instance_id": target.InstanceID}), nil

	case "extend":
		var body struct {
			Duration string `json:"duration"`
		}
		if req.Body != "" {
			if err := parseJSON(req.Body, &body); err != nil || body.Duration == "" {
				return errResp(http.StatusBadRequest, "body must include {\"duration\": \"2h\"}"), nil
			}
		} else {
			body.Duration = req.QueryStringParameters["duration"]
		}
		if body.Duration == "" {
			return errResp(http.StatusBadRequest, "duration required"), nil
		}
		if err := client.UpdateInstanceTags(ctx, target.Region, target.InstanceID, map[string]string{
			"spawn:ttl": body.Duration,
		}); err != nil {
			return errResp(http.StatusInternalServerError, fmt.Sprintf("extend failed: %v", err)), nil
		}
		return jsonResp(http.StatusOK, map[string]string{"status": "extended", "ttl": body.Duration, "instance_id": target.InstanceID}), nil

	default:
		return errResp(http.StatusBadRequest, fmt.Sprintf("unknown action %q — valid: stop, start, hibernate, terminate, extend", action)), nil
	}
}
