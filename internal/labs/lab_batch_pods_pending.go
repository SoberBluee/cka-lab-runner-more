package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&BatchPodsPendingLab{})
}

type BatchPodsPendingLab struct {
	BaseLab
}

func (l *BatchPodsPendingLab) ID() string {
	return "batch_pods_pending"
}

func (l *BatchPodsPendingLab) Title() string {
	return "Batch Pods Pending"
}

func (l *BatchPodsPendingLab) Category() Category {
	return CategoryScheduling
}

func (l *BatchPodsPendingLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *BatchPodsPendingLab) Description() string {
	return `A Deployment named 'reporter' in namespace 'analytics' has pods that never leave Pending.
Nodes report Ready.

Your task: Fix the problem so reporter pods become Running.`
}

func (l *BatchPodsPendingLab) Hints() []string {
	return []string{
		"Describe a Pending pod and focus on Events",
		"Insufficient cpu/memory is a common Pending reason",
		"Compare resource requests with node allocatable capacity",
		"Lower the requests (or free capacity) so the scheduler can place the pod",
	}
}

func (l *BatchPodsPendingLab) EstimatedTime() int {
	return 15
}

func (l *BatchPodsPendingLab) Tags() []string {
	return []string{"scheduling", "pending", "resources", "troubleshooting"}
}

func (l *BatchPodsPendingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *BatchPodsPendingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: analytics
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reporter
  namespace: analytics
spec:
  replicas: 1
  selector:
    matchLabels:
      app: reporter
  template:
    metadata:
      labels:
        app: reporter
    spec:
      containers:
      - name: reporter
        image: nginx:alpine
        resources:
          requests:
            cpu: "32"
            memory: 64Gi
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating reporter deployment: %w", err)
	}
	return nil
}

func (l *BatchPodsPendingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "analytics",
		"-l", "app=reporter", "-o", "jsonpath={.items[*].status.phase}")
	if strings.Contains(phase, "Pending") {
		return nil
	}
	return fmt.Errorf("expected reporter pods Pending, got %q", phase)
}

func (l *BatchPodsPendingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "analytics",
		"-l", "app=reporter", "-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pods: %w", err)
	}
	phases := strings.Fields(output)
	if len(phases) == 0 {
		return fmt.Errorf("no reporter pods found")
	}
	for _, p := range phases {
		if p != "Running" {
			return fmt.Errorf("reporter pods not all Running yet (got: %s)", output)
		}
	}
	return nil
}

func (l *BatchPodsPendingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm pods are Pending",
			Command:     "kubectl get pods -n analytics",
		},
		{
			Description: "Describe a Pending pod",
			Command:     "kubectl describe pod -n analytics -l app=reporter | tail -20",
			Notes:       "Events should mention Insufficient cpu and/or memory",
		},
		{
			Description: "Check node allocatable resources",
			Command:     "kubectl describe node | grep -A10 Allocatable",
		},
		{
			Description: "Reduce resource requests",
			Command:     "kubectl set resources deployment reporter -n analytics --requests=cpu=100m,memory=128Mi",
		},
		{
			Description: "Verify pods are Running",
			Command:     "kubectl get pods -n analytics",
		},
	}
}
