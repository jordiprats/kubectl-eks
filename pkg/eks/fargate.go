package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/jordiprats/kubectl-eks/pkg/awsutil"
)

type FargateProfileInfo struct {
	Profile        string
	Region         string
	ClusterName    string
	Name           string
	Status         string
	PodExecRoleArn string
	Subnets        []string
	Selectors      []FargateSelector
}

type FargateSelector struct {
	Namespace string
	Labels    map[string]string
}

// Client wraps the EKS API for easier testing
type Client struct {
	api awsutil.EKSAPI
}

// NewClient creates a new EKS client
func NewClient(profile, region string) (*Client, error) {
	cfg, err := awsutil.LoadConfig(profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &Client{api: eks.NewFromConfig(cfg)}, nil
}

// NewClientWithAPI creates a client with a custom API implementation (for testing)
func NewClientWithAPI(api awsutil.EKSAPI) *Client {
	return &Client{api: api}
}

func GetEKSFargateProfiles(profile, region, clusterName string) ([]FargateProfileInfo, error) {
	client, err := NewClient(profile, region)
	if err != nil {
		return nil, err
	}
	return client.GetFargateProfiles(clusterName)
}

func (c *Client) GetFargateProfiles(clusterName string) ([]FargateProfileInfo, error) {
	// List all Fargate profile names
	var profileNames []string
	var nextToken *string

	for {
		output, err := c.api.ListFargateProfiles(context.TODO(), &eks.ListFargateProfilesInput{
			ClusterName: aws.String(clusterName),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list Fargate profiles: %w", err)
		}

		profileNames = append(profileNames, output.FargateProfileNames...)

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	// Describe each profile
	var profiles []FargateProfileInfo
	for _, name := range profileNames {
		resp, err := c.api.DescribeFargateProfile(context.TODO(), &eks.DescribeFargateProfileInput{
			ClusterName:        aws.String(clusterName),
			FargateProfileName: aws.String(name),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe Fargate profile %s: %w", name, err)
		}

		fp := resp.FargateProfile
		info := FargateProfileInfo{
			Name:           aws.ToString(fp.FargateProfileName),
			Status:         string(fp.Status),
			PodExecRoleArn: aws.ToString(fp.PodExecutionRoleArn),
			Subnets:        fp.Subnets,
		}

		for _, sel := range fp.Selectors {
			selector := FargateSelector{
				Namespace: aws.ToString(sel.Namespace),
				Labels:    sel.Labels,
			}
			info.Selectors = append(info.Selectors, selector)
		}

		profiles = append(profiles, info)
	}

	return profiles, nil
}
