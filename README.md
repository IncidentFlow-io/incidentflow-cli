# incidentflow-cli

Official CLI for [IncidentFlow](https://incidentflow.io) — install and manage Kubernetes Agents, check cluster status, and run diagnostics, all from your terminal or through an AI assistant.

---

## Overview

`incidentflow` is a single binary that:

- Authenticates with the IncidentFlow platform
- Detects your Kubernetes context and checks connectivity
- Creates a registration token via the platform API
- Installs the IncidentFlow Agent using Helm
- Waits for the deployment to roll out
- Confirms the cluster is **Online** in IncidentFlow

The same installation logic is also exposed as MCP tools (`incidentflow.install_cluster`, etc.) so Claude Code, Cursor, Windsurf, ChatGPT with MCP, and Codex CLI can all drive cluster installs through the exact same Go code path.

---

## Requirements

| Tool | Version | Notes |
|---|---|---|
| Go | 1.22+ | For building from source |
| Helm | 3.x | Must be in `$PATH` |
| kubectl | any | Kubeconfig must point to a target cluster |
| Kubernetes | 1.24+ | Cluster admin or equivalent permissions |

---

## Build from source

```bash
git clone https://github.com/incidentflow/incidentflow-cli.git
cd incidentflow-cli
make install
```

`make install` runs `GOWORK=off go install ./cmd/incidentflow` and places the binary in your `$GOPATH/bin` (usually `~/go/bin`). Make sure that directory is in your `$PATH`.

To build a binary without installing:

```bash
make build
# binary is at ./bin/incidentflow
```

To build manually:

```bash
GOWORK=off go build -o ./bin/incidentflow ./cmd/incidentflow
```

---

## Quick start

### 1. Verify the binary works

```bash
incidentflow --help
```

```
incidentflow is the official CLI for IncidentFlow.

Install and manage IncidentFlow Agents in Kubernetes clusters,
monitor cluster status, and interact with the IncidentFlow platform.

Usage:
  incidentflow [command]

Available Commands:
  cluster     Manage Kubernetes clusters
  login       Authenticate with IncidentFlow
  logout      Remove local credentials
  whoami      Show current authenticated user

Flags:
  -h, --help   help for incidentflow
```

### 2. Log in

```bash
incidentflow login
```

You will be prompted for your workspace slug and an API token (`if_pat_xxx`).  
Credentials are saved to `~/.incidentflow/config.yaml` (mode `0600`).

To pass credentials non-interactively (CI/scripts):

```bash
incidentflow login --workspace myworkspace --token if_pat_xxx
```

### 3. Verify authentication

```bash
incidentflow whoami
```

```
Email:     you@example.com
Workspace: myworkspace
API URL:   https://platform-api.incidentflow.io
```

---

## Installing an Agent into a cluster

Point `kubectl` at your target cluster, then run:

```bash
incidentflow cluster install --name production-eu-west-1
```

The CLI will run through all preflight checks and print live progress:

```
Installing IncidentFlow Agent

✓ Kubernetes context detected: production
✓ Kubernetes API reachable
✓ Helm detected: v3.15.0
✓ Permissions verified
✓ Registration token created
✓ Helm release installed
✓ Deployment available
✓ Agent registered
✓ Heartbeat received

Cluster production-eu-west-1 is Online.
```

### Full install options

```bash
incidentflow cluster install \
  --name production-eu-west-1 \
  --display-name "Production EU West 1" \
  --namespace incidentflow-agent \
  --platform-url https://platform-api.incidentflow.io \
  --gateway-url wss://gateway.incidentflow.io/agent-gateway/agents/ws \
  --chart oci://ghcr.io/incidentflow-io/charts/incidentflow-k8s-agent \
  --wait
```

| Flag | Default | Description |
|---|---|---|
| `--name` | *(required)* | Unique cluster identifier used in IncidentFlow |
| `--display-name` | same as `--name` | Human-readable label shown in the UI |
| `--namespace` | `incidentflow-agent` | Kubernetes namespace to install into |
| `--platform-url` | from config | IncidentFlow platform API URL |
| `--gateway-url` | `wss://gateway.incidentflow.io/...` | WebSocket gateway URL |
| `--chart` | `oci://ghcr.io/...` | Helm chart OCI reference |
| `--wait` | `true` | Wait for agent to become Online before exiting |

---

## Cluster status

List all registered clusters:

```bash
incidentflow cluster status
```

Check a specific cluster:

```bash
incidentflow cluster status --name production-eu-west-1
```

```
Cluster:        production-eu-west-1
Status:         online
Last heartbeat: 12s ago
Agent version:  v0.1.0
Namespace:      incidentflow-agent
```

---

## Diagnostics

If an agent is not connecting or behaving unexpectedly:

```bash
incidentflow cluster diagnose --name production-eu-west-1
```

This checks:

- Helm release status
- Kubernetes Deployment readiness
- Pod phase and names
- Warning events in the namespace
- Platform API agent status and last heartbeat

Example output:

```
Diagnostics: production-eu-west-1

Helm release:
  incidentflow-k8s-agent  incidentflow-agent  deployed

Deployment:
  1/1 ready

Pods:
  incidentflow-k8s-agent-7d9f8b-xkp2q  Running

Warning events:
  no warnings

Platform status:
  Status: online
  Last heartbeat: 8s ago
  Agent version: v0.1.0
```

---

## Uninstall

Remove the Helm release from a cluster:

```bash
incidentflow cluster uninstall --name production-eu-west-1
```

Also deregister the cluster from the platform:

```bash
incidentflow cluster uninstall --name production-eu-west-1 --revoke-credentials
```

---

## Configuration file

Stored at `~/.incidentflow/config.yaml`:

```yaml
api_url: https://platform-api.incidentflow.io
workspace: myworkspace
token: if_pat_xxx
```

Environment variables override the config file (prefix `INCIDENTFLOW_`):

```bash
export INCIDENTFLOW_TOKEN=if_pat_xxx
export INCIDENTFLOW_WORKSPACE=myworkspace
```

---

## Help for any command

Every command and subcommand accepts `--help`:

```bash
incidentflow --help
incidentflow cluster --help
incidentflow cluster install --help
incidentflow cluster status --help
incidentflow cluster diagnose --help
incidentflow cluster uninstall --help
```

---

## MCP tools

The `pkg/mcp` package exposes all cluster operations as MCP tool handlers that share the **same Go installer code** as the CLI. No installation logic is duplicated.

| MCP tool | Equivalent CLI command |
|---|---|
| `incidentflow.install_cluster` | `cluster install` |
| `incidentflow.cluster_status` | `cluster status` |
| `incidentflow.list_clusters` | `cluster status` (all) |
| `incidentflow.uninstall_cluster` | `cluster uninstall` |
| `incidentflow.diagnose_cluster` | `cluster diagnose` |
| `incidentflow.upgrade_agent` | *(helm upgrade only)* |
| `incidentflow.rotate_agent_credentials` | *(token rotation)* |

When an AI assistant (Claude Code, Cursor, ChatGPT with MCP) is asked to install an IncidentFlow Agent, it calls `incidentflow.install_cluster` which internally runs the exact same `InstallCluster()` function defined in `internal/install/installer.go`.

---

## Repository structure

```
incidentflow-cli/
  cmd/incidentflow/       main.go entry point
  internal/
    api/                  Platform API client (auth, tokens, agents)
    cli/                  Cobra commands (login, cluster install/status/...)
    config/               ~/.incidentflow/config.yaml
    helm/                 Helm client (upgrade, uninstall, status)
    install/              Core installer — single source of truth
    kube/                 Kubernetes client (context, RBAC check, rollout wait)
    output/               Terminal printer (✓/✗ colors, JSON mode)
  pkg/mcp/                MCP tool handlers
  Makefile
```

---

## Error messages

The CLI returns clear, actionable errors.

**Helm not installed:**
```
Helm is not installed.

Install Helm 3 and run again:
  https://helm.sh/docs/intro/install/
```

**No Kubernetes context:**
```
No Kubernetes context found.

Run:
  kubectl config get-contexts
  kubectl config use-context <context>
```

**Insufficient permissions:**
```
IncidentFlow could not create resources in the cluster.

Required permissions:
  - create namespaces
  - create deployments
  - create secrets
  - create serviceaccounts
  - create clusterroles
  - create clusterrolebindings
```

**Agent did not come Online:**
```
Agent was installed but did not become Online.

Run:
  incidentflow cluster diagnose --name production-eu-west-1
```
