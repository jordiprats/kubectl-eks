package sts

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
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

func tryGetCallerIdentity(profile, region string) (*sts.GetCallerIdentityOutput, error) {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	result, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to get caller identity: %w", err)
	}
	return result, nil
}

func TryGetAccountID(profile, region string) (string, error) {
	result, err := tryGetCallerIdentity(profile, region)
	if err != nil {
		return "", err
	}
	return aws.ToString(result.Account), nil
}

func TryGetCallerARN(profile, region string) (string, error) {
	result, err := tryGetCallerIdentity(profile, region)
	if err != nil {
		return "", err
	}
	return aws.ToString(result.Arn), nil
}

func RoleFromCallerARN(callerARN string) string {
	parsed, err := arn.Parse(callerARN)
	if err != nil {
		return callerARN
	}
	parts := strings.Split(parsed.Resource, "/")
	if parts[0] == "assumed-role" && len(parts) >= 3 {
		return parts[1]
	}
	return parsed.Resource
}
