package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&APIPodsPendingLab{})
}

type APIPodsPendingLab struct {
	BaseLab
}

func (l *APIPodsPendingLab) ID() string {
	return "api_pods_pending"
}

func (l *APIPodsPendingLab) Title() string {
	return "API Pods Pending"
}

func (l *APIPodsPendingLab) Category() Category {
	return CategoryScheduling
}

func (l *APIPodsPendingLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *APIPodsPendingLab) Description() string {
	return `The 'payments' Deployment in namespace 'billing' has pods stuck Pending.
Investigate and fix the issue so the pods run successfully.`
}

func (l *APIPodsPendingLab) Hints() []string {
	return []string{
		"Describe a Pending pod — Events usually name the constraint",
		"Inspect affinity / anti-affinity rules on the pod template",
		"requiredDuringSchedulingIgnoredDuringExecution must be satisfiable",
		"Fix by labeling nodes correctly or relaxing the affinity rule",
	}
}

func (l *APIPodsPendingLab) EstimatedTime() int {
	return 20
}

func (l *APIPodsPendingLab) Tags() []string {
	return []string{"scheduling", "pending", "affinity", "troubleshooting"}
}

func (l *APIPodsPendingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *APIPodsPendingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: billing
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments
  namespace: billing
spec:
  replicas: 1
  selector:
    matchLabels:
      app: payments
  template:
    metadata:
      labels:
        app: payments
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: topology.kubernetes.io/zone
                operator: In
                values:
                - eu-west-1a
      containers:
      - name: payments
        image: nginx:alpine
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating payments deployment: %w", err)
	}
	return nil
}

func (l *APIPodsPendingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "billing",
		"-l", "app=payments", "-o", "jsonpath={.items[*].status.phase}")
	if strings.Contains(phase, "Pending") {
		return nil
	}
	return fmt.Errorf("expected payments pods Pending, got %q", phase)
}

func (l *APIPodsPendingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "billing",
		"-l", "app=payments", "-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pods: %w", err)
	}
	phases := strings.Fields(output)
	if len(phases) == 0 {
		return fmt.Errorf("no payments pods found")
	}
	for _, p := range phases {
		if p != "Running" {
			return fmt.Errorf("payments pods not all Running yet (got: %s)", output)
		}
	}
	return nil
}

func (l *APIPodsPendingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm pods are Pending",
			Command:     "kubectl get pods -n billing",
		},
		{
			Description: "Describe a Pending pod",
			Command:     "kubectl describe pod -n billing -l app=payments | tail -30",
			Notes:       "Look for node affinity / didn't match messages",
		},
		{
			Description: "Inspect affinity in the Deployment",
			Command:     "kubectl get deploy payments -n billing -o yaml | grep -A20 affinity",
		},
		{
			Description: "Fix option A: label a node with the required zone",
			Command:     "kubectl label node <node-name> topology.kubernetes.io/zone=eu-west-1a --overwrite",
		},
		{
			Description: "Fix option B: remove the required affinity",
			Command:     "kubectl patch deployment payments -n billing --type=json -p='[{\"op\":\"remove\",\"path\":\"/spec/template/spec/affinity\"}]'",
		},
		{
			Description: "Verify pods are Running",
			Command:     "kubectl get pods -n billing",
		},
	}
}
