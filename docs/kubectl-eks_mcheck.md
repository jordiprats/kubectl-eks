## kubectl-eks mcheck

Check health status of resources across multiple clusters

### Synopsis

Check the readiness/health status of Kubernetes resources across all clusters that match a filter.

Checks the following resources:
  - Pods: Running or Completed status (excludes Completed from unhealthy)
  - Deployments: Ready replicas match desired replicas
  - StatefulSets: Ready replicas match desired replicas
  - DaemonSets: Ready nodes match desired nodes
  - ReplicaSets: Ready replicas match desired replicas

By default, checks all namespaces and only shows unhealthy resources.
Use -n to check a specific namespace, use --all to show healthy resources too.

```
kubectl-eks mcheck [flags]
```

### Examples

```
  # Check all resources across clusters (all namespaces, only unhealthy)
  kubectl eks mcheck

  # Check resources in specific namespace only
  kubectl eks mcheck -n kube-system

  # Show all resources including healthy ones
  kubectl eks mcheck --all

  # Filter clusters
  kubectl eks mcheck --cluster-contains prod

  # Check specific resource types
  kubectl eks mcheck --pods --deployments

  # Summary only (no individual resources)
  kubectl eks mcheck --summary
```

### Options

```
      --all                           Show all resources including healthy ones
  -c, --cluster-contains string       Cluster name contains string
  -x, --cluster-not-contains string   Cluster name does not contain string
      --daemonsets                    Check only daemonsets
      --deployments                   Check only deployments
  -h, --help                          help for mcheck
  -n, --namespace string              Kubernetes namespace (default: all namespaces)
      --no-headers                    Don't print headers
      --pods                          Check only pods
  -p, --profile string                AWS profile to use
  -q, --profile-contains string       AWS profile contains string
  -u, --refresh                       Do not use cached data, refresh from AWS
  -r, --region string                 AWS region to use
      --replicasets                   Check only replicasets
      --statefulsets                  Check only statefulsets
      --summary                       Show health summary
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
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use
      --verbose                        Show verbose discovery warnings and diagnostics
```

### SEE ALSO

* [kubectl-eks](kubectl-eks.md)	 - A kubectl plugin for managing Amazon EKS clusters

