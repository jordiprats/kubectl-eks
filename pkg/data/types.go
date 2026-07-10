package data

import (
	"time"
)

type ClusterInfo struct {
	ClusterName            string
	Namespace              string
	Region                 string
	AWSProfile             string
	AWSAccountID           string
	Status                 string
	Version                string
	Arn                    string
	CreatedAt              string
	NodeCount              int
	NodeReady              int
	NodeNotReady           int
	NodeSchedDisabled      int
	EC2NodeCount           int
	FargateNodeCount       int
	CPUUsedTotal           string
	CPUCapacityTotal       string
	CPUAllocatableTotal    string
	MemoryUsedTotal        string
	MemoryCapacityTotal    string
	MemoryAllocatableTotal string
}

type ClusterNodeInfo struct {
	Profile     string
	Region      string
	ClusterName string
	Node        NodeInfo
}

type NodeInfo struct {
	Name               string
	InstanceType       string
	Compute            string
	ManagedBy          string
	Created            time.Time
	Status             string
	CPUCapacity        string
	CPUAllocatable     string
	CPUUsed            string
	MemoryCapacity     string
	MemoryAllocatable  string
	MemoryUsed         string
	PodsCapacity       string
	PodsAllocatable    string
	PodsUsed           string
	PodsRunning        int
	MemoryPressure     string
	DiskPressure       string
	PIDPressure        string
	NetworkUnavailable string
}

type KubeCtlEksCache struct {
	ClusterByARN map[string]ClusterInfo
	ClusterList  map[string]map[string][]ClusterInfo
}

type JsonPathResult struct {
	Profile     string
	Region      string
	ClusterName string
	Namespace   string
	Resource    string
	Value       string
	Error       string
}

type ResourceResult struct {
	Profile     string
	Region      string
	ClusterName string
	Namespace   string
	Name        string
	Kind        string
	Data        interface{}
	Error       string
	Status      string
}

type AWSProfile struct {
	Name           string
	DefaultRegion  string
	HintEKSRegions []string
}

type AWSConfig struct {
	Profiles map[string]AWSProfile
}

type AMIInfo struct {
	ID              string
	Name            string
	Architecture    string
	State           string
	DeprecationTime string
}

type ClientStat struct {
	LastRequestTime            time.Time `json:"lastRequestTime"`
	NumberOfRequestsLast30Days int64     `json:"numberOfRequestsLast30Days"`
	UserAgent                  string    `json:"userAgent"`
}

type DeprecationDetail struct {
	ClientStats                    []ClientStat `json:"clientStats"`
	ReplacedWith                   string       `json:"replacedWith"`
	StartServingReplacementVersion string       `json:"startServingReplacementVersion"`
	StopServingVersion             string       `json:"stopServingVersion"`
	Usage                          string       `json:"usage"`
}

type CategorySpecificSummary struct {
	DeprecationDetails []DeprecationDetail `json:"deprecationDetails"`
}

type EKSInsightInfo struct {
	Profile        string
	Region         string
	ClusterName    string
	ID             string
	Description    string
	Category       string
	Status         string
	Recommendation string
	Reason         string
	Summary        *CategorySpecificSummary
	AdditionalInfo *map[string]*string
}

type WhoAmIInfo struct {
	AWSProfile  string
	Region      string
	ClusterName string
	AWSArn      string
	AWSAccount  string
	AWSUserId   string
	K8sUsername string
	K8sUID      string
	K8sGroups   []string
}

type ResourceQuotaInfo struct {
	Profile      string
	Region       string
	ClusterName  string
	Namespace    string
	QuotaName    string
	ResourceName string
	Hard         string
	Used         string
}

type EventInfo struct {
	Profile     string
	Region      string
	ClusterName string
	Namespace   string
	LastSeen    time.Time
	Type        string
	Reason      string
	Object      string
	Message     string
	Count       int32
}

type IRSAInfo struct {
	Profile            string
	Region             string
	ClusterName        string
	Namespace          string
	ServiceAccountName string
	IAMRoleARN         string
}

type Kube2IAMInfo struct {
	Profile     string
	Region      string
	ClusterName string
	Namespace   string
	PodName     string
	IAMRole     string
	NodeName    string
}

type PodIdentityInfo struct {
	Profile            string
	Region             string
	ClusterName        string
	Namespace          string
	ServiceAccountName string
	IAMRoleARN         string
	IdentityType       string
}
