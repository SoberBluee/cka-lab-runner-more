package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&SchedulerOfflineLab{})
}

type SchedulerOfflineLab struct {
	BaseLab
}

func (l *SchedulerOfflineLab) ID() string {
	return "scheduler_offline"
}

func (l *SchedulerOfflineLab) Title() string {
	return "Kube-Scheduler Offline"
}

func (l *SchedulerOfflineLab) Category() Category {
	return CategoryScheduling
}

func (l *SchedulerOfflineLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *SchedulerOfflineLab) Description() string {
	return `The kube-scheduler is not running properly.
New pods are stuck in the Pending state and are not being scheduled to nodes.

Your task: Fix the kube-scheduler so it can schedule pods again.`
}

func (l *SchedulerOfflineLab) Hints() []string {
	return []string{
		"Check the kube-scheduler pod status and recent Events",
		"Inspect the pod logs for errors loading credentials or configuration files",
		"Review the volumes and volumeMounts in the kube-scheduler static pod manifest",
		"Confirm the files mounted into the scheduler container still exist and are valid on the host",
	}
}

func (l *SchedulerOfflineLab) EstimatedTime() int {
	return 30
}

func (l *SchedulerOfflineLab) Tags() []string {
	return []string{"scheduler", "static-pods", "scheduling", "troubleshooting"}
}

func (l *SchedulerOfflineLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *SchedulerOfflineLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodeName, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	containerName := nodeName

	// Move the real kubeconfig aside. The volume uses FileOrCreate, so kubelet
	// recreates an empty scheduler.conf and the scheduler fails to authenticate.
	_, err = dockerExec(ctx, containerName, "mv",
		"/etc/kubernetes/scheduler.conf",
		"/etc/kubernetes/scheduler.conf.bak")
	if err != nil {
		return fmt.Errorf("moving scheduler kubeconfig: %w", err)
	}

	// Touch the static pod manifest so kubelet restarts and FileOrCreate runs
	_, err = dockerExec(ctx, containerName, "sh", "-c",
		"touch /etc/kubernetes/manifests/kube-scheduler.yaml")
	if err != nil {
		return fmt.Errorf("touching scheduler manifest: %w", err)
	}

	return nil
}

func (l *SchedulerOfflineLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(15 * time.Second)

	testPod := `apiVersion: v1
kind: Pod
metadata:
  name: test-sched-offline
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
	output, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "test-sched-offline", "-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(output) == "Pending" {
		return nil
	}

	return nil
}

func (l *SchedulerOfflineLab) Verify(ctx context.Context, kubeconfigPath string) error {
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
  name: verify-sched-offline
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
	output, err = kubectl(ctx, kubeconfigPath, "get", "pod", "verify-sched-offline",
		"-o", "jsonpath={.spec.nodeName}")
	if err != nil {
		kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-offline", "--ignore-not-found=true")
		return fmt.Errorf("failed to check test pod: %w", err)
	}

	kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-offline", "--ignore-not-found=true")

	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("test pod was not scheduled to a node")
	}

	return nil
}

func (l *SchedulerOfflineLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the scheduler pod status",
			Command:     "kubectl get pods -n kube-system | grep scheduler",
			Notes:       "Expect CrashLoopBackOff or Error",
		},
		{
			Description: "Check the scheduler pod logs",
			Command:     "kubectl logs -n kube-system -l component=kube-scheduler",
			Notes:       "Look for kubeconfig / client config / unauthorized errors",
		},
		{
			Description: "Access the control plane node",
			Command:     "docker exec -it <cluster-name>-control-plane bash",
		},
		{
			Description: "Inspect the volume mount in the manifest",
			Command:     "grep -A5 'volumes:' /etc/kubernetes/manifests/kube-scheduler.yaml",
			Notes:       "Note the hostPath for scheduler.conf",
		},
		{
			Description: "Check the host kubeconfig file",
			Command:     "ls -la /etc/kubernetes/scheduler.conf*; wc -l /etc/kubernetes/scheduler.conf",
			Notes:       "An empty FileOrCreate placeholder may have replaced the real file; look for scheduler.conf.bak",
		},
		{
			Description: "Restore the real scheduler kubeconfig",
			Command:     "mv /etc/kubernetes/scheduler.conf.bak /etc/kubernetes/scheduler.conf",
			Notes:       "If an empty scheduler.conf exists, remove or overwrite it with the backup first",
		},
		{
			Description: "Force the static pod to reload if needed",
			Command:     "touch /etc/kubernetes/manifests/kube-scheduler.yaml",
		},
		{
			Description: "Verify the scheduler is running",
			Command:     "kubectl get pods -n kube-system | grep scheduler",
		},
		{
			Description: "Test pod scheduling",
			Command:     "kubectl run test-nginx --image=nginx:alpine",
		},
	}
}
