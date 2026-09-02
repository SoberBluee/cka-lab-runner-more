package labs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&MaintenanceWindowPrepLab{})
}

type MaintenanceWindowPrepLab struct {
	BaseLab
}

func (l *MaintenanceWindowPrepLab) ID() string {
	return "maintenance_window_prep"
}

func (l *MaintenanceWindowPrepLab) Title() string {
	return "Node Handover For Maintenance Window"
}

func (l *MaintenanceWindowPrepLab) Category() Category {
	return CategoryControlPlane
}

func (l *MaintenanceWindowPrepLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *MaintenanceWindowPrepLab) Description() string {
	return `A maintenance window starts in ten minutes. The datacenter team will pull the node
hosting the 'platform' namespace workload, and they need it handed over cleanly.

Your task: take the node out of the scheduling pool and make sure the 'legacy-worker'
workload is no longer running on it. Leave the Deployment's replica count untouched —
the workload must come back by itself once the node returns.

This is a single-node cluster, so cluster add-ons will be evacuated too. That is expected.
Verify while the node is still handed over, then return it to service afterwards.`
}

func (l *MaintenanceWindowPrepLab) Hints() []string {
	return []string{
		"kubectl get nodes shows a STATUS column that changes when a node leaves the scheduling pool",
		"Marking a node unschedulable stops new pods but does not move the ones already there",
		"One command does both: it marks the node and evicts the pods it can",
		"Pods owned by DaemonSets and pods with local storage need extra flags before the eviction will proceed",
	}
}

func (l *MaintenanceWindowPrepLab) EstimatedTime() int {
	return 20
}

func (l *MaintenanceWindowPrepLab) Tags() []string {
	return []string{"nodes", "lifecycle", "procedure", "control-plane"}
}

func (l *MaintenanceWindowPrepLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}

	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		_, _ = kubectl(ctx, kubeconfigPath, "uncordon", node)
	}
	return nil
}

func (l *MaintenanceWindowPrepLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: platform
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: legacy-worker
  namespace: platform
spec:
  replicas: 2
  selector:
    matchLabels:
      app: legacy-worker
  template:
    metadata:
      labels:
        app: legacy-worker
    spec:
      containers:
      - name: worker
        image: nginx:alpine
        ports:
        - containerPort: 80
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying maintenance scenario: %w", err)
	}

	return deploymentReady(ctx, kubeconfigPath, "platform", "legacy-worker", 2, 120*time.Second)
}

func (l *MaintenanceWindowPrepLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		output, err := kubectl(ctx, kubeconfigPath, "get", "node", node,
			"-o", "jsonpath={.spec.unschedulable}")
		if err == nil && strings.TrimSpace(output) == "true" {
			return fmt.Errorf("node %s is already unschedulable", node)
		}
	}
	return nil
}

func (l *MaintenanceWindowPrepLab) Verify(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	cordoned := 0
	for _, node := range nodes {
		output, err := kubectl(ctx, kubeconfigPath, "get", "node", node,
			"-o", "jsonpath={.spec.unschedulable}")
		if err != nil {
			return fmt.Errorf("reading node %s: %w", node, err)
		}
		if strings.TrimSpace(output) == "true" {
			cordoned++
		}
	}
	if cordoned == 0 {
		return fmt.Errorf("no node has been taken out of the scheduling pool")
	}

	replicas, err := kubectl(ctx, kubeconfigPath, "get", "deployment", "legacy-worker",
		"-n", "platform", "-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return fmt.Errorf("reading legacy-worker deployment: %w", err)
	}
	desired, _ := strconv.Atoi(strings.TrimSpace(replicas))
	if desired < 2 {
		return fmt.Errorf("legacy-worker replicas dropped to %d — scale it back to 2 and evict the pods instead", desired)
	}

	running, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "platform",
		"-l", "app=legacy-worker", "--field-selector=status.phase=Running",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return fmt.Errorf("listing legacy-worker pods: %w", err)
	}
	if len(strings.Fields(running)) > 0 {
		return fmt.Errorf("legacy-worker still has %d running pod(s) on the node", len(strings.Fields(running)))
	}

	return nil
}

func (l *MaintenanceWindowPrepLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm where the workload is running",
			Command:     "kubectl get pods -n platform -o wide",
			Notes:       "Note the node name hosting the legacy-worker pods",
		},
		{
			Description: "Mark the node unschedulable and evict its pods",
			Command:     "kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data",
			Notes:       "Without --ignore-daemonsets the drain refuses to start; --delete-emptydir-data is needed for add-on pods using local scratch space",
		},
		{
			Description: "Confirm the handover",
			Command:     "kubectl get nodes && kubectl get pods -n platform",
			Notes:       "Node shows SchedulingDisabled and the legacy-worker pods are Pending, not deleted",
		},
		{
			Description: "Verify the lab now, before returning the node to service",
			Command:     "cka-lab-runner lab verify maintenance_window_prep",
		},
		{
			Description: "Return the node to service after the maintenance window",
			Command:     "kubectl uncordon <node-name>",
			Notes:       "Pending pods schedule again on their own — this is the step people forget in the real upgrade workflow",
		},
	}
}
