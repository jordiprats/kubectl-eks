package data

// HealthCheckResult contains the health status of a single resource
type HealthCheckResult struct {
	Profile     string
	Region      string
	ClusterName string
	Namespace   string
	Kind        string
	Name        string
	Ready       string
	Status      string
	Message     string
	IsHealthy   bool
}

// ClusterHealthSummary contains aggregated health status for a cluster
type ClusterHealthSummary struct {
	Profile             string
	Region              string
	ClusterName         string
	TotalPods           int
	HealthyPods         int
	TotalDeployments    int
	HealthyDeployments  int
	TotalStatefulSets   int
	HealthyStatefulSets int
	TotalDaemonSets     int
	HealthyDaemonSets   int
	TotalReplicaSets    int
	HealthyReplicaSets  int
	TotalNodes          int
	HealthyNodes        int
	OverallStatus       string
}
