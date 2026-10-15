package sts

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func GetAccountID(profile, region string) (string, error) {
	ctx := context.Background()

	// Load the AWS configuration using the profile and region
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create a new STS client
	stsSvc := sts.NewFromConfig(cfg)

	// Call GetCallerIdentity
	result, err := stsSvc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		log.Fatalf("Failed to get caller identity: %v", err)
	}

	return aws.ToString(result.Account), nil
}

func TryGetAccountID(profile, region string) (string, error) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	stsSvc := sts.NewFromConfig(cfg)

	result, err := stsSvc.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %w", err)
	}

	return aws.ToString(result.Account), nil
}
