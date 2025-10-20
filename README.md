# cka-lab-runner

A **production-grade** practice lab runner for the Certified Kubernetes Administrator (CKA) exam.

## Overview

`cka-lab-runner` is a CLI tool that helps you practice for the CKA exam by creating realistic broken scenarios in a local Kubernetes cluster. It spins up a local cluster (using kind), applies exam-like misconfigurations, and lets you practice debugging and fixing them—with automatic verification of your solutions.

## Features

- **8 Production Labs**: Comprehensive scenarios covering all major CKA topics
- **Automatic Verification**: Check if you fixed the issue correctly with `lab verify`
- **Rich Metadata**: Estimated completion times (10-25 min) and searchable tags
- **Multiple Categories**: Control-plane, DNS, Networking, Storage, RBAC, Workloads, Scheduling
- **Difficulty Levels**: Easy, medium, and hard scenarios for progressive learning
- **Solution Guides**: Step-by-step canonical solutions with commands and notes
- **Local Clusters**: Uses kind (k3d and minikube support planned)
- **CI Integration**: GitHub Actions workflow validates all labs

## Requirements

- Go 1.22+ (for building from source)
- Docker
- kubectl
- kind

### Installing Prerequisites

**Docker**: Follow the [official Docker installation guide](https://docs.docker.com/get-docker/)

**kubectl**:
```bash
# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/

# macOS
brew install kubectl
```

**kind**:
```bash
# Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# macOS
brew install kind
```

## Installation

### From Source

```bash
git clone https://github.com/CuriousLearner/cka-lab-runner.git
cd cka-lab-runner
make build
# Or: go build -o cka-lab-runner ./cmd/cka-lab-runner
sudo mv bin/cka-lab-runner /usr/local/bin/
```

### Using Makefile

```bash
make install  # Builds and installs to /usr/local/bin
```

## Quick Start

### 1. Initialize Configuration

```bash
cka-lab-runner init
```

This creates a `cka-lab-runner.yaml` config file with default settings:

```yaml
cluster:
  provider: kind
  name: cka-lab
  k8sVersion: v1.30.0

labs:
  defaultNamespace: lab
```

### 2. Create the Cluster

```bash
cka-lab-runner up
```

This creates a local Kubernetes cluster using kind.

### 3. List Available Labs

```bash
cka-lab-runner lab list
```

Output:
```
ID                        Title                                    Category           Difficulty
───────────────────────────────────────────────────────────────────────────────────────────────
coredns_broken_config     CoreDNS Broken Configuration             dns                easy
etcd_wrong_ip             Etcd Wrong IP Address                    control-plane      medium
image_pull_backoff        ImagePullBackOff Error                   workloads          easy
network_policy_blocking   Network Policy Blocking Traffic          networking         medium
pod_crashloop             Pod in CrashLoopBackOff                  workloads          easy
pvc_pending               PersistentVolumeClaim Stuck in Pending   storage            medium
rbac_permission_denied    RBAC Permission Denied                   rbac               medium
scheduler_not_running     Kube-Scheduler Not Running               scheduling         medium
```

### 4. Run a Lab

```bash
cka-lab-runner lab run coredns_broken_config
```

This applies the broken scenario and displays:
```
╔═══════════════════════════════════════════════════════════════════╗
║ Lab: CoreDNS Broken Configuration                                  ║
╚═══════════════════════════════════════════════════════════════════╝

ID:              coredns_broken_config
Category:        dns
Difficulty:      easy
Estimated Time:  15 minutes
Tags:            dns, coredns, configmap, troubleshooting

Description:
DNS resolution is not working in the cluster...

Hints:
  1. Check the CoreDNS pods in the kube-system namespace
  2. Look at the CoreDNS ConfigMap
  ...
```

### 5. Debug the Issue

Use `kubectl` and other tools to diagnose and fix the problem, just like in the real exam.

```bash
kubectl get pods -n kube-system
kubectl logs -n kube-system -l k8s-app=kube-dns
kubectl edit configmap coredns -n kube-system
```

### 6. Verify Your Fix ✨

Check if you fixed the issue correctly:

```bash
cka-lab-runner lab verify coredns_broken_config
```

Output:
```
ℹ Verifying lab: CoreDNS Broken Configuration
✓ Congratulations! You successfully fixed: CoreDNS Broken Configuration
```

### 7. View the Solution (Optional)

If you need help:

```bash
cka-lab-runner lab solution coredns_broken_config
```

### 8. Clean Up

```bash
cka-lab-runner down
```

## Available Labs (8 Total)

### Control Plane
- **etcd_wrong_ip** (Medium, 25 min) - Fix incorrect etcd IP in API server configuration
  - Tags: `etcd`, `api-server`, `static-pods`, `control-plane`

### Scheduling
- **scheduler_not_running** (Medium, 20 min) - Debug and fix a broken kube-scheduler
  - Tags: `scheduler`, `static-pods`, `scheduling`, `troubleshooting`

### DNS
- **coredns_broken_config** (Easy, 15 min) - Fix invalid CoreDNS Corefile configuration
  - Tags: `dns`, `coredns`, `configmap`, `troubleshooting`

### Networking
- **network_policy_blocking** (Medium, 20 min) - Fix NetworkPolicy blocking legitimate traffic
  - Tags: `networking`, `network-policy`, `labels`, `selectors`

### Storage
- **pvc_pending** (Medium, 20 min) - Debug PVC stuck in Pending due to selector mismatch
  - Tags: `storage`, `pv`, `pvc`, `persistent-volume`, `troubleshooting`

### RBAC
- **rbac_permission_denied** (Medium, 20 min) - Fix Role missing required permissions
  - Tags: `rbac`, `roles`, `rolebindings`, `permissions`, `security`

### Workloads
- **pod_crashloop** (Easy, 15 min) - Debug and fix a deployment in CrashLoopBackOff
  - Tags: `pods`, `crashloop`, `configmap`, `troubleshooting`, `workloads`

- **image_pull_backoff** (Easy, 10 min) - Fix typo in container image name
  - Tags: `pods`, `images`, `troubleshooting`, `image-pull`, `deployments`

## Advanced Usage

### Filter Labs

List labs by category:
```bash
cka-lab-runner lab list --category control-plane
cka-lab-runner lab list --category storage
cka-lab-runner lab list --category rbac
```

List labs by difficulty:
```bash
cka-lab-runner lab list --difficulty easy
cka-lab-runner lab list --difficulty medium
```

### Random Lab Selection

Pick a random lab for practice:
```bash
cka-lab-runner lab random
```

With filters:
```bash
cka-lab-runner lab random --category networking --difficulty medium
```

With a fixed seed (for reproducibility in CI):
```bash
cka-lab-runner lab random --seed 42
```

### Verify Your Solution

After fixing a lab, verify it's correct:
```bash
cka-lab-runner lab verify <lab-id>
```

This automatically checks if you've correctly resolved the issue and provides feedback.

### Recreate Cluster

If you need to recreate the cluster:
```bash
cka-lab-runner up --recreate
```

### Custom Config Location

```bash
cka-lab-runner --config /path/to/config.yaml up
```

## Adding a New Lab

Labs are implemented as Go types that satisfy the `Lab` interface. Here's how to add a new one:

### 1. Create a New Lab File

Create a file in `internal/labs/`, for example `lab_my_scenario.go`:

```go
package labs

import (
	"context"
	"fmt"
	"time"
)

func init() {
	Register(&MyScenarioLab{})
}

type MyScenarioLab struct {
	BaseLab  // Provides default implementations
}

func (l *MyScenarioLab) ID() string {
	return "my_scenario"
}

func (l *MyScenarioLab) Title() string {
	return "My Scenario Title"
}

func (l *MyScenarioLab) Category() Category {
	return CategoryWorkloads
}

func (l *MyScenarioLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *MyScenarioLab) Description() string {
	return `Problem statement that the user will see.

Your task: Fix the issue.`
}

func (l *MyScenarioLab) Hints() []string {
	return []string{
		"General hint about where to look",
		"More specific hint",
		"Very specific hint",
		"Almost gives it away",
	}
}

func (l *MyScenarioLab) EstimatedTime() int {
	return 20  // minutes
}

func (l *MyScenarioLab) Tags() []string {
	return []string{"tag1", "tag2", "troubleshooting"}
}

func (l *MyScenarioLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	// Optional: Set up baseline state
	return nil
}

func (l *MyScenarioLab) Break(ctx context.Context, kubeconfigPath string) error {
	// Apply the broken scenario
	manifest := `apiVersion: v1
kind: Pod
metadata:
  name: broken-pod
spec:
  containers:
  - name: nginx
    image: nginx:broken-tag
`
	return kubectlApply(ctx, kubeconfigPath, manifest)
}

func (l *MyScenarioLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	// Optional: Verify the lab is in broken state
	return nil
}

func (l *MyScenarioLab) Verify(ctx context.Context, kubeconfigPath string) error {
	// Check if the user fixed it correctly
	output, err := kubectl(ctx, kubeconfigPath, "get", "pod", "broken-pod",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("pod not fixed: %w", err)
	}
	if output != "Running" {
		return fmt.Errorf("pod not running yet")
	}
	return nil
}

func (l *MyScenarioLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check pod status",
			Command:     "kubectl get pods",
			Notes:       "The pod should be in ImagePullBackOff",
		},
		{
			Description: "Fix the image tag",
			Command:     "kubectl edit pod broken-pod",
			Notes:       "Change the image to nginx:alpine",
		},
	}
}
```

### 2. Rebuild

```bash
make build
# Or: go build -o cka-lab-runner ./cmd/cka-lab-runner
```

Your new lab will automatically be registered and available via `cka-lab-runner lab list`.

For more details, see [CONTRIBUTING.md](CONTRIBUTING.md).

## Project Structure

```
cka-lab-runner/
├── cmd/
│   └── cka-lab-runner/     # Main CLI application
│       └── main.go
├── internal/
│   ├── cli/                # CLI helpers and printers
│   │   └── printer.go
│   ├── cluster/            # Cluster provider abstraction
│   │   ├── provider.go
│   │   └── kind.go
│   ├── config/             # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   └── labs/               # Lab definitions and registry
│       ├── lab.go
│       ├── registry.go
│       ├── registry_test.go
│       ├── util.go
│       ├── lab_etcd_wrong_ip.go
│       ├── lab_scheduler_not_running.go
│       ├── lab_coredns_broken_config.go
│       ├── lab_pod_crashloop.go
│       ├── lab_network_policy.go
│       ├── lab_rbac_permission_denied.go
│       ├── lab_pvc_pending.go
│       └── lab_image_pull_backoff.go
├── .github/
│   └── workflows/
│       └── ci.yaml
├── CONTRIBUTING.md
├── EXAMPLES.md
├── FEATURES.md
├── Makefile
├── demo.sh
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

## Development

### Using Makefile

```bash
make help          # Show all available commands
make build         # Build the binary
make test          # Run tests
make ci            # Run full CI checks (fmt, vet, test, build)
make install       # Install to /usr/local/bin
make clean         # Clean build artifacts
```

### Running Tests

```bash
go test ./...
# Or: make test
```

### Building

```bash
go build -o cka-lab-runner ./cmd/cka-lab-runner
# Or: make build
```

### Code Style

The project follows standard Go conventions:
- `gofmt` for formatting
- `go vet` for static analysis
- Idiomatic Go patterns

## CI/CD

The project includes a GitHub Actions workflow that:
1. Runs tests and linters
2. Builds the binary
3. Creates a kind cluster
4. Runs a random lab
5. Validates the solution can be rendered

This ensures all labs remain functional as the codebase evolves.

## Documentation

- **[README.md](README.md)** - This file, main user guide
- **[EXAMPLES.md](EXAMPLES.md)** - Detailed walkthroughs of all labs
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Guide for adding new labs
- **[FEATURES.md](FEATURES.md)** - Comprehensive feature list

## Roadmap

### v1.0 - Completed ✅
**Core Infrastructure:**
- ✅ CLI tool with Cobra framework
- ✅ Kind cluster provider (fully implemented)
- ✅ YAML-based configuration system
- ✅ Lab registry and management system
- ✅ Makefile and developer tooling
- ✅ GitHub Actions CI/CD pipeline

**Lab Coverage (8 Total):**
- ✅ Control-plane labs (etcd, scheduler)
- ✅ DNS labs (CoreDNS)
- ✅ Networking labs (NetworkPolicy)
- ✅ Storage labs (PersistentVolumes/PVC)
- ✅ RBAC labs (Role permissions)
- ✅ Workloads labs (CrashLoop, ImagePull)
- ✅ Difficulty progression (Easy → Medium → Hard)

**Advanced Features:**
- ✅ Automatic verification system (`lab verify` command)
- ✅ Rich metadata (estimated times, searchable tags)
- ✅ Category and difficulty filtering
- ✅ Reproducible random lab selection (with seeds)
- ✅ Progressive hints (4 per lab)
- ✅ Step-by-step solution guides
- ✅ BaseLab pattern for easy extension

**Documentation:**
- ✅ Comprehensive README (500+ lines)
- ✅ Detailed examples and walkthroughs
- ✅ Contributing guide with templates
- ✅ Feature documentation
- ✅ Unit tests with passing CI pipeline

### Future Enhancements
**v1.1 - Additional Cluster Providers:**
- [ ] k3d provider support
- [ ] minikube provider support
- [ ] Cluster provider auto-detection

**v1.2 - More Lab Scenarios:**
- [ ] Cluster upgrade simulation
- [ ] etcd backup and restore
- [ ] Node failure scenarios (kubelet stopped)
- [ ] Ingress configuration issues
- [ ] Certificate expiration problems
- [ ] StatefulSet and DaemonSet labs

**v2.0 - Exam Simulation:**
- [ ] Timer mode with exam countdown
- [ ] Full exam simulation (series of labs)
- [ ] Progress tracking across sessions
- [ ] Performance metrics and scoring
- [ ] Lab completion certificates

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

### Adding Labs

New lab scenarios are especially welcome. We need labs for:
- Cluster upgrades
- etcd backup and restore
- Node failures (kubelet stopped)
- Ingress issues
- Certificate problems
- And more!

See [CONTRIBUTING.md](CONTRIBUTING.md) for a complete guide with templates and examples.

## License

MIT License - see [LICENSE](LICENSE) file for details

## Acknowledgments

This project is designed to help people prepare for the CKA exam. It is not affiliated with the Linux Foundation or the Cloud Native Computing Foundation.

**Built with ❤️ for the Kubernetes community**
