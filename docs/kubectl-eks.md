## kubectl-eks

A kubectl plugin for managing Amazon EKS clusters

### Synopsis

kubectl-eks provides convenient commands for listing, inspecting, 
and switching between EKS clusters and their associated resources.

Prerequisites:
- AWS CLI installed and configured
- kubectl installed and configured

```
kubectl-eks [flags]
```

### Examples

```
  # Show current cluster info
  kubectl eks
  
  # List all clusters
  kubectl eks list
  
  # Switch to a cluster
  kubectl eks use my-cluster
```

### Options

```
      --as string                      Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --as-uid string                  UID to impersonate for the operation.
      --as-user-extra stringArray      User extras to impersonate for the operation, this flag can be repeated to specify multiple values for the same key.
      --cache-dir string               Default cache directory (default "/Users/jprats/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
      --disable-compression            If true, opt-out of response compression for all requests to the server
  -h, --help                           help for kubectl-eks
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string               If present, the namespace scope for this CLI request
      --no-headers                     When using the default or custom-column output format, don't print headers (default print headers)
  -u, --refresh                        Do not use cached data, refresh from AWS
  -r, --region string                  Switch to the same cluster in a different region
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --verbose                        Show verbose discovery warnings and diagnostics
```

### SEE ALSO

* [kubectl-eks aws-profile](kubectl-eks_aws-profile.md)	 - Get AWS profile
* [kubectl-eks cache](kubectl-eks_cache.md)	 - Manage the local cluster cache
* [kubectl-eks completion](kubectl-eks_completion.md)	 - Generate the autocompletion script for the specified shell
* [kubectl-eks events](kubectl-eks_events.md)	 - Show Kubernetes events across namespaces
* [kubectl-eks fargate-profiles](kubectl-eks_fargate-profiles.md)	 - List EKS Fargate profiles and their selectors
* [kubectl-eks insights](kubectl-eks_insights.md)	 - Show EKS cluster insights and recommendations
* [kubectl-eks irsa](kubectl-eks_irsa.md)	 - List service accounts with IRSA annotations and their IAM roles
* [kubectl-eks karpenter](kubectl-eks_karpenter.md)	 - Karpenter resource management commands
* [kubectl-eks kube2iam](kubectl-eks_kube2iam.md)	 - List pods with kube2iam annotations and their IAM roles
* [kubectl-eks list](kubectl-eks_list.md)	 - List all EKS clusters in your AWS account
* [kubectl-eks mcheck](kubectl-eks_mcheck.md)	 - Check health status of resources across multiple clusters
* [kubectl-eks mcp-server](kubectl-eks_mcp-server.md)	 - Start an MCP (Model Context Protocol) server over stdio
* [kubectl-eks mget](kubectl-eks_mget.md)	 - Get resources from multiple clusters
* [kubectl-eks nodegroups](kubectl-eks_nodegroups.md)	 - List EKS managed node groups
* [kubectl-eks nodes](kubectl-eks_nodes.md)	 - List Kubernetes nodes with EC2 instance details
* [kubectl-eks pod-identity](kubectl-eks_pod-identity.md)	 - List EKS Pod Identity associations from the AWS EKS API
* [kubectl-eks quotas](kubectl-eks_quotas.md)	 - Show ResourceQuota usage per namespace
* [kubectl-eks stacks](kubectl-eks_stacks.md)	 - List CloudFormation stacks associated with EKS clusters
* [kubectl-eks stats](kubectl-eks_stats.md)	 - Show aggregated cluster statistics and resource usage
* [kubectl-eks updates](kubectl-eks_updates.md)	 - Check for available Kubernetes and add-on updates
* [kubectl-eks use](kubectl-eks_use.md)	 - Switch kubectl context to a different EKS cluster
* [kubectl-eks whoami](kubectl-eks_whoami.md)	 - Show current AWS IAM identity and Kubernetes user mapping

