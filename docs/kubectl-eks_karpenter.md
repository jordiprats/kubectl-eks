## kubectl-eks karpenter

Karpenter resource management commands

### Synopsis

Manage and inspect Karpenter resources across EKS clusters.

Provides commands to list and inspect Karpenter NodePools, NodeClaims,
AMI usage, and drift status.

### Examples

```
  # List Karpenter NodePools across clusters
  kubectl eks karpenter nodepools
  
  # List active NodeClaims
  kubectl eks karpenter nodeclaims
  
  # Check AMI usage per NodePool
  kubectl eks karpenter ami
  
  # List drifted nodes/nodeclaims
  kubectl eks karpenter drift
```

### Options

```
  -h, --help   help for karpenter
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
      --proxy-url string               Proxy URL to use for requests to the API server
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --verbose                        Show verbose discovery warnings and diagnostics
```

### SEE ALSO

* [kubectl-eks](kubectl-eks.md)	 - A kubectl plugin for managing Amazon EKS clusters
* [kubectl-eks karpenter ami](kubectl-eks_karpenter_ami.md)	 - Show AMI usage across Karpenter NodePools
* [kubectl-eks karpenter drift](kubectl-eks_karpenter_drift.md)	 - List drifted Karpenter nodes and NodeClaims
* [kubectl-eks karpenter nodeclaims](kubectl-eks_karpenter_nodeclaims.md)	 - List Karpenter NodeClaims across clusters
* [kubectl-eks karpenter nodepools](kubectl-eks_karpenter_nodepools.md)	 - List Karpenter NodePools across clusters

