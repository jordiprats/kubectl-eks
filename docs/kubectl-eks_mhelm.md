## kubectl-eks mhelm

List Helm releases from multiple clusters

### Synopsis

List Helm releases from all EKS clusters that match a filter.
By default, releases are listed from the default namespace. Use -n to query a
specific namespace or -A to query all namespaces.

```
kubectl-eks mhelm [release-name] [flags]
```

### Examples

```
  # List releases in the default namespace
  kubectl eks mhelm

  # List releases in a specific namespace
  kubectl eks mhelm -n kube-system

  # List releases in all namespaces
  kubectl eks mhelm -A

	# Filter by chart and app version substrings
	kubectl eks mhelm -A --chart-version 1.12 --app-version 2.4

	# Exclude chart or app version substrings
	kubectl eks mhelm -A --not-chart-version beta --not-app-version 1.

  # Find a release by exact name across production clusters
  kubectl eks mhelm ingress-nginx -A --cluster-contains prod
```

### Options

```
  -A, --all-namespaces                Query all Kubernetes namespaces
      --app-version string            Filter by Helm app version substring
      --chart-version string          Filter by Helm chart version substring
  -c, --cluster-contains string       Filter by cluster name substring
  -x, --cluster-not-contains string   Exclude clusters whose name contains this substring
  -h, --help                          help for mhelm
  -n, --namespace string              Kubernetes namespace
      --no-headers                    Don't print headers
      --not-app-version string        Exclude Helm app version substring
      --not-chart-version string      Exclude Helm chart version substring
  -V, --not-version string            Exclude clusters with this EKS version
  -p, --profile string                Filter by exact AWS profile name (account)
  -q, --profile-contains string       Filter by AWS profile name (account) substring
  -Q, --profile-not-contains string   Exclude profiles whose name contains this substring
  -u, --refresh                       Do not use cached data, refresh from AWS
  -r, --region string                 Filter by AWS region
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

