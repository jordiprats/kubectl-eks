package cf

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/jordiprats/kubectl-eks/pkg/awsutil"
)

type StackInfo struct {
	Profile     string
	Region      string
	ClusterName string
	Name        string
	Status      string
}

// Client wraps the CloudFormation API for easier testing
type Client struct {
	api awsutil.CloudFormationAPI
}

// NewClient creates a new CloudFormation client
func NewClient(profile, region string) (*Client, error) {
	cfg, err := awsutil.LoadConfig(profile, region)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &Client{api: cloudformation.NewFromConfig(cfg)}, nil
}

// NewClientWithAPI creates a client with a custom API implementation (for testing)
func NewClientWithAPI(api awsutil.CloudFormationAPI) *Client {
	return &Client{api: api}
}

func GetStacks(name, profile, region string) ([]StackInfo, error) {
	client, err := NewClient(profile, region)
	if err != nil {
		return nil, err
	}
	return client.GetStacks(name)
}

func (c *Client) GetStacks(name string) ([]StackInfo, error) {
	var stackList []StackInfo
	var nextToken *string

	for {
		output, err := c.api.ListStacks(context.TODO(), &cloudformation.ListStacksInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list stacks: %w", err)
		}

		for _, stack := range output.StackSummaries {
			if strings.Contains(*stack.StackName, name) {
				stackList = append(stackList, StackInfo{
					Name:   *stack.StackName,
					Status: string(stack.StackStatus),
				})
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return stackList, nil
}

func GetStacksByParameter(paramName, paramValue, profile, region string) ([]StackInfo, error) {
	client, err := NewClient(profile, region)
	if err != nil {
		return nil, err
	}
	return client.GetStacksByParameter(paramName, paramValue)
}

func (c *Client) GetStacksByParameter(paramName, paramValue string) ([]StackInfo, error) {
	var stackList []StackInfo
	var nextToken *string

	for {
		output, err := c.api.DescribeStacks(context.TODO(), &cloudformation.DescribeStacksInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe stacks: %w", err)
		}

		for _, stack := range output.Stacks {
			if stack.StackStatus == types.StackStatusDeleteComplete {
				continue
			}

			if hasParameterValue(stack.Parameters, paramName, paramValue) {
				stackList = append(stackList, StackInfo{
					Name:   *stack.StackName,
					Status: string(stack.StackStatus),
				})
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return stackList, nil
}

func hasParameterValue(params []types.Parameter, key, value string) bool {
	for _, p := range params {
		if p.ParameterKey != nil && *p.ParameterKey == key {
			return p.ParameterValue != nil && *p.ParameterValue == value
		}
	}
	return false
}
