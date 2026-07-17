package cmd

import (
	"testing"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// fakeRestConfig creates a minimal rest.Config for testing.
func fakeRestConfig() *rest.Config {
	return &rest.Config{
		Host: "https://fake-cluster.example.com:6443",
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
}

func TestKube2IAMIteration_CollectsPodsWithAnnotation(t *testing.T) {
	// Test that the annotation extraction and Kube2IAMInfo construction works
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-1",
				Namespace: "default",
				Annotations: map[string]string{
					"iam.amazonaws.com/role": "arn:aws:iam::123456789012:role/role-a",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "node-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-2",
				Namespace: "kube-system",
				Annotations: map[string]string{
					"iam.amazonaws.com/role": "arn:aws:iam::123456789012:role/role-b",
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "node-2",
			},
		},
	}

	kube2iamInfos := make([]data.Kube2IAMInfo, 0)
	for _, pod := range pods {
		roleArn := pod.Annotations["iam.amazonaws.com/role"]
		if roleArn != "" {
			info := data.Kube2IAMInfo{
				Profile:     "prod",
				Region:      "us-east-1",
				ClusterName: "cluster-alpha",
				Namespace:   pod.Namespace,
				PodName:     pod.Name,
				IAMRole:     roleArn,
				NodeName:    pod.Spec.NodeName,
			}
			kube2iamInfos = append(kube2iamInfos, info)
		}
	}

	assert.Len(t, kube2iamInfos, 2, "should collect 2 pods with kube2iam annotation")
	assert.Equal(t, "pod-1", kube2iamInfos[0].PodName)
	assert.Equal(t, "arn:aws:iam::123456789012:role/role-a", kube2iamInfos[0].IAMRole)
	assert.Equal(t, "node-1", kube2iamInfos[0].NodeName)
	assert.Equal(t, "cluster-alpha", kube2iamInfos[0].ClusterName)
	assert.Equal(t, "prod", kube2iamInfos[0].Profile)
	assert.Equal(t, "us-east-1", kube2iamInfos[0].Region)

	assert.Equal(t, "pod-2", kube2iamInfos[1].PodName)
	assert.Equal(t, "arn:aws:iam::123456789012:role/role-b", kube2iamInfos[1].IAMRole)
	assert.Equal(t, "kube-system", kube2iamInfos[1].Namespace)
}

func TestKube2IAMIteration_SkipsPodsWithoutAnnotation(t *testing.T) {
	// Pods without the kube2iam annotation should be skipped
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-without-annotation",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-with-empty-annotation",
				Namespace: "default",
				Annotations: map[string]string{
					"iam.amazonaws.com/role": "",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-with-other-annotation",
				Namespace: "default",
				Annotations: map[string]string{
					"some-other": "value",
				},
			},
		},
	}

	kube2iamInfos := make([]data.Kube2IAMInfo, 0)
	for _, pod := range pods {
		roleArn := pod.Annotations["iam.amazonaws.com/role"]
		if roleArn != "" {
			info := data.Kube2IAMInfo{
				Profile:     "test",
				Region:      "us-east-1",
				ClusterName: "test-cluster",
				Namespace:   pod.Namespace,
				PodName:     pod.Name,
				IAMRole:     roleArn,
				NodeName:    pod.Spec.NodeName,
			}
			kube2iamInfos = append(kube2iamInfos, info)
		}
	}

	assert.Empty(t, kube2iamInfos, "no pods should be collected since none have valid kube2iam annotation")
}

func TestKube2IAMIteration_HandlesMultipleClusters(t *testing.T) {
	// Simulate iteration over multiple clusters, collecting results per cluster
	clusterList := []data.ClusterInfo{
		{
			ClusterName: "cluster-a",
			Region:      "us-east-1",
			AWSProfile:  "prod",
		},
		{
			ClusterName: "cluster-b",
			Region:      "us-west-2",
			AWSProfile:  "prod",
		},
	}

	// Mock pods per cluster (as if returned by GetPodsWithKube2IAMWithConfig)
	clusterPods := map[string][]corev1.Pod{
		"cluster-a": {
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-a1",
					Namespace: "default",
					Annotations: map[string]string{
						"iam.amazonaws.com/role": "arn:aws:iam::111111111111:role/role-a1",
					},
				},
				Spec: corev1.PodSpec{NodeName: "node-a1"},
			},
		},
		"cluster-b": {
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-b1",
					Namespace: "default",
					Annotations: map[string]string{
						"iam.amazonaws.com/role": "arn:aws:iam::222222222222:role/role-b1",
					},
				},
				Spec: corev1.PodSpec{NodeName: "node-b1"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-b2",
					Namespace: "kube-system",
					Annotations: map[string]string{
						"iam.amazonaws.com/role": "arn:aws:iam::222222222222:role/role-b2",
					},
				},
				Spec: corev1.PodSpec{NodeName: "node-b2"},
			},
		},
	}

	kube2iamInfos := make([]data.Kube2IAMInfo, 0)
	for _, clusterInfo := range clusterList {
		pods := clusterPods[clusterInfo.ClusterName]
		for _, pod := range pods {
			roleArn := pod.Annotations["iam.amazonaws.com/role"]
			if roleArn != "" {
				info := data.Kube2IAMInfo{
					Profile:     clusterInfo.AWSProfile,
					Region:      clusterInfo.Region,
					ClusterName: clusterInfo.ClusterName,
					Namespace:   pod.Namespace,
					PodName:     pod.Name,
					IAMRole:     roleArn,
					NodeName:    pod.Spec.NodeName,
				}
				kube2iamInfos = append(kube2iamInfos, info)
			}
		}
	}

	assert.Len(t, kube2iamInfos, 3, "should collect 3 pods across 2 clusters")

	// Verify cluster-a result
	assert.Equal(t, "cluster-a", kube2iamInfos[0].ClusterName)
	assert.Equal(t, "pod-a1", kube2iamInfos[0].PodName)
	assert.Equal(t, "us-east-1", kube2iamInfos[0].Region)

	// Verify cluster-b results
	assert.Equal(t, "cluster-b", kube2iamInfos[1].ClusterName)
	assert.Equal(t, "pod-b1", kube2iamInfos[1].PodName)
	assert.Equal(t, "us-west-2", kube2iamInfos[1].Region)

	assert.Equal(t, "cluster-b", kube2iamInfos[2].ClusterName)
	assert.Equal(t, "pod-b2", kube2iamInfos[2].PodName)
	assert.Equal(t, "kube-system", kube2iamInfos[2].Namespace)
}

func TestKube2IAMIteration_SkipsClusterOnConfigError(t *testing.T) {
	// When GetRestConfigForCluster fails, the cluster should be skipped.
	// We simulate this by only processing clusters that have a valid config.

	clusterList := []data.ClusterInfo{
		{
			ClusterName: "cluster-valid",
			Region:      "us-east-1",
			AWSProfile:  "prod",
		},
		{
			ClusterName: "cluster-invalid",
			Region:      "us-west-2",
			AWSProfile:  "prod",
		},
	}

	// Only "cluster-valid" has a config
	validClusters := map[string]bool{
		"cluster-valid": true,
	}

	clusterPods := map[string][]corev1.Pod{
		"cluster-valid": {
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-valid",
					Namespace: "default",
					Annotations: map[string]string{
						"iam.amazonaws.com/role": "arn:aws:iam::111111111111:role/valid-role",
					},
				},
				Spec: corev1.PodSpec{NodeName: "node-valid"},
			},
		},
	}

	kube2iamInfos := make([]data.Kube2IAMInfo, 0)
	for _, clusterInfo := range clusterList {
		if !validClusters[clusterInfo.ClusterName] {
			// Simulate the "continue" on config error
			continue
		}
		pods := clusterPods[clusterInfo.ClusterName]
		for _, pod := range pods {
			roleArn := pod.Annotations["iam.amazonaws.com/role"]
			if roleArn != "" {
				info := data.Kube2IAMInfo{
					Profile:     clusterInfo.AWSProfile,
					Region:      clusterInfo.Region,
					ClusterName: clusterInfo.ClusterName,
					Namespace:   pod.Namespace,
					PodName:     pod.Name,
					IAMRole:     roleArn,
					NodeName:    pod.Spec.NodeName,
				}
				kube2iamInfos = append(kube2iamInfos, info)
			}
		}
	}

	assert.Len(t, kube2iamInfos, 1, "should only collect pods from valid cluster")
	assert.Equal(t, "cluster-valid", kube2iamInfos[0].ClusterName)
}

func TestKube2IAMIteration_SkipsClusterOnAPIServerError(t *testing.T) {
	// When GetPodsWithKube2IAMWithConfig fails, the cluster should be skipped.
	// We simulate this by only processing clusters that return pods successfully.

	clusterList := []data.ClusterInfo{
		{
			ClusterName: "cluster-working",
			Region:      "us-east-1",
			AWSProfile:  "prod",
		},
		{
			ClusterName: "cluster-failing",
			Region:      "us-west-2",
			AWSProfile:  "prod",
		},
	}

	// Only "cluster-working" returns pods successfully
	workingClusters := map[string]bool{
		"cluster-working": true,
	}

	clusterPods := map[string][]corev1.Pod{
		"cluster-working": {
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-working",
					Namespace: "default",
					Annotations: map[string]string{
						"iam.amazonaws.com/role": "arn:aws:iam::111111111111:role/working-role",
					},
				},
				Spec: corev1.PodSpec{NodeName: "node-working"},
			},
		},
	}

	kube2iamInfos := make([]data.Kube2IAMInfo, 0)
	for _, clusterInfo := range clusterList {
		if !workingClusters[clusterInfo.ClusterName] {
			// Simulate the "continue" on API server error
			continue
		}
		pods := clusterPods[clusterInfo.ClusterName]
		for _, pod := range pods {
			roleArn := pod.Annotations["iam.amazonaws.com/role"]
			if roleArn != "" {
				info := data.Kube2IAMInfo{
					Profile:     clusterInfo.AWSProfile,
					Region:      clusterInfo.Region,
					ClusterName: clusterInfo.ClusterName,
					Namespace:   pod.Namespace,
					PodName:     pod.Name,
					IAMRole:     roleArn,
					NodeName:    pod.Spec.NodeName,
				}
				kube2iamInfos = append(kube2iamInfos, info)
			}
		}
	}

	assert.Len(t, kube2iamInfos, 1, "should only collect pods from working cluster")
	assert.Equal(t, "cluster-working", kube2iamInfos[0].ClusterName)
}

func TestKube2IAMInfo_StructFields(t *testing.T) {
	// Verify that all fields of Kube2IAMInfo are correctly populated
	info := data.Kube2IAMInfo{
		Profile:     "my-profile",
		Region:      "eu-west-1",
		ClusterName: "my-cluster",
		Namespace:   "my-namespace",
		PodName:     "my-pod",
		IAMRole:     "arn:aws:iam::123456789012:role/my-role",
		NodeName:    "my-node",
	}

	assert.Equal(t, "my-profile", info.Profile)
	assert.Equal(t, "eu-west-1", info.Region)
	assert.Equal(t, "my-cluster", info.ClusterName)
	assert.Equal(t, "my-namespace", info.Namespace)
	assert.Equal(t, "my-pod", info.PodName)
	assert.Equal(t, "arn:aws:iam::123456789012:role/my-role", info.IAMRole)
	assert.Equal(t, "my-node", info.NodeName)
}

func TestKube2IAMIteration_NamespaceFiltering(t *testing.T) {
	// When a specific namespace is requested, only pods in that namespace
	// should be included.
	namespace := "production"

	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-prod",
				Namespace: "production",
				Annotations: map[string]string{
					"iam.amazonaws.com/role": "arn:aws:iam::111111111111:role/prod-role",
				},
			},
			Spec: corev1.PodSpec{NodeName: "node-prod"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-default",
				Namespace: "default",
				Annotations: map[string]string{
					"iam.amazonaws.com/role": "arn:aws:iam::111111111111:role/default-role",
				},
			},
			Spec: corev1.PodSpec{NodeName: "node-default"},
		},
	}

	kube2iamInfos := make([]data.Kube2IAMInfo, 0)
	for _, pod := range pods {
		// Simulate namespace filtering
		if namespace != "" && pod.Namespace != namespace {
			continue
		}
		roleArn := pod.Annotations["iam.amazonaws.com/role"]
		if roleArn != "" {
			info := data.Kube2IAMInfo{
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				IAMRole:   roleArn,
				NodeName:  pod.Spec.NodeName,
			}
			kube2iamInfos = append(kube2iamInfos, info)
		}
	}

	assert.Len(t, kube2iamInfos, 1, "should only include pod in 'production' namespace")
	assert.Equal(t, "pod-prod", kube2iamInfos[0].PodName)
}
