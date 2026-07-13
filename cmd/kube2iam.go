package cmd

import (
	"context"
	"log"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/jordiprats/kubectl-eks/pkg/k8s"
	"github.com/jordiprats/kubectl-eks/pkg/printutils"
	"github.com/spf13/cobra"
)

var kube2iamCmd = &cobra.Command{
	Use:     "kube2iam",
	Aliases: []string{"k2iam", "k2i"},
	Short:   "List pods with kube2iam annotations and their IAM roles",
	Long: `List pods with kube2iam annotations and their associated IAM role ARNs.

Shows the pod name, namespace, and associated IAM role from the
iam.amazonaws.com/role annotation.`,
	Example: `  # List all pods with kube2iam annotations
  kubectl eks kube2iam

  # List kube2iam pods in specific namespace
  kubectl eks kube2iam -n production

  # List kube2iam pods across all namespaces
  kubectl eks kube2iam -A`,
	Run: func(cmd *cobra.Command, args []string) {
		clusterInfo, err := GetCurrentClusterInfo()
		if err != nil {
			log.Fatalf("Error getting current cluster info: %v", err)
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		allNamespaces, _ := cmd.Flags().GetBool("all-namespaces")
		noHeaders, _ := cmd.Flags().GetBool("no-headers")

		// Default to all namespaces
		if !allNamespaces && namespace == "" {
			allNamespaces = true
		}

		if allNamespaces {
			namespace = ""
		}

		pods, err := k8s.GetPodsWithKube2IAM(context.Background(), namespace)
		if err != nil {
			log.Fatalf("Error getting pods: %v", err)
		}

		if len(pods) == 0 {
			if namespace == "" {
				log.Println("No pods with kube2iam annotations found")
			} else {
				log.Printf("No pods with kube2iam annotations found in namespace: %s\n", namespace)
			}
			return
		}

		kube2iamInfos := make([]data.Kube2IAMInfo, 0)
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

		if len(kube2iamInfos) == 0 {
			log.Println("No pods with kube2iam annotations found")
			return
		}

		printutils.PrintKube2IAM(noHeaders, kube2iamInfos...)
	},
}

func init() {
	kube2iamCmd.Flags().StringP("namespace", "n", "", "Namespace to show kube2iam for")
	kube2iamCmd.Flags().BoolP("all-namespaces", "A", false, "Show kube2iam across all namespaces (default)")
	rootCmd.AddCommand(kube2iamCmd)
}
