package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&JobPodPendingLab{})
}

type JobPodPendingLab struct {
	BaseLab
}

func (l *JobPodPendingLab) ID() string {
	return "job_pod_pending"
}

func (l *JobPodPendingLab) Title() string {
	return "Pod Stuck Pending"
}

func (l *JobPodPendingLab) Category() Category {
	return CategoryScheduling
}

func (l *JobPodPendingLab) Difficulty() Difficulty {
	return DifficultyEasy
}

func (l *JobPodPendingLab) Description() string {
	return `A Pod named 'migrate' in namespace 'ops' is stuck in Pending and never starts.
Fix it so the pod is scheduled and Running.`
}

func (l *JobPodPendingLab) Hints() []string {
	return []string{
		"Describe the pod and read Events",
		"Check whether the pod is pinned to a specific node",
		"spec.nodeName bypasses the scheduler — the named node must exist",
		"Remove or correct nodeName, then recreate the pod if needed",
	}
}

func (l *JobPodPendingLab) EstimatedTime() int {
	return 10
}

func (l *JobPodPendingLab) Tags() []string {
	return []string{"scheduling", "pending", "nodeName", "troubleshooting"}
}

func (l *JobPodPendingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *JobPodPendingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: ops
---
apiVersion: v1
kind: Pod
metadata:
  name: migrate
  namespace: ops
spec:
  nodeName: worker-node-99
  containers:
  - name: migrate
    image: busybox:1.28
    command: ["sh", "-c", "sleep 3600"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating migrate pod: %w", err)
	}
	return nil
}

func (l *JobPodPendingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "migrate", "-n", "ops",
		"-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(phase) == "Pending" {
		return nil
	}
	return fmt.Errorf("expected migrate pod Pending, got %q", phase)
}

func (l *JobPodPendingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pod", "migrate", "-n", "ops",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pod: %w", err)
	}
	if strings.TrimSpace(output) != "Running" {
		return fmt.Errorf("migrate pod is not Running yet (got: %s)", output)
	}

	nodeName, err := kubectl(ctx, kubeconfigPath, "get", "pod", "migrate", "-n", "ops",
		"-o", "jsonpath={.spec.nodeName}")
	if err != nil {
		return fmt.Errorf("failed to check nodeName: %w", err)
	}
	if strings.TrimSpace(nodeName) == "worker-node-99" {
		return fmt.Errorf("pod is still pinned to nonexistent node worker-node-99")
	}
	return nil
}

func (l *JobPodPendingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the pod is Pending",
			Command:     "kubectl get pod migrate -n ops",
		},
		{
			Description: "Describe the pod",
			Command:     "kubectl describe pod migrate -n ops",
			Notes:       "Look for nodeName / node not found style messages",
		},
		{
			Description: "Inspect nodeName",
			Command:     "kubectl get pod migrate -n ops -o yaml | grep nodeName",
			Notes:       "Pinned to worker-node-99 which does not exist",
		},
		{
			Description: "Recreate the pod without nodeName",
			Command:     "kubectl delete pod migrate -n ops && kubectl run migrate -n ops --image=busybox:1.28 --restart=Never --command -- sleep 3600",
			Notes:       "Or apply a fixed manifest without spec.nodeName",
		},
		{
			Description: "Verify the pod is Running",
			Command:     "kubectl get pod migrate -n ops",
		},
	}
}
