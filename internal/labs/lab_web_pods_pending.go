package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&WebPodsPendingLab{})
}

type WebPodsPendingLab struct {
	BaseLab
}

func (l *WebPodsPendingLab) ID() string {
	return "web_pods_pending"
}

func (l *WebPodsPendingLab) Title() string {
	return "Web Pods Pending"
}

func (l *WebPodsPendingLab) Category() Category {
	return CategoryScheduling
}

func (l *WebPodsPendingLab) Difficulty() Difficulty {
	return DifficultyEasy
}

func (l *WebPodsPendingLab) Description() string {
	return `A Deployment named 'frontend' in namespace 'shopfront' cannot schedule its pods.
All pods remain Pending even though the cluster looks healthy.

Your task: Get the frontend pods to Running.`
}

func (l *WebPodsPendingLab) Hints() []string {
	return []string{
		"Describe a Pending pod and read Events",
		"Look at the pod spec for scheduling constraints",
		"Compare required node labels with actual node labels",
		"Either label the node or relax the workload constraint",
	}
}

func (l *WebPodsPendingLab) EstimatedTime() int {
	return 15
}

func (l *WebPodsPendingLab) Tags() []string {
	return []string{"scheduling", "pending", "nodeSelector", "troubleshooting"}
}

func (l *WebPodsPendingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *WebPodsPendingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: shopfront
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: shopfront
spec:
  replicas: 2
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      nodeSelector:
        disktype: ssd
      containers:
      - name: frontend
        image: nginx:alpine
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating frontend deployment: %w", err)
	}
	return nil
}

func (l *WebPodsPendingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "shopfront",
		"-l", "app=frontend", "-o", "jsonpath={.items[*].status.phase}")
	if strings.Contains(phase, "Pending") {
		return nil
	}
	return fmt.Errorf("expected frontend pods Pending, got %q", phase)
}

func (l *WebPodsPendingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "shopfront",
		"-l", "app=frontend", "-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pods: %w", err)
	}
	phases := strings.Fields(output)
	if len(phases) == 0 {
		return fmt.Errorf("no frontend pods found")
	}
	for _, p := range phases {
		if p != "Running" {
			return fmt.Errorf("frontend pods not all Running yet (got: %s)", output)
		}
	}
	return nil
}

func (l *WebPodsPendingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm pods are Pending",
			Command:     "kubectl get pods -n shopfront",
		},
		{
			Description: "Describe a Pending pod",
			Command:     "kubectl describe pod -n shopfront -l app=frontend | tail -20",
			Notes:       "Events mention nodeSelector disktype=ssd",
		},
		{
			Description: "Check node labels",
			Command:     "kubectl get nodes --show-labels",
		},
		{
			Description: "Fix option A: label a node",
			Command:     "kubectl label node <node-name> disktype=ssd",
		},
		{
			Description: "Fix option B: remove the nodeSelector",
			Command:     "kubectl patch deployment frontend -n shopfront --type=json -p='[{\"op\":\"remove\",\"path\":\"/spec/template/spec/nodeSelector\"}]'",
		},
		{
			Description: "Verify pods are Running",
			Command:     "kubectl get pods -n shopfront",
		},
	}
}
