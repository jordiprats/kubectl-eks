## kubectl-eks use

Switch kubectl context to a different EKS cluster

### Synopsis

Switch kubectl context to a different EKS cluster by updating kubeconfig.

Accepts either a cluster ARN or partial cluster name. When using a partial
name, the command applies the same cluster filters as 'list' and switches only
when exactly one cluster matches by default.

When multiple clusters match, you can choose one with --oldest or --newest.

If a kubeconfig context for the target cluster already exists and the
credentials are still valid, the switch is performed locally without calling
AWS APIs, making it significantly faster. When credentials have expired or no
matching context exists, a full 'aws eks update-kubeconfig' is performed.

Optionally specify a namespace to set as default, or use a different AWS
profile for authentication.

```
kubectl-eks use [cluster-name-or-arn] [flags]
```

### Options

```
      --bash-comment                  Prefix output with '# ' so it looks like a bash comment
  -c, --cluster-contains string       Cluster name contains string
  -x, --cluster-not-contains string   Cluster name does not contain string
  -h, --help                          help for use
  -n, --namespace string              Set specific namespace for the context
      --newest                        When multiple clusters match, switch to the newest cluster
      --oldest                        When multiple clusters match, switch to the oldest cluster
  -p, --profile string                Set specific AWS profile for the context
  -q, --profile-contains string       AWS profile contains string
  -u, --refresh                       Refresh data from AWS
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

