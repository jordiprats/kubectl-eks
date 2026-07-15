package eks

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
)

type EKSNodeGroupInfo struct {
	Profile            string
	Region             string
	ClusterName        string
	Name               string
	CapacityType       string
	ReleaseVersion     string
	LaunchTemplate     string
	InstanceType       string
	DesiredCapacity    int64
	MaxCapacity        int64
	MinCapacity        int64
	Version            string
	Status             string
	BootstrapArguments string
	BootstrapMatch     string
}

func GetEKSNodeGroups(profile, region, clusterName string) ([]EKSNodeGroupInfo, error) {
	ctx := context.Background()

	// Load the AWS configuration using the profile and region
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create clients
	eksSvc := eks.NewFromConfig(cfg)
	ec2Svc := ec2.NewFromConfig(cfg)

	// Get NG names
	result, err := eksSvc.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list node groups for cluster %s in region %s: %w", clusterName, region, err)
	}

	ngList := make([]EKSNodeGroupInfo, len(result.Nodegroups))

	for i, ng := range result.Nodegroups {
		// describe NodeGroup
		ngDesc, err := eksSvc.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(ng),
		})

		if err != nil {
			continue
		}

		if ngDesc.Nodegroup == nil {
			continue
		}

		instanceType := ""

		bootstrapArguments := ""
		if ngDesc.Nodegroup.LaunchTemplate != nil {
			ltDesc, err := ec2Svc.DescribeLaunchTemplateVersions(ctx, &ec2.DescribeLaunchTemplateVersionsInput{
				LaunchTemplateId: ngDesc.Nodegroup.LaunchTemplate.Id,
				Versions:         []string{aws.ToString(ngDesc.Nodegroup.LaunchTemplate.Version)},
			})

			if err == nil && len(ltDesc.LaunchTemplateVersions) > 0 && ltDesc.LaunchTemplateVersions[0].LaunchTemplateData != nil {
				instanceType = string(ltDesc.LaunchTemplateVersions[0].LaunchTemplateData.InstanceType)
				if ltDesc.LaunchTemplateVersions[0].LaunchTemplateData.UserData != nil {
					userData := aws.ToString(ltDesc.LaunchTemplateVersions[0].LaunchTemplateData.UserData)
					decoded, err := base64.StdEncoding.DecodeString(userData)
					if err == nil {
						bootstrapArguments = extractKubeletFlags(string(decoded))
					}
				}
			}
		}

		launchTemplateID := ""
		if ngDesc.Nodegroup.LaunchTemplate != nil {
			launchTemplateID = aws.ToString(ngDesc.Nodegroup.LaunchTemplate.Id)
		}

		ngList[i] = EKSNodeGroupInfo{
			Profile:            profile,
			Region:             region,
			ClusterName:        clusterName,
			Name:               ng,
			CapacityType:       string(ngDesc.Nodegroup.CapacityType),
			ReleaseVersion:     aws.ToString(ngDesc.Nodegroup.ReleaseVersion),
			InstanceType:       instanceType,
			LaunchTemplate:     launchTemplateID,
			DesiredCapacity:    int64(aws.ToInt32(ngDesc.Nodegroup.ScalingConfig.DesiredSize)),
			MaxCapacity:        int64(aws.ToInt32(ngDesc.Nodegroup.ScalingConfig.MaxSize)),
			MinCapacity:        int64(aws.ToInt32(ngDesc.Nodegroup.ScalingConfig.MinSize)),
			Version:            aws.ToString(ngDesc.Nodegroup.Version),
			Status:             string(ngDesc.Nodegroup.Status),
			BootstrapArguments: bootstrapArguments,
		}
	}

	return ngList, nil
}

func MatchBootstrapRegex(ngList []EKSNodeGroupInfo, pattern string) ([]EKSNodeGroupInfo, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	for i := range ngList {
		ngList[i].BootstrapMatch = re.FindString(ngList[i].BootstrapArguments)
	}
	return ngList, nil
}

var kubeletFlagsRegex = regexp.MustCompile(`(?m)^\s*-\s+(--\S.*)$`)

func extractKubeletFlags(userData string) string {
	var flags []string
	for _, line := range strings.Split(userData, "\n") {
		m := kubeletFlagsRegex.FindStringSubmatch(line)
		if m != nil {
			flags = append(flags, strings.TrimSpace(m[1]))
		}
	}
	return strings.Join(flags, " ")
}
