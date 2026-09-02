package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&SchedulerUnhealthyLab{})
}

type SchedulerUnhealthyLab struct {
	BaseLab
}

func (l *SchedulerUnhealthyLab) ID() string {
	return "scheduler_unhealthy"
}

func (l *SchedulerUnhealthyLab) Title() string {
	return "Kube-Scheduler Unhealthy"
}

func (l *SchedulerUnhealthyLab) Category() Category {
	return CategoryScheduling
}

func (l *SchedulerUnhealthyLab) Difficulty() Difficulty {
	return DifficultyEasy
}

func (l *SchedulerUnhealthyLab) Description() string {
	return `The kube-scheduler is not running properly.
New pods are stuck in the Pending state and are not being scheduled to nodes.

Your task: Fix the kube-scheduler so it can schedule pods again.`
}

func (l *SchedulerUnhealthyLab) Hints() []string {
	return []string{
		"Check the kube-scheduler pod status in the kube-system namespace",
		"Describe the pod and inspect Events",
		"Examine the kube-scheduler static pod manifest in /etc/kubernetes/manifests",
		"The kubelet automatically restarts static pods when their manifests change",
	}
}

func (l *SchedulerUnhealthyLab) EstimatedTime() int {
	return 15
}

func (l *SchedulerUnhealthyLab) Tags() []string {
	return []string{"scheduler", "static-pods", "scheduling", "troubleshooting"}
}

func (l *SchedulerUnhealthyLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *SchedulerUnhealthyLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodeName, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	containerName := nodeName

	output, err := dockerExec(ctx, containerName, "cat", "/etc/kubernetes/manifests/kube-scheduler.yaml")
	if err != nil {
		return fmt.Errorf("reading kube-scheduler manifest: %w", err)
	}

	// Only the image line uses "kube-scheduler:<tag>"; the command entry is "- kube-scheduler"
	modifiedManifest := strings.Replace(output, "kube-scheduler:", "kube-scheduler-typo:", 1)
	if modifiedManifest == output {
		return fmt.Errorf("could not find kube-scheduler image reference to modify")
	}

	writeCmd := fmt.Sprintf("cat > /etc/kubernetes/manifests/kube-scheduler.yaml << 'EOF'\n%s\nEOF", modifiedManifest)
	_, err = dockerExec(ctx, containerName, "sh", "-c", writeCmd)
	if err != nil {
		return fmt.Errorf("writing modified manifest: %w", err)
	}

	return nil
}

func (l *SchedulerUnhealthyLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)

	testPod := `apiVersion: v1
kind: Pod
metadata:
  name: test-sched-unhealthy
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:alpine
`
	if err := kubectlApply(ctx, kubeconfigPath, testPod); err != nil {
		return nil
	}

	time.Sleep(5 * time.Second)
	output, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "test-sched-unhealthy", "-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(output) == "Pending" {
		return nil
	}

	return nil
}

func (l *SchedulerUnhealthyLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
		"-l", "component=kube-scheduler",
		"-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check scheduler pod: %w", err)
	}

	if !strings.Contains(output, "Running") {
		return fmt.Errorf("scheduler pod is not running yet")
	}

	testPod := `apiVersion: v1
kind: Pod
metadata:
  name: verify-sched-unhealthy
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:alpine
`
	if err := kubectlApply(ctx, kubeconfigPath, testPod); err != nil {
		return fmt.Errorf("failed to create test pod: %w", err)
	}

	time.Sleep(10 * time.Second)
	output, err = kubectl(ctx, kubeconfigPath, "get", "pod", "verify-sched-unhealthy",
		"-o", "jsonpath={.spec.nodeName}")
	if err != nil {
		kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-unhealthy", "--ignore-not-found=true")
		return fmt.Errorf("failed to check test pod: %w", err)
	}

	kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-unhealthy", "--ignore-not-found=true")

	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("test pod was not scheduled to a node")
	}

	return nil
}

func (l *SchedulerUnhealthyLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the scheduler pod status",
			Command:     "kubectl get pods -n kube-system | grep scheduler",
			Notes:       "The kube-scheduler pod should be in ImagePullBackOff or ErrImagePull",
		},
		{
			Description: "Describe the scheduler pod",
			Command:     "kubectl describe pod -n kube-system -l component=kube-scheduler",
			Notes:       "Events should show a Failed to pull image error",
		},
		{
			Description: "Access the control plane node",
			Command:     "docker exec -it <cluster-name>-control-plane bash",
		},
		{
			Description: "Examine the image in the manifest",
			Command:     "grep image: /etc/kubernetes/manifests/kube-scheduler.yaml",
			Notes:       "Look for a typo in the kube-scheduler image name",
		},
		{
			Description: "Fix the image name",
			Command:     "sed -i 's/kube-scheduler-typo:/kube-scheduler:/' /etc/kubernetes/manifests/kube-scheduler.yaml",
			Notes:       "The kubelet will detect the change and restart the scheduler",
		},
		{
			Description: "Verify the scheduler is running",
			Command:     "kubectl get pods -n kube-system | grep scheduler",
			Notes:       "The scheduler pod should now be in Running state",
		},
		{
			Description: "Test pod scheduling",
			Command:     "kubectl run test-nginx --image=nginx:alpine",
			Notes:       "The pod should be scheduled and running",
		},
	}
}
