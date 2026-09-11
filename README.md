# cka-lab-runner

Practice for the Certified Kubernetes Administrator (CKA) exam by debugging realistic broken scenarios in a local Kubernetes cluster.

## What is this?

A CLI tool that creates broken Kubernetes scenarios for you to fix—just like the CKA exam. It sets up a local cluster, breaks something specific, and lets you practice troubleshooting. When you're done, it verifies you fixed it correctly.

## Quick Start

### Prerequisites

- Docker
- kubectl
- At least one of: kind, k3d, or minikube
- Helm 3 (only required for `mock_exam_01`)

### Install

```bash
git clone https://github.com/CuriousLearner/cka-lab-runner.git
cd cka-lab-runner
make build
sudo mv bin/cka-lab-runner /usr/local/bin/
```

### Run Your First Lab

```bash
# Create config and cluster
cka-lab-runner init
cka-lab-runner up

# List available labs
cka-lab-runner lab list

# Run a lab
cka-lab-runner lab run pod_crashloop

# Fix the issue using kubectl...

# Verify your fix
cka-lab-runner lab verify pod_crashloop

# View solution if needed
cka-lab-runner lab solution pod_crashloop

# Clean up
cka-lab-runner down
```

<<<<<<< Updated upstream
## Available Labs (73)
=======
## Available Labs (69)
>>>>>>> Stashed changes

Lab IDs and titles describe the **symptom**, not the root cause — the same way a ticket
would reach you on the job. Don't read the category column if you want a cold diagnosis.

### Control Plane
- **etcd_wrong_ip** (Medium, 25min) - Fix API server → etcd communication
- **scheduler_not_running** (Medium, 20min) - Debug broken kube-scheduler
- **scheduler_unavailable** (Easy, 15min) - Debug unavailable kube-scheduler
- **scheduler_unhealthy** (Easy, 15min) - Debug unhealthy kube-scheduler
- **scheduler_degraded** (Hard, 25min) - Debug degraded kube-scheduler
- **scheduler_failing** (Hard, 25min) - Debug failing kube-scheduler
- **scheduler_offline** (Hard, 30min) - Debug offline kube-scheduler
- **cluster_upgrade** (Hard, 30min) - Cluster upgrade simulation
- **etcd_backup_restore** (Hard, 30min) - etcd backup and restore
- **kubelet_stopped** (Medium, 20min) - Fix stopped kubelet service
- **maintenance_window_prep** (Medium, 20min) - Hand a node over for a maintenance window
- **fleet_records_mismatch** (Medium, 20min) - Correct an inventory record against the live cluster
- **recovery_point_missing** (Hard, 30min) - Produce a verifiable point-in-time copy of cluster state
- **config_history_recovery** (Hard, 30min) - Rebuild state from a prior copy after a bad delete

### Scheduling (Pending pods — exam style)
- **app_pods_pending** (Medium, 15min) - Deployment pods stuck Pending (investigate)
- **web_pods_pending** (Easy, 15min) - Frontend pods stuck Pending (investigate)
- **api_pods_pending** (Medium, 20min) - API pods stuck Pending (investigate)
- **batch_pods_pending** (Medium, 15min) - Batch pods stuck Pending (investigate)
- **job_pod_pending** (Easy, 10min) - Single pod stuck Pending (investigate)
- **tenant_pods_repelled** (Medium, 15min) - Billing pods never land on a node
- **analytics_pods_unscheduled** (Hard, 20min) - Placement config present but still Pending

### Networking
- **network_policy_blocking** (Medium, 20min) - Fix NetworkPolicy blocking traffic
- **netpol_dns_blocked** (Hard, 25min) - Fix DNS blocked by NetworkPolicy
- **netpol_namespace_isolation** (Hard, 25min) - Fix cross-namespace NetworkPolicy
- **ingress_broken** (Medium, 20min) - Fix Ingress configuration
- **service_unreachable** (Medium, 15min) - Fix Service with no backends
- **service_wrong_port** (Medium, 15min) - Fix Service port mismatch
- **payments_api_unreachable** (Medium, 20min) - Storefront checkout calls time out
- **outbound_sync_failing** (Hard, 25min) - Worker cannot resolve or reach its vendor
- **metrics_endpoint_timeout** (Medium, 20min) - Scraper times out on one target only
- **partner_isolation_required** (Hard, 25min) - Restrict an API to one partner namespace (authoring)
- **dispatch_calls_refused** (Medium, 20min) - Calls refused although pods are Running
- **catalog_lookup_refused** (Medium, 15min) - Lookups fail right after a rename
- **retail_rollout_unreachable** (Hard, 30min) - Only newly created Services are unreachable
- **orders_no_backends** (Medium, 7min) - Orders Service has no backends [Weight: 6%]

> The NetworkPolicy labs need a CNI that actually enforces policies. If a policy lab is
> already "solved" the moment it starts, your cluster is ignoring NetworkPolicies — create
> the cluster with a policy-enforcing CNI (for example Calico on kind) before drilling them.

### DNS
- **coredns_broken_config** (Easy, 15min) - Fix CoreDNS configuration
- **service_discovery_broken** (Medium, 20min) - Service DNS names fail to resolve
- **external_lookups_failing** (Medium, 20min) - External DNS lookups fail from pods
- **lookups_timing_out** (Easy, 15min) - DNS lookups hang or time out
- **dns_unreachable** (Hard, 20min) - Cluster DNS unreachable despite healthy pods
- **app_cannot_resolve_services** (Medium, 20min) - One workload cannot resolve Services while the rest of the cluster is fine
- **edge_agent_resolution_failing** (Medium, 20min) - A host-network pod resolves nothing from the cluster
- **dns_pods_restarting** (Hard, 25min) - DNS pods crash-loop although the Corefile is valid
- **dns_pods_never_ready** (Hard, 30min) - DNS pods stay Running but never Ready, so kube-dns has no endpoints
- **legacy_hostname_unresolvable** (Medium, 25min) - Add a static record to cluster DNS without regressing anything else

### Storage
- **pvc_pending** (Medium, 20min) - Debug PVC stuck in Pending
- **storage_claim_pending** (Medium, 20min) - Fix StorageClass mismatch on claim
- **storage_pod_pending** (Medium, 15min) - Fix pod waiting on volume claim
- **database_unavailable** (Medium, 20min) - Database pod stuck Pending
- **cache_service_down** (Medium, 20min) - Cache deployment pods stuck Pending
- **webroot_missing** (Easy, 15min) - Web pod fails to become Ready
- **replica_pods_stuck** (Hard, 25min) - StatefulSet ordered pods stuck Pending

### RBAC
- **rbac_permission_denied** (Medium, 20min) - Fix missing Role permissions
- **rbac_sa_denied** (Medium, 20min) - Fix ServiceAccount Deployment permissions
- **rbac_binding_broken** (Hard, 20min) - Fix broken RoleBinding roleRef
- **inventory_sync_unauthorized** (Medium, 20min) - Workload rejected by the API
- **agent_identity_lost** (Medium, 15min) - Pod cannot find its credentials
- **fleet_report_forbidden** (Hard, 25min) - Report only sees one namespace (least privilege)
- **apps_developer_access** (Medium, 8min) - Onboard developer API access [Weight: 5%]

### Security
- **cert_expiration** (Hard, 25min) - Check certificate expiration

### Mock Exams
- **mock_exam_01** (Hard, 120min) - 12-task weighted exam covering workloads, node runtime, CRDs, Services, storage, autoscaling, Gateway API, and Helm
- **mock_exam_02** (Hard, 120min) - 12-task weighted exam covering workloads, CRDs, HPA/VPA, and Gateway API (no Helm)
- **mock_exam_03** (Medium, 90min) - 10-task easy/medium practice exam covering Pods, Deployments, Services, storage, RBAC, taints/tolerations, and Jobs

Exams require the default Docker-based kind cluster. Helm 3 is required only for `mock_exam_01`.

```bash
./cka-lab-runner lab run mock_exam_03
./cka-lab-runner lab verify mock_exam_03
```

Verification awards partial credit per requirement and saves the best score. Run it
as often as needed; the timer stops only at 100/100. Node-local tasks are completed
inside the kind node:

```bash
docker exec -it cka-lab-control-plane bash
```

### Workloads
- **pod_crashloop** (Easy, 15min) - Debug CrashLoopBackOff
- **image_pull_backoff** (Easy, 10min) - Fix image name typo
- **statefulset_broken** (Medium, 25min) - Fix StatefulSet configuration
- **daemonset_not_scheduled** (Medium, 20min) - Fix DaemonSet scheduling
- **deployments_not_progressing** (Hard, 30min) - New Deployments never create pods
- **agent_rollout_incomplete** (Medium, 20min) - Agent covers zero nodes
- **collector_skipping_nodes** (Hard, 25min) - Collector deployed but collecting nothing
- **retention_object_rejected** (Medium, 20min) - Team manifest rejected as unknown kind
- **backup_policy_incomplete** (Medium, 20min) - Stored object no longer passes validation
- **ops_health_snapshot** (Hard, 10min) - Ops health snapshot incomplete [Weight: 8%]

### Helm (CKA 2025+)
- **webshop_release_broken** (Medium, 10min) - Bad upgrade; restore with Helm rollback [Weight: 7%]
- **portal_chart_missing** (Medium, 10min) - Install a provided chart with exact values [Weight: 6%]
- **billing_values_drift** (Hard, 12min) - Fix drifted Helm values via upgrade + values file [Weight: 8%]

> Helm labs require the `helm` CLI on your workstation. Charts are embedded and synced to
> `/opt/CKA/charts/cka-webapp` on the kind node and mirrored at `/tmp/opt/CKA/charts/cka-webapp`
> for local `helm install`/`upgrade` commands.

## Drill Sets

Repetition beats reading for the procedural parts of the exam. Run each set start to finish,
then run it again from a fresh cluster until the commands come without thinking.

Work through a set one lab at a time — `lab run` wipes the previous lab's resources, so only
start the next lab after verifying the current one.

**Cluster lifecycle and state recovery** (the set worth repeating five times):
`maintenance_window_prep`, `fleet_records_mismatch`, `recovery_point_missing`,
`config_history_recovery`, `deployments_not_progressing`, `etcd_backup_restore`,
`cluster_upgrade`

**Node assignment — taints, tolerations, DaemonSets:**
`tenant_pods_repelled`, `analytics_pods_unscheduled`, `agent_rollout_incomplete`,
`collector_skipping_nodes`, `daemonset_not_scheduled`

**Traffic — policies first, then general connectivity:**
`payments_api_unreachable`, `metrics_endpoint_timeout`, `outbound_sync_failing`,
`partner_isolation_required`, `dispatch_calls_refused`, `catalog_lookup_refused`,
`retail_rollout_unreachable`

**Pod identity and permissions:**
`inventory_sync_unauthorized`, `agent_identity_lost`, `fleet_report_forbidden`

**API extensions:** `retention_object_rejected`, `backup_policy_incomplete`

**Helm (install / upgrade / rollback):**
`portal_chart_missing`, `webshop_release_broken`, `billing_values_drift`

Time yourself: `lab run` starts a timer that `lab verify` stops, and `lab list` shows your
solve time per lab so you can see the repetitions getting faster.

## Commands

```bash
# Setup
cka-lab-runner init                    # Create config file
cka-lab-runner up                      # Create cluster
cka-lab-runner up --recreate           # Recreate existing cluster
cka-lab-runner down                    # Delete cluster

# Labs
cka-lab-runner lab list                           # List all labs
cka-lab-runner lab list --category networking     # Filter by category
cka-lab-runner lab list --difficulty easy         # Filter by difficulty
cka-lab-runner lab random                         # Random lab
cka-lab-runner lab random --category storage      # Random lab in category
cka-lab-runner lab run <lab-id>                   # Run a lab
cka-lab-runner lab verify <lab-id>                # Verify your fix (marks complete + stops timer)
cka-lab-runner lab solution <lab-id>              # Show solution
cka-lab-runner lab reset-progress                 # Clear all completion progress
cka-lab-runner lab reset-progress --lab <lab-id>  # Clear one lab's progress
```

Progress is stored in `cka-lab-progress.yaml` (gitignored). Successful `lab verify` marks a lab done and records your solve time; `lab list` shows ✓ / ✗ and a **Time** column (`*` means the timer is still running).

## Adding Your Own Labs

Labs are Go types that implement the `Lab` interface. Create a file in `internal/labs/`:

```go
package labs

import "context"

func init() {
    Register(&MyLab{})
}

type MyLab struct {
    BaseLab
}

func (l *MyLab) ID() string { return "my_lab" }
func (l *MyLab) Title() string { return "My Lab Title" }
func (l *MyLab) Category() Category { return CategoryWorkloads }
func (l *MyLab) Difficulty() Difficulty { return DifficultyMedium }
func (l *MyLab) EstimatedTime() int { return 20 }
func (l *MyLab) Tags() []string { return []string{"pods", "troubleshooting"} }

func (l *MyLab) Description() string {
    return "Problem description"
}

func (l *MyLab) Hints() []string {
    return []string{
        "Check the pod status",
        "Look at the pod logs",
        "Check the pod configuration",
        "Fix the image tag",
    }
}

func (l *MyLab) Break(ctx context.Context, kubeconfigPath string) error {
    manifest := `apiVersion: v1
kind: Pod
metadata:
  name: broken-pod
spec:
  containers:
  - name: nginx
    image: nginx:broken`
    return kubectlApply(ctx, kubeconfigPath, manifest)
}

func (l *MyLab) Verify(ctx context.Context, kubeconfigPath string) error {
    output, err := kubectl(ctx, kubeconfigPath, "get", "pod", "broken-pod",
        "-o", "jsonpath={.status.phase}")
    if err != nil || output != "Running" {
        return fmt.Errorf("pod not running")
    }
    return nil
}

func (l *MyLab) SolutionSteps() []SolutionStep {
    return []SolutionStep{
        {
            Description: "Check pod status",
            Command:     "kubectl get pods",
            Notes:       "Pod should be in ImagePullBackOff",
        },
        {
            Description: "Fix the image",
            Command:     "kubectl edit pod broken-pod",
            Notes:       "Change image to nginx:alpine",
        },
    }
}
```

Rebuild with `make build` and your lab is ready.

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed examples.

## Configuration

Default config (`cka-lab-runner.yaml`):

```yaml
cluster:
  provider: kind      # or k3d, minikube (auto-detected if not specified)
  name: cka-lab
  k8sVersion: v1.30.0

labs:
  defaultNamespace: lab
```

## Development

```bash
make build         # Build binary
make test          # Run tests
make install       # Install to /usr/local/bin
make clean         # Clean build artifacts
```

## Contributing

Contributions welcome! Add new labs, fix bugs, or improve docs. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT License - see [LICENSE](LICENSE)
