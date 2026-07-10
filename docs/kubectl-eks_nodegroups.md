## kubectl-eks nodegroups

List EKS managed node groups

### Synopsis

List EKS managed node groups with configuration and status details.

Displays node group name, status, instance types, scaling configuration
(min/max/desired size), AMI type, capacity type (On-Demand/Spot), and
current Kubernetes version.

Use this to audit node group configurations and identify scaling settings.
When cluster filters are provided, queries multiple clusters.
Without filters, queries the current cluster context.

```
kubectl-eks nodegroups [flags]
```

### Examples

```
  # List nodegroups for current cluster
  kubectl eks nodegroups

  # Filter by profile substring
  kubectl eks nodegroups --profile-contains genprod

  # Filter by cluster name substring
  kubectl eks nodegroups --cluster-contains v2-b

  # Exclude clusters by name substring
  kubectl eks nodegroups --cluster-not-contains staging

  # Filter by region
  kubectl eks nodegroups --region us-west-2

  # Combine filters
  kubectl eks nodegroups -q genprod -c v2-b -x orch -r us-west-2

  # Watch nodegroups (refresh every 30s by default)
  kubectl eks nodegroups -w

  # Watch with custom interval
  kubectl eks nodegroups -w 5s
```

### Options

```
  -a, --ami string                    Describe AMI used by the nodegroup
  -c, --cluster-contains string       Filter by cluster name substring
  -x, --cluster-not-contains string   Exclude clusters whose name contains this substring
  -h, --help                          help for nodegroups
  -p, --profile string                Filter by exact AWS profile name (account)
  -q, --profile-contains string       Filter by AWS profile name (account) substring
  -Q, --profile-not-contains string   Exclude profiles whose name contains this substring
  -u, --refresh                       Do not use cached data, refresh from AWS
  -r, --region string                 Filter by AWS region
  -v, --version string                Filter by EKS version
  -w, --watch duration[=30s]          Watch mode: refresh every interval (default 30s, e.g. -w 5s)
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

