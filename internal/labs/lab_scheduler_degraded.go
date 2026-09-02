package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&SchedulerDegradedLab{})
}

type SchedulerDegradedLab struct {
	BaseLab
}

func (l *SchedulerDegradedLab) ID() string {
	return "scheduler_degraded"
}

func (l *SchedulerDegradedLab) Title() string {
	return "Kube-Scheduler Degraded"
}

func (l *SchedulerDegradedLab) Category() Category {
	return CategoryScheduling
}

func (l *SchedulerDegradedLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *SchedulerDegradedLab) Description() string {
	return `The kube-scheduler is not running properly.
New pods are stuck in the Pending state and are not being scheduled to nodes.

Your task: Fix the kube-scheduler so it can schedule pods again.`
}

func (l *SchedulerDegradedLab) Hints() []string {
	return []string{
		"Check the kube-scheduler pod status in the kube-system namespace",
		"Inspect the pod logs for startup or configuration errors",
		"Compare the kube-scheduler static pod flags with a known-good control plane component",
		"Pay close attention to file paths referenced by the scheduler arguments",
	}
}

func (l *SchedulerDegradedLab) EstimatedTime() int {
	return 25
}

func (l *SchedulerDegradedLab) Tags() []string {
	return []string{"scheduler", "static-pods", "scheduling", "troubleshooting"}
}

func (l *SchedulerDegradedLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *SchedulerDegradedLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodeName, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	containerName := nodeName

	output, err := dockerExec(ctx, containerName, "cat", "/etc/kubernetes/manifests/kube-scheduler.yaml")
	if err != nil {
		return fmt.Errorf("reading kube-scheduler manifest: %w", err)
	}

	// Typo only the --kubeconfig flag path; leave authz/authn flags alone so logs are subtler
	modifiedManifest := strings.Replace(output,
		"--kubeconfig=/etc/kubernetes/scheduler.conf",
		"--kubeconfig=/etc/kubernetes/scheduler.confx",
		1)
	if modifiedManifest == output {
		return fmt.Errorf("could not find --kubeconfig flag to modify")
	}

	writeCmd := fmt.Sprintf("cat > /etc/kubernetes/manifests/kube-scheduler.yaml << 'EOF'\n%s\nEOF", modifiedManifest)
	_, err = dockerExec(ctx, containerName, "sh", "-c", writeCmd)
	if err != nil {
		return fmt.Errorf("writing modified manifest: %w", err)
	}

	return nil
}

func (l *SchedulerDegradedLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)

	testPod := `apiVersion: v1
kind: Pod
metadata:
  name: test-sched-degraded
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
	output, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "test-sched-degraded", "-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(output) == "Pending" {
		return nil
	}

	return nil
}

func (l *SchedulerDegradedLab) Verify(ctx context.Context, kubeconfigPath string) error {
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
  name: verify-sched-degraded
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
	output, err = kubectl(ctx, kubeconfigPath, "get", "pod", "verify-sched-degraded",
		"-o", "jsonpath={.spec.nodeName}")
	if err != nil {
		kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-degraded", "--ignore-not-found=true")
		return fmt.Errorf("failed to check test pod: %w", err)
	}

	kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-degraded", "--ignore-not-found=true")

	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("test pod was not scheduled to a node")
	}

	return nil
}

func (l *SchedulerDegradedLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the scheduler pod status",
			Command:     "kubectl get pods -n kube-system | grep scheduler",
			Notes:       "The pod should be in CrashLoopBackOff or Error",
		},
		{
			Description: "Check the scheduler pod logs",
			Command:     "kubectl logs -n kube-system -l component=kube-scheduler",
			Notes:       "Look for errors about an inability to load the kubeconfig file",
		},
		{
			Description: "Access the control plane node",
			Command:     "docker exec -it <cluster-name>-control-plane bash",
		},
		{
			Description: "Inspect scheduler flags in the manifest",
			Command:     "grep kubeconfig /etc/kubernetes/manifests/kube-scheduler.yaml",
			Notes:       "Compare --kubeconfig with the authentication/authorization kubeconfig paths",
		},
		{
			Description: "Fix the typo in the --kubeconfig path",
			Command:     "sed -i 's|--kubeconfig=/etc/kubernetes/scheduler.confx|--kubeconfig=/etc/kubernetes/scheduler.conf|' /etc/kubernetes/manifests/kube-scheduler.yaml",
			Notes:       "The kubelet will restart the static pod after the change",
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
