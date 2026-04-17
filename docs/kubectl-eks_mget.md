## kubectl-eks mget

Get resources from multiple clusters

### Synopsis

Get Kubernetes resources from all clusters that match a filter.
Similar to 'kubectl get' but works across multiple EKS clusters.

Supports output formats:
  -o wide          Additional details
  -o json          JSON output
  -o yaml          YAML output
  -o jsonpath=...  Extract specific fields using JSONPath

```
kubectl-eks mget [resource-type] [resource-name] [flags]
```

### Examples

```
  # List all pods across clusters
  kubectl eks mget pods

  # List pods in specific namespace
  kubectl eks mget pods -n kube-system

  # Get a specific deployment
  kubectl eks mget deployment my-app

  # Extract specific fields with JSONPath
  kubectl eks mget pods -o jsonpath='{.spec.dnsPolicy}'
  
  # List deployments with additional details
  kubectl eks mget deployments -o wide
  
  # Filter clusters and resources
  kubectl eks mget pods --cluster-contains prod --resource-starts-with nginx

  # Filter resources by name substring
  kubectl eks mget pods --cluster-contains prod --resource-contains api
  
  # Works with any resource including CRDs
  kubectl eks mget ec2nodeclass -A
```

### Options

```
  -A, --all-namespaces                Query all Kubernetes namespaces
  -c, --cluster-contains string       Cluster name contains string
  -x, --cluster-not-contains string   Cluster name does not contain string
  -h, --help                          help for mget
  -n, --namespace string              Kubernetes namespace
      --no-headers                    Don't print headers
  -o, --output string                 Output format: wide|json|yaml|jsonpath=...
  -p, --profile string                AWS profile to use
  -q, --profile-contains string       AWS profile contains string
  -u, --refresh                       Do not use cached data, refresh from AWS
  -r, --region string                 AWS region to use
      --resource-contains string      Filter resources that contain this string
  -w, --resource-starts-with string   Filter resources that start with this string
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

