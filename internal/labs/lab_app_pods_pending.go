package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&AppPodsPendingLab{})
}

type AppPodsPendingLab struct {
	BaseLab
}

func (l *AppPodsPendingLab) ID() string {
	return "app_pods_pending"
}

func (l *AppPodsPendingLab) Title() string {
	return "Application Pods Pending"
}

func (l *AppPodsPendingLab) Category() Category {
	return CategoryScheduling
}

func (l *AppPodsPendingLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *AppPodsPendingLab) Description() string {
	return `A Deployment named 'checkout' in namespace 'orders' has pods stuck in Pending.
The cluster nodes are Ready.

Your task: Make the checkout pods schedule and reach Running.`
}

func (l *AppPodsPendingLab) Hints() []string {
	return []string{
		"Describe a Pending pod and read the Events carefully",
		"Compare pod scheduling constraints with node state",
		"Check node taints and whether the pod tolerates them",
		"You can fix this either by adjusting the workload or the node",
	}
}

func (l *AppPodsPendingLab) EstimatedTime() int {
	return 15
}

func (l *AppPodsPendingLab) Tags() []string {
	return []string{"scheduling", "pending", "taints", "tolerations", "troubleshooting"}
}

func (l *AppPodsPendingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *AppPodsPendingLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodeName, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	_, err = kubectl(ctx, kubeconfigPath, "taint", "nodes", nodeName,
		"dedicated=critical:NoSchedule", "--overwrite")
	if err != nil {
		return fmt.Errorf("tainting node: %w", err)
	}

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: orders
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout
  namespace: orders
spec:
  replicas: 2
  selector:
    matchLabels:
      app: checkout
  template:
    metadata:
      labels:
        app: checkout
    spec:
      containers:
      - name: checkout
        image: nginx:alpine
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating checkout deployment: %w", err)
	}
	return nil
}

func (l *AppPodsPendingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "orders",
		"-l", "app=checkout", "-o", "jsonpath={.items[*].status.phase}")
	if strings.Contains(phase, "Pending") {
		return nil
	}
	return fmt.Errorf("expected checkout pods Pending, got %q", phase)
}

func (l *AppPodsPendingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "orders",
		"-l", "app=checkout", "-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pods: %w", err)
	}
	phases := strings.Fields(output)
	if len(phases) == 0 {
		return fmt.Errorf("no checkout pods found")
	}
	for _, p := range phases {
		if p != "Running" {
			return fmt.Errorf("checkout pods not all Running yet (got: %s)", output)
		}
	}
	return nil
}

func (l *AppPodsPendingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm pods are Pending",
			Command:     "kubectl get pods -n orders",
		},
		{
			Description: "Describe a Pending pod",
			Command:     "kubectl describe pod -n orders -l app=checkout | tail -30",
			Notes:       "Events should mention untolerated taint dedicated=critical:NoSchedule",
		},
		{
			Description: "Inspect node taints",
			Command:     "kubectl describe nodes | grep -A5 Taints",
		},
		{
			Description: "Fix option A: remove the taint",
			Command:     "kubectl taint nodes <node-name> dedicated=critical:NoSchedule-",
		},
		{
			Description: "Fix option B: add a matching toleration to the Deployment",
			Command:     "kubectl patch deployment checkout -n orders --type=json -p='[{\"op\":\"add\",\"path\":\"/spec/template/spec/tolerations\",\"value\":[{\"key\":\"dedicated\",\"operator\":\"Equal\",\"value\":\"critical\",\"effect\":\"NoSchedule\"}]}]'",
		},
		{
			Description: "Verify pods are Running",
			Command:     "kubectl get pods -n orders",
		},
	}
}
