package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&SchedulerUnavailableLab{})
}

type SchedulerUnavailableLab struct {
	BaseLab
}

func (l *SchedulerUnavailableLab) ID() string {
	return "scheduler_unavailable"
}

func (l *SchedulerUnavailableLab) Title() string {
	return "Kube-Scheduler Unavailable"
}

func (l *SchedulerUnavailableLab) Category() Category {
	return CategoryScheduling
}

func (l *SchedulerUnavailableLab) Difficulty() Difficulty {
	return DifficultyEasy
}

func (l *SchedulerUnavailableLab) Description() string {
	return `The kube-scheduler is not working.
New pods are stuck in the Pending state and are not being scheduled to nodes.

Your task: Fix the kube-scheduler so it can schedule pods again.`
}

func (l *SchedulerUnavailableLab) Hints() []string {
	return []string{
		"Check the kube-scheduler pod status in the kube-system namespace",
		"Static pods are defined by manifests in /etc/kubernetes/manifests on the control plane",
		"List the files in that directory and compare them to other control plane components",
		"The kubelet only creates static pods for manifests present in that directory",
	}
}

func (l *SchedulerUnavailableLab) EstimatedTime() int {
	return 15
}

func (l *SchedulerUnavailableLab) Tags() []string {
	return []string{"scheduler", "static-pods", "scheduling", "troubleshooting"}
}

func (l *SchedulerUnavailableLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *SchedulerUnavailableLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodeName, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	containerName := nodeName

	_, err = dockerExec(ctx, containerName, "mv",
		"/etc/kubernetes/manifests/kube-scheduler.yaml",
		"/etc/kubernetes/kube-scheduler.yaml.bak")
	if err != nil {
		return fmt.Errorf("moving kube-scheduler manifest: %w", err)
	}

	return nil
}

func (l *SchedulerUnavailableLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)

	testPod := `apiVersion: v1
kind: Pod
metadata:
  name: test-sched-unavailable
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
	output, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "test-sched-unavailable", "-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(output) == "Pending" {
		return nil
	}

	return nil
}

func (l *SchedulerUnavailableLab) Verify(ctx context.Context, kubeconfigPath string) error {
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
  name: verify-sched-unavailable
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
	output, err = kubectl(ctx, kubeconfigPath, "get", "pod", "verify-sched-unavailable",
		"-o", "jsonpath={.spec.nodeName}")
	if err != nil {
		kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-unavailable", "--ignore-not-found=true")
		return fmt.Errorf("failed to check test pod: %w", err)
	}

	kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-unavailable", "--ignore-not-found=true")

	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("test pod was not scheduled to a node")
	}

	return nil
}

func (l *SchedulerUnavailableLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the scheduler pod status",
			Command:     "kubectl get pods -n kube-system | grep scheduler",
			Notes:       "The kube-scheduler pod may be missing entirely",
		},
		{
			Description: "Access the control plane node",
			Command:     "docker exec -it <cluster-name>-control-plane bash",
		},
		{
			Description: "List static pod manifests",
			Command:     "ls -la /etc/kubernetes/manifests/",
			Notes:       "kube-scheduler.yaml should be missing from this directory",
		},
		{
			Description: "Search for the moved manifest",
			Command:     "find /etc/kubernetes -name '*scheduler*'",
			Notes:       "The manifest was moved to /etc/kubernetes/kube-scheduler.yaml.bak",
		},
		{
			Description: "Restore the manifest to the manifests directory",
			Command:     "mv /etc/kubernetes/kube-scheduler.yaml.bak /etc/kubernetes/manifests/kube-scheduler.yaml",
			Notes:       "The kubelet will detect the file and recreate the static pod",
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
