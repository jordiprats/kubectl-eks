# kubectl-eks

A kubectl plugin for **managing Amazon EKS clusters**. This plugin provides convenient commands for listing, inspecting, and switching between EKS clusters.

<!-- vscode-markdown-toc -->
* [Prerequisites](#Prerequisites)
* [Installation](#Installation)
	* [Binary Installation](#BinaryInstallation)
* [Documentation](#Documentation)
* [Quick Start](#QuickStart)

<!-- vscode-markdown-toc-config
	numbering=false
	autoSave=true
	/vscode-markdown-toc-config -->
<!-- /vscode-markdown-toc -->

## <a name='Prerequisites'></a>Prerequisites

- **AWS CLI:** Ensure that `aws` is installed and configured with the appropriate credentials and region.
- **kubectl:** Ensure you have `kubectl` installed and configured.

## <a name='Installation'></a>Installation

We provide pre-built binaries for Linux and macOS (arm64 and x86-64). Follow the instructions below to install the plugin on your system.

### <a name='BinaryInstallation'></a>Binary Installation

1. Download the binary for your operating system from the [releases page](https://github.com/jordiprats/kubectl-eks/releases).

2. Install the binary:

    ```bash
    chmod +x kubectl-eks
    mv kubectl-eks /usr/local/bin/
    ```

3. Verify installation:

    ```bash
    kubectl eks --help
    ```

## <a name='Documentation'></a>Documentation

Full command reference documentation is available in the [docs/](docs/) directory:

- [kubectl eks](docs/kubectl-eks.md) - Main command and cluster information
- [kubectl eks list](docs/kubectl-eks_list.md) - List all EKS clusters
- [kubectl eks use](docs/kubectl-eks_use.md) - Switch to a different cluster
- [kubectl eks cache](docs/kubectl-eks_cache.md) - Manage the local cluster cache
- [kubectl eks mget](docs/kubectl-eks_mget.md) - Get resources from multiple clusters
- [kubectl eks mcheck](docs/kubectl-eks_mcheck.md) - Check health status of resources across clusters
- [kubectl eks nodes](docs/kubectl-eks_nodes.md) - List nodes with EC2 instance details
- [kubectl eks stats](docs/kubectl-eks_stats.md) - Get cluster statistics
- [kubectl eks nodegroups](docs/kubectl-eks_nodegroups.md) - List cluster node groups
- [kubectl eks insights](docs/kubectl-eks_insights.md) - Get cluster insights
- [kubectl eks updates](docs/kubectl-eks_updates.md) - Check for updates
- [kubectl eks events](docs/kubectl-eks_events.md) - Show Kubernetes events across namespaces
- [kubectl eks stacks](docs/kubectl-eks_stacks.md) - Get CloudFormation stacks
- [kubectl eks quotas](docs/kubectl-eks_quotas.md) - Show ResourceQuota usage per namespace
- [kubectl eks whoami](docs/kubectl-eks_whoami.md) - Show current AWS IAM identity and Kubernetes user mapping
- [kubectl eks irsa](docs/kubectl-eks_irsa.md) - List service accounts with IRSA annotations and their IAM roles
- [kubectl eks pod-identity](docs/kubectl-eks_pod-identity.md) - List EKS Pod Identity associations
- [kubectl eks kube2iam](docs/kubectl-eks_kube2iam.md) - List pods with kube2iam annotations and their IAM roles
- [kubectl eks fargate-profiles](docs/kubectl-eks_fargate-profiles.md) - List Fargate profiles and their selectors
- [kubectl eks karpenter](docs/kubectl-eks_karpenter.md) - Karpenter resource management commands
- [kubectl eks aws-profile](docs/kubectl-eks_aws-profile.md) - Get AWS profile
- [kubectl eks mcp-server](docs/kubectl-eks_mcp-server.md) - Start an MCP server for AI assistant integration
- [kubectl eks completion](docs/kubectl-eks_completion.md) - Generate shell autocompletion

Browse all commands and their options in the [docs](docs/) folder.

## MCP Server (AI Integration)

kubectl-eks includes a built-in [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server that exposes all functionality as tools for AI assistants like Claude Desktop, VS Code Copilot, Cursor, and others.

### Usage

```bash
# Start the MCP server (used by AI clients, not typically run manually)
kubectl-eks mcp-server
```

### Client Configuration

**Claude Desktop** (`~/.claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kubectl-eks": {
      "command": "kubectl-eks",
      "args": ["mcp-server"]
    }
  }
}
```

**VS Code** (`.vscode/mcp.json`):

```json
{
  "servers": {
    "kubectl-eks": {
      "command": "kubectl-eks",
      "args": ["mcp-server"]
    }
  }
}
```

**Cursor** (`~/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "kubectl-eks": {
      "command": "kubectl-eks",
      "args": ["mcp-server"]
    }
  }
}
```

### Available MCP Tools

| Tool | Description |
|------|-------------|
| `list_clusters` | List EKS clusters across accounts/regions with optional filters |
| `get_current_cluster` | Show current cluster context (name, region, profile, version) |
| `use_cluster` | Switch kubectl context to a different EKS cluster |
| `get_nodes` | List Kubernetes nodes with instance details, CPU/memory, pressure |
| `get_nodegroups` | List EKS managed node groups with scaling config |
| `get_events` | Show Kubernetes events (scheduling, errors, warnings) |
| `get_insights` | Show EKS insights, deprecation warnings, upgrade blockers |
| `whoami` | Show current AWS IAM identity and Kubernetes user mapping |
| `get_stats` | Show cluster statistics (node/pod counts, health) |
| `get_karpenter_nodepools` | List Karpenter NodePools with config and limits |
| `get_karpenter_nodeclaims` | List active Karpenter NodeClaims |
| `get_karpenter_drift` | List drifted Karpenter nodes/NodeClaims |
| `get_pod_identity` | List EKS Pod Identity associations |
| `get_irsa` | List IRSA service accounts and IAM role ARNs |
| `get_updates` | Check for available Kubernetes/add-on updates |
| `get_fargate_profiles` | List Fargate profiles and selectors |
| `get_quotas` | Show ResourceQuota usage per namespace |

Most multi-cluster tools accept optional filter parameters: `profile`, `profile_contains`, `cluster_contains`, `cluster_not_contains`, `region`, and `version`.

## <a name='QuickStart'></a>Quick Start

```bash
# List all EKS clusters
kubectl eks list

# Filter clusters by name
kubectl eks list --name-contains prod

# Switch to a specific cluster
kubectl eks use my-cluster

# Pre-warm the cache for faster subsequent commands
kubectl eks cache refresh

# Show cached clusters
kubectl eks cache show

# Get resources from multiple clusters
kubectl eks mget pods -q prod

# Filter resources by field value
kubectl eks mget pods --filter status.phase=Running

# View cluster statistics
kubectl eks stats

# Get insights about a cluster
kubectl eks insights my-cluster
```