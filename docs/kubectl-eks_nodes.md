## kubectl-eks nodes

List Kubernetes nodes with EC2 instance details

### Synopsis

List Kubernetes nodes across EKS clusters with EC2 instance metadata.

Shows node status, instance type, capacity (CPU/memory), provider ID,
and other node attributes. Queries both Kubernetes and AWS APIs for
complete node information.

When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.

```
kubectl-eks nodes [flags]
```

### Examples

```
  # List nodes for current cluster
  kubectl eks nodes

	# List nodes for current cluster with pressure indicators
	kubectl eks nodes -o wide

  # List nodes across clusters matching filter
  kubectl eks nodes --cluster-contains prod

  # List nodes for specific profile
  kubectl eks nodes --profile my-aws-profile

  # List nodes across all clusters in a region
  kubectl eks nodes --region us-west-2
```

### Options

```
  -c, --cluster-contains string       Cluster name contains string
  -x, --cluster-not-contains string   Cluster name does not contain string
  -h, --help                          help for nodes
  -o, --output string                 Output format: wide
  -p, --profile string                AWS profile to use
  -q, --profile-contains string       AWS profile contains string
  -u, --refresh                       Do not use cached data, refresh from AWS
  -r, --region string                 AWS region to use
  -v, --version string                Filter by EKS version
```

### Options inherited from parent commands

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
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string               If present, the namespace scope for this CLI request
      --no-headers                     When using the default or custom-column output format, don't print headers (default print headers)
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --verbose                        Show verbose discovery warnings and diagnostics
```

### SEE ALSO

* [kubectl-eks](kubectl-eks.md)	 - A kubectl plugin for managing Amazon EKS clusters

