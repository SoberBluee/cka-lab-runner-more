package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&AnalyticsPodsUnscheduledLab{})
}

type AnalyticsPodsUnscheduledLab struct {
	BaseLab
}

func (l *AnalyticsPodsUnscheduledLab) ID() string {
	return "analytics_pods_unscheduled"
}

func (l *AnalyticsPodsUnscheduledLab) Title() string {
	return "Analytics Workload Stays Pending Despite Config"
}

func (l *AnalyticsPodsUnscheduledLab) Category() Category {
	return CategoryScheduling
}

func (l *AnalyticsPodsUnscheduledLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *AnalyticsPodsUnscheduledLab) Description() string {
	return `The team that owns the 'analytics' Deployment in namespace 'insights' insists they
already configured it for the reserved node pool, and the manifest does contain the extra
placement config. The pods are still Pending.

The node pool reservation is intentional and must remain untouched.

Your task: work out why the existing placement config does not satisfy the reservation,
correct it, and get all 2 replicas Running.`
}

func (l *AnalyticsPodsUnscheduledLab) Hints() []string {
	return []string{
		"Compare the node's taint with the pod template line by line: key, value and effect all have to line up",
		"kubectl get node <node> -o jsonpath='{.spec.taints}' prints the exact triple",
		"kubectl get deploy analytics -n insights -o jsonpath='{.spec.template.spec.tolerations}' prints what the pod claims to accept",
		"A toleration for one effect does nothing about a taint with a different effect",
		"operator: Equal also requires the value to match; operator: Exists ignores the value",
	}
}

func (l *AnalyticsPodsUnscheduledLab) EstimatedTime() int {
	return 20
}

func (l *AnalyticsPodsUnscheduledLab) Tags() []string {
	return []string{"scheduling", "pending-pods", "nodes", "troubleshooting"}
}

func (l *AnalyticsPodsUnscheduledLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *AnalyticsPodsUnscheduledLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if _, err := kubectl(ctx, kubeconfigPath, "taint", "nodes", node,
			"tier=gold:NoSchedule", "--overwrite"); err != nil {
			return fmt.Errorf("reserving node %s: %w", node, err)
		}
	}

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: insights
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: analytics
  namespace: insights
spec:
  replicas: 2
  selector:
    matchLabels:
      app: analytics
  template:
    metadata:
      labels:
        app: analytics
    spec:
      tolerations:
      - key: tier
        operator: Equal
        value: silver
        effect: NoExecute
      containers:
      - name: analytics
        image: nginx:alpine
        ports:
        - containerPort: 80
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying analytics scenario: %w", err)
	}
	return nil
}

func (l *AnalyticsPodsUnscheduledLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)

	ready, err := kubectl(ctx, kubeconfigPath, "get", "deployment", "analytics", "-n", "insights",
		"-o", "jsonpath={.status.readyReplicas}")
	if err != nil {
		return fmt.Errorf("analytics deployment not found: %w", err)
	}
	if strings.TrimSpace(ready) != "" && strings.TrimSpace(ready) != "0" {
		return fmt.Errorf("analytics already has ready replicas")
	}
	return nil
}

func (l *AnalyticsPodsUnscheduledLab) Verify(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if !nodeHasTaintKey(ctx, kubeconfigPath, node, "tier") {
			return fmt.Errorf("the tier reservation was removed from node %s — put it back and fix the pod template instead", node)
		}
	}

	return deploymentReady(ctx, kubeconfigPath, "insights", "analytics", 2, 90*time.Second)
}

func (l *AnalyticsPodsUnscheduledLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Read the scheduling failure",
			Command:     "kubectl describe pod -n insights -l app=analytics | grep -A5 Events",
		},
		{
			Description: "Print the node's taint",
			Command:     "kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{\"  \"}{.spec.taints}{\"\\n\"}{end}'",
			Notes:       "tier=gold with effect NoSchedule",
		},
		{
			Description: "Print what the pod template tolerates",
			Command:     "kubectl get deploy analytics -n insights -o jsonpath='{.spec.template.spec.tolerations}'",
			Notes:       "value silver and effect NoExecute — neither matches the taint",
		},
		{
			Description: "Correct the toleration",
			Command: `kubectl patch deployment analytics -n insights --type merge -p '{"spec":{"template":{"spec":{"tolerations":[
  {"key":"tier","operator":"Equal","value":"gold","effect":"NoSchedule"}]}}}}'`,
		},
		{
			Description: "Confirm placement",
			Command:     "kubectl get pods -n insights -o wide",
		},
	}
}
