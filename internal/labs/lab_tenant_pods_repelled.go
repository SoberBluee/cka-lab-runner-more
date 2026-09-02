package labs

import (
	"context"
	"fmt"
	"time"
)

func init() {
	Register(&TenantPodsRepelledLab{})
}

type TenantPodsRepelledLab struct {
	BaseLab
}

func (l *TenantPodsRepelledLab) ID() string {
	return "tenant_pods_repelled"
}

func (l *TenantPodsRepelledLab) Title() string {
	return "Billing Pods Will Not Land On Any Node"
}

func (l *TenantPodsRepelledLab) Category() Category {
	return CategoryScheduling
}

func (l *TenantPodsRepelledLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *TenantPodsRepelledLab) Description() string {
	return `The 'billing' Deployment in namespace 'finance' has been Pending since it was
created. Nodes are Ready and have plenty of capacity, but no pod ever gets placed.

Platform policy marks these nodes as reserved for approved tenants, and that reservation
must stay exactly as it is — you may not remove it or relabel the nodes around it.

Your task: get both billing replicas Running while leaving the node reservation in place.`
}

func (l *TenantPodsRepelledLab) Hints() []string {
	return []string{
		"kubectl describe pod -n finance <pod> — read the FailedScheduling message carefully, it names what rejected the pod",
		"kubectl describe node <node> | grep -i taint shows the reservation on the node",
		"A node reservation repels pods unless the pod itself declares that it accepts it",
		"That declaration goes in the pod template under spec.tolerations, matching key, value and effect",
	}
}

func (l *TenantPodsRepelledLab) EstimatedTime() int {
	return 15
}

func (l *TenantPodsRepelledLab) Tags() []string {
	return []string{"scheduling", "pending-pods", "nodes", "troubleshooting"}
}

func (l *TenantPodsRepelledLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *TenantPodsRepelledLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if _, err := kubectl(ctx, kubeconfigPath, "taint", "nodes", node,
			"tenancy=dedicated:NoSchedule", "--overwrite"); err != nil {
			return fmt.Errorf("reserving node %s: %w", node, err)
		}
	}

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: finance
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: billing
  namespace: finance
spec:
  replicas: 2
  selector:
    matchLabels:
      app: billing
  template:
    metadata:
      labels:
        app: billing
    spec:
      containers:
      - name: billing
        image: nginx:alpine
        ports:
        - containerPort: 80
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying billing scenario: %w", err)
	}
	return nil
}

func (l *TenantPodsRepelledLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)

	ready, err := kubectl(ctx, kubeconfigPath, "get", "deployment", "billing", "-n", "finance",
		"-o", "jsonpath={.status.readyReplicas}")
	if err != nil {
		return fmt.Errorf("billing deployment not found: %w", err)
	}
	if ready != "" && ready != "0" {
		return fmt.Errorf("billing already has ready replicas")
	}
	return nil
}

func (l *TenantPodsRepelledLab) Verify(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if !nodeHasTaintKey(ctx, kubeconfigPath, node, "tenancy") {
			return fmt.Errorf("the tenancy reservation was removed from node %s — restore it and make the pods accept it instead", node)
		}
	}

	return deploymentReady(ctx, kubeconfigPath, "finance", "billing", 2, 90*time.Second)
}

func (l *TenantPodsRepelledLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Read why the scheduler rejected the pods",
			Command:     "kubectl describe pod -n finance -l app=billing | grep -A5 Events",
			Notes:       "The message names the taint that no pod tolerates",
		},
		{
			Description: "Inspect the node reservation",
			Command:     "kubectl describe node <node-name> | grep -i -A2 taints",
			Notes:       "tenancy=dedicated:NoSchedule",
		},
		{
			Description: "Teach the workload to accept the reservation",
			Command: `kubectl patch deployment billing -n finance --type merge -p '{"spec":{"template":{"spec":{"tolerations":[
  {"key":"tenancy","operator":"Equal","value":"dedicated","effect":"NoSchedule"}]}}}}'`,
			Notes: "operator: Exists with just the key also works when you do not care about the value",
		},
		{
			Description: "Confirm the pods are placed",
			Command:     "kubectl get pods -n finance -o wide",
		},
		{
			Description: "Confirm the reservation is still on the node",
			Command:     "kubectl describe node <node-name> | grep -i -A2 taints",
			Notes:       "Removing the taint would also have fixed scheduling, but it violates the platform policy this lab enforces",
		},
	}
}
