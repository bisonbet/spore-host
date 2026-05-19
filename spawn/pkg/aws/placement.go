package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// CreatePlacementGroup creates a cluster placement group for MPI
func (c *Client) CreatePlacementGroup(ctx context.Context, name string) error {
	ec2Client := ec2.NewFromConfig(c.cfg)

	_, err := ec2Client.CreatePlacementGroup(ctx, &ec2.CreatePlacementGroupInput{
		GroupName: aws.String(name),
		Strategy:  types.PlacementStrategyCluster,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypePlacementGroup,
				Tags: []types.Tag{
					{Key: aws.String("spawn:managed"), Value: aws.String("true")},
					{Key: aws.String("spawn:purpose"), Value: aws.String("mpi")},
				},
			},
		},
	})

	if err != nil {
		// Check if already exists (error string contains "already exists")
		if strings.Contains(err.Error(), "already exists") {
			return nil // Already exists, not an error
		}
		return fmt.Errorf("create placement group: %w", err)
	}

	return nil
}

// DeletePlacementGroup removes a placement group
func (c *Client) DeletePlacementGroup(ctx context.Context, name string) error {
	ec2Client := ec2.NewFromConfig(c.cfg)

	_, err := ec2Client.DeletePlacementGroup(ctx, &ec2.DeletePlacementGroupInput{
		GroupName: aws.String(name),
	})
	return err
}

// ValidateInstanceTypeForPlacementGroup checks if instance type supports cluster placement
func (c *Client) ValidateInstanceTypeForPlacementGroup(ctx context.Context, instanceType string) error {
	// Only certain instance families support cluster placement groups:
	// - Compute optimized: c4, c5, c5n, c6g, c6gn, c7g
	// - Memory optimized: r4, r5, r5n, r6g, x1, x1e
	// - Storage optimized: d2, h1, i3, i3en
	// - Accelerated: p2, p3, p4, g3, g4dn, inf1

	supportedPrefixes := []string{
		"c4.", "c5.", "c5n.", "c6g.", "c6gn.", "c7g.",
		"r4.", "r5.", "r5n.", "r6g.",
		"x1.", "x1e.",
		"d2.", "h1.", "i3.", "i3en.",
		"p2.", "p3.", "p4.", "g3.", "g4dn.", "inf1.",
	}

	for _, prefix := range supportedPrefixes {
		if strings.HasPrefix(instanceType, prefix) {
			return nil
		}
	}

	return fmt.Errorf("instance type %s does not support cluster placement groups", instanceType)
}

// ValidateInstanceTypeForEFAInRegion checks if instance type supports EFA by
// querying the EC2 API in the specified launch region. Some instance types
// (e.g. hpc6a.48xlarge) only exist in certain regions and DescribeInstanceTypes
// returns InvalidInstanceType when queried from a different region.
func (c *Client) ValidateInstanceTypeForEFAInRegion(ctx context.Context, instanceType, region string) error {
	cfg := c.cfg.Copy()
	if region != "" {
		cfg.Region = region
	}
	ec2Client := ec2.NewFromConfig(cfg)

	output, err := ec2Client.DescribeInstanceTypes(ctx, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: []types.InstanceType{types.InstanceType(instanceType)},
	})
	if err != nil {
		return fmt.Errorf("describe instance type %s: %w", instanceType, err)
	}
	if len(output.InstanceTypes) == 0 {
		return fmt.Errorf("instance type %s not found", instanceType)
	}

	info := output.InstanceTypes[0]
	if info.NetworkInfo == nil || !aws.ToBool(info.NetworkInfo.EfaSupported) {
		return fmt.Errorf("instance type %s does not support EFA", instanceType)
	}

	return nil
}
