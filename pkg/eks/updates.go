package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

type EKSUpdateInfo struct {
	Profile     string
	Region      string
	ClusterName string
	Type        string
	Status      string
	Errors      []string
}

func GetEKSUpdates(profile, region, clusterName string) ([]EKSUpdateInfo, error) {
	ctx := context.Background()

	// Load the AWS configuration using the profile and region
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create an EKS client
	eksSvc := eks.NewFromConfig(cfg)

	updatesOut, err := eksSvc.ListUpdates(ctx, &eks.ListUpdatesInput{
		Name: aws.String(clusterName),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list updates for cluster %s in region %s: %w", clusterName, region, err)
	}

	updatesList := []EKSUpdateInfo{}

	for _, updateID := range updatesOut.UpdateIds {
		updateDesc, err := eksSvc.DescribeUpdate(ctx, &eks.DescribeUpdateInput{
			Name:     aws.String(clusterName),
			UpdateId: aws.String(updateID),
		})
		if err != nil {
			continue
		}

		if updateDesc.Update == nil {
			continue
		}

		newUpdate := EKSUpdateInfo{
			Type:   string(updateDesc.Update.Type),
			Status: string(updateDesc.Update.Status),
		}

		for _, updateErr := range updateDesc.Update.Errors {
			if updateErr.ErrorMessage != nil {
				newUpdate.Errors = append(newUpdate.Errors, *updateErr.ErrorMessage)
			}
		}

		if len(newUpdate.Status) == 0 && len(newUpdate.Type) == 0 {
			continue
		}

		updatesList = append(updatesList, newUpdate)
	}

	return updatesList, nil
}
