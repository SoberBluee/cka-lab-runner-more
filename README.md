# cka-lab-runner

A reproducible practice lab runner for the Certified Kubernetes Administrator (CKA) exam.

## Overview

`cka-lab-runner` is a CLI tool that helps you practice for the CKA exam by creating realistic broken scenarios in a local Kubernetes cluster. It spins up a local cluster (using kind), applies exam-like misconfigurations, and lets you practice debugging and fixing them.

## Features

- **Reproducible labs**: Consistent broken scenarios for practice
- **Multiple categories**: Control plane, networking, DNS, scheduling, workloads
- **Difficulty levels**: Easy, medium, and hard scenarios
- **Solution guides**: Step-by-step canonical solutions
- **Local clusters**: Uses kind (k3d and minikube support planned)
- **CI integration**: GitHub Actions workflow to validate labs

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
go build -o cka-lab-runner ./cmd/cka-lab-runner
sudo mv cka-lab-runner /usr/local/bin/
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
ID                        Title                                    Category            Difficulty
───────────────────────────────────────────────────────────────────────────────────────────────────
coredns_broken_config     CoreDNS Broken Configuration             dns                 easy
etcd_wrong_ip             Etcd Wrong IP Address                    control-plane       medium
network_policy_blocking   Network Policy Blocking Traffic          networking          medium
pod_crashloop             Pod in CrashLoopBackOff                  workloads           easy
scheduler_not_running     Kube-Scheduler Not Running               scheduling          medium
```

### 4. Run a Lab

```bash
cka-lab-runner lab run coredns_broken_config
```

This applies the broken scenario to your cluster and displays the problem statement.

### 5. Debug the Issue

Use `kubectl` and other tools to diagnose and fix the problem, just like in the real exam.

```bash
kubectl get pods -n kube-system
kubectl logs -n kube-system -l k8s-app=kube-dns
kubectl edit configmap coredns -n kube-system
```

### 6. View the Solution

When you're ready to see the canonical solution:

```bash
cka-lab-runner lab solution coredns_broken_config
```

### 7. Clean Up

```bash
cka-lab-runner down
```

## Available Labs

### Control Plane
- **etcd_wrong_ip** (medium): Fix incorrect etcd IP in API server configuration
- **scheduler_not_running** (medium): Debug and fix a broken kube-scheduler

### DNS
- **coredns_broken_config** (easy): Fix invalid CoreDNS Corefile configuration

### Networking
- **network_policy_blocking** (medium): Fix NetworkPolicy blocking legitimate traffic

### Workloads
- **pod_crashloop** (easy): Debug and fix a deployment in CrashLoopBackOff

### Scheduling
- Part of scheduler_not_running lab

## Advanced Usage

### Filter Labs

List labs by category:
```bash
cka-lab-runner lab list --category control-plane
```

List labs by difficulty:
```bash
cka-lab-runner lab list --difficulty easy
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

type MyScenarioLab struct{}

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
		"Hint 1",
		"Hint 2",
	}
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

```go
go build -o cka-lab-runner ./cmd/cka-lab-runner
```

Your new lab will automatically be registered and available via `cka-lab-runner lab list`.

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
│       └── lab_network_policy.go
├── .github/
│   └── workflows/
│       └── ci.yaml
├── go.mod
├── go.sum
└── README.md
```

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o cka-lab-runner ./cmd/cka-lab-runner
```

### Code Style

The project follows standard Go conventions:
- `gofmt` for formatting
- `go vet` for static analysis
- Idiomatic Go patterns

## CI/CD

The project includes a GitHub Actions workflow that:
1. Builds the binary
2. Creates a kind cluster
3. Runs a random lab
4. Validates the solution can be rendered

This ensures all labs remain functional as the codebase evolves.

## Roadmap

### v1.1
- [ ] k3d provider support
- [ ] minikube provider support
- [ ] More labs (RBAC, persistent volumes, upgrades)

### v2.0
- [ ] Timer mode for exam simulation
- [ ] Progress tracking
- [ ] Lab validation (automatic checking if you fixed it correctly)

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

### Adding Labs

New lab scenarios are especially welcome. See the "Adding a New Lab" section above.

## License

MIT License - see LICENSE file for details

## Acknowledgments

This project is designed to help people prepare for the CKA exam. It is not affiliated with the Linux Foundation or the Cloud Native Computing Foundation.
