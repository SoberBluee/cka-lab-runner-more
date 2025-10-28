# cka-lab-runner

Practice for the Certified Kubernetes Administrator (CKA) exam by debugging realistic broken scenarios in a local Kubernetes cluster.

## What is this?

A CLI tool that creates broken Kubernetes scenarios for you to fix—just like the CKA exam. It sets up a local cluster, breaks something specific, and lets you practice troubleshooting. When you're done, it verifies you fixed it correctly.

## Quick Start

### Prerequisites

- Docker
- kubectl
- At least one of: kind, k3d, or minikube

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

## Available Labs (15)

### Control Plane
- **etcd_wrong_ip** (Medium, 25min) - Fix API server → etcd communication
- **scheduler_not_running** (Medium, 20min) - Debug broken kube-scheduler
- **cluster_upgrade** (Hard, 30min) - Cluster upgrade simulation
- **etcd_backup_restore** (Hard, 30min) - etcd backup and restore
- **kubelet_stopped** (Medium, 20min) - Fix stopped kubelet service

### Networking
- **network_policy_blocking** (Medium, 20min) - Fix NetworkPolicy blocking traffic
- **ingress_broken** (Medium, 20min) - Fix Ingress configuration

### DNS
- **coredns_broken_config** (Easy, 15min) - Fix CoreDNS configuration

### Storage
- **pvc_pending** (Medium, 20min) - Debug PVC stuck in Pending

### RBAC
- **rbac_permission_denied** (Medium, 20min) - Fix missing Role permissions

### Security
- **cert_expiration** (Hard, 25min) - Check certificate expiration

### Workloads
- **pod_crashloop** (Easy, 15min) - Debug CrashLoopBackOff
- **image_pull_backoff** (Easy, 10min) - Fix image name typo
- **statefulset_broken** (Medium, 25min) - Fix StatefulSet configuration
- **daemonset_not_scheduled** (Medium, 20min) - Fix DaemonSet scheduling

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
cka-lab-runner lab verify <lab-id>                # Verify your fix
cka-lab-runner lab solution <lab-id>              # Show solution
```

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
