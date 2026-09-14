package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&UpgradeWindowOpenLab{})
}

type UpgradeWindowOpenLab struct {
	BaseLab
}

func (l *UpgradeWindowOpenLab) ID() string             { return "upgrade_window_open" }
func (l *UpgradeWindowOpenLab) Title() string          { return "Upgrade Window Open" }
func (l *UpgradeWindowOpenLab) Category() Category     { return CategoryControlPlane }
func (l *UpgradeWindowOpenLab) Difficulty() Difficulty { return DifficultyHard }
func (l *UpgradeWindowOpenLab) EstimatedTime() int     { return 30 }
func (l *UpgradeWindowOpenLab) Hints() []string        { return nil }
func (l *UpgradeWindowOpenLab) Tags() []string {
	return []string{"mixed", "upgrade", "drain", "kubeadm", "scheduling"}
}

func (l *UpgradeWindowOpenLab) Description() string {
	return `Set the context before doing any work:

  kubectl config use-context cka-lab

[Weight: 15%] | Time limit: 20–30 minutes

Context:
An upgrade window is open. Workloads must keep running while nodes are upgraded
one at a time. Deployment gold-nginx is running in the default namespace.

Task:
1. Upgrade the cluster one node at a time, starting with the control-plane node.
2. Drain the worker node before upgrading it.
3. Minimize downtime: ensure gold-nginx can be rescheduled onto an alternate node
   before each node upgrade.
4. After the worker is drained for upgrade, gold-nginx Pods must run on the
   control-plane node.

Constraints:
- Work only in context cka-lab.
- Do not leave nodes cordoned when finished.
- Both nodes must be Ready when you are done.
- This lab requires a multi-node cluster (see configs/kind-multinode.yaml).
`
}

func (l *UpgradeWindowOpenLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if len(nodes) < 2 {
		return fmt.Errorf(`upgrade_window_open requires at least 2 nodes.

Create a multi-node kind cluster:
  kind delete cluster --name cka-lab
  kind create cluster --name cka-lab \
    --config configs/kind-multinode.yaml \
    --image kindest/node:v1.34.0`)
	}
	return nil
}

func (l *UpgradeWindowOpenLab) Break(ctx context.Context, kubeconfigPath string) error {
	cp, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	worker, err := firstWorkerNode(ctx, kubeconfigPath, cp)
	if err != nil {
		return err
	}

	_, _ = kubectl(ctx, kubeconfigPath, "delete", "deploy", "gold-nginx",
		"--ignore-not-found=true", "--wait=false")

	manifest := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: gold-nginx
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gold-nginx
  template:
    metadata:
      labels:
        app: gold-nginx
    spec:
      nodeSelector:
        kubernetes.io/hostname: %s
      containers:
      - name: nginx
        image: nginx:alpine
`, worker)
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating gold-nginx: %w", err)
	}

	return waitFor(ctx, 90*time.Second, func() error {
		node, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-l", "app=gold-nginx",
			"-o", "jsonpath={.items[0].spec.nodeName}")
		if err != nil {
			return err
		}
		if strings.TrimSpace(node) != worker {
			return fmt.Errorf("gold-nginx not yet on worker %s (on %q)", worker, strings.TrimSpace(node))
		}
		return nil
	})
}

func (l *UpgradeWindowOpenLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	cp, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	node, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-l", "app=gold-nginx",
		"-o", "jsonpath={.items[0].spec.nodeName}")
	if err != nil {
		return fmt.Errorf("gold-nginx not found: %w", err)
	}
	if strings.TrimSpace(node) == cp {
		return fmt.Errorf("gold-nginx already on control-plane")
	}
	return nil
}

func (l *UpgradeWindowOpenLab) Verify(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if len(nodes) < 2 {
		return fmt.Errorf("expected at least 2 nodes")
	}

	for _, n := range nodes {
		unsched, err := kubectl(ctx, kubeconfigPath, "get", "node", n,
			"-o", "jsonpath={.spec.unschedulable}")
		if err != nil {
			return err
		}
		if strings.TrimSpace(unsched) == "true" {
			return fmt.Errorf("node %s is still cordoned — uncordon when finished", n)
		}
		ready := false
		conds, _ := kubectl(ctx, kubeconfigPath, "get", "node", n,
			"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`)
		if strings.TrimSpace(conds) == "True" {
			ready = true
		}
		if !ready {
			return fmt.Errorf("node %s is not Ready", n)
		}
	}

	cp, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	// Candidate must clear the worker pin so the pod can land on the control-plane.
	selector, _ := kubectl(ctx, kubeconfigPath, "get", "deploy", "gold-nginx",
		"-o", "jsonpath={.spec.template.spec.nodeSelector.kubernetes\\.io/hostname}")
	if strings.TrimSpace(selector) != "" && strings.TrimSpace(selector) != cp {
		return fmt.Errorf("gold-nginx is still pinned to %q — allow it to run on the control-plane", strings.TrimSpace(selector))
	}

	podNode, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-l", "app=gold-nginx",
		"--field-selector=status.phase=Running",
		"-o", "jsonpath={.items[0].spec.nodeName}")
	if err != nil || strings.TrimSpace(podNode) == "" {
		return fmt.Errorf("no Running gold-nginx Pod found")
	}
	if strings.TrimSpace(podNode) != cp {
		return fmt.Errorf("gold-nginx must run on control-plane %s (currently on %s)", cp, strings.TrimSpace(podNode))
	}
	return nil
}

func (l *UpgradeWindowOpenLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Provision multi-node kind if needed",
			Command: `kind delete cluster --name cka-lab
kind create cluster --name cka-lab \
  --config configs/kind-multinode.yaml \
  --image kindest/node:v1.34.0
kubectl get nodes`,
			Notes: "Lab prep fails on a single-node cluster.",
		},
		{
			Description: "Identify nodes and current gold-nginx placement",
			Command:     `kubectl get nodes -o wide; kubectl get pods -l app=gold-nginx -o wide`,
		},
		{
			Description: "Allow scheduling on the control-plane (exam often needs this)",
			Command: `CP=$(kubectl get nodes -o jsonpath='{.items[?(@.metadata.labels.node-role\.kubernetes\.io/control-plane)].metadata.name}')
kubectl taint nodes "$CP" node-role.kubernetes.io/control-plane:NoSchedule- || true`,
			Notes: "Docs search: Taints and Tolerations; kubeadm upgrade",
		},
		{
			Description: "Remove the worker nodeSelector pin and drain the worker",
			Command: `kubectl patch deploy gold-nginx --type json -p='[{"op":"remove","path":"/spec/template/spec/nodeSelector"}]'
WORKER=$(kubectl get nodes -o jsonpath='{.items[?(@.metadata.labels.node-role\.kubernetes\.io/control-plane!="")].metadata.name}' 2>/dev/null)
# Prefer: pick the non-control-plane node
WORKER=$(kubectl get nodes --no-headers | awk '!/control-plane/{print $1; exit}')
kubectl drain "$WORKER" --ignore-daemonsets --delete-emptydir-data`,
			Notes: "On kind, enter the worker with: docker exec -it cka-lab-worker bash",
		},
		{
			Description: "Confirm gold-nginx is on the control-plane, then uncordon",
			Command: `kubectl get pods -l app=gold-nginx -o wide
kubectl uncordon "$WORKER"
kubectl get nodes`,
			Notes: "Full apt/kubeadm upgrade steps are in the README Multi-node section for free practice.",
		},
	}
}

func firstWorkerNode(ctx context.Context, kubeconfigPath, controlPlane string) (string, error) {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return "", err
	}
	for _, n := range nodes {
		if n != controlPlane {
			return n, nil
		}
	}
	return "", fmt.Errorf("no worker node found")
}
