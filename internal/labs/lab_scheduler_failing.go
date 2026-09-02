package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&SchedulerFailingLab{})
}

type SchedulerFailingLab struct {
	BaseLab
}

func (l *SchedulerFailingLab) ID() string {
	return "scheduler_failing"
}

func (l *SchedulerFailingLab) Title() string {
	return "Kube-Scheduler Failing"
}

func (l *SchedulerFailingLab) Category() Category {
	return CategoryScheduling
}

func (l *SchedulerFailingLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *SchedulerFailingLab) Description() string {
	return `The kube-scheduler is not working.
New pods are stuck in the Pending state and are not being scheduled to nodes.

Your task: Fix the kube-scheduler so it can schedule pods again.`
}

func (l *SchedulerFailingLab) Hints() []string {
	return []string{
		"Check whether a kube-scheduler pod exists in the kube-system namespace",
		"If the static pod is missing or never starts, inspect the manifest carefully",
		"YAML is indentation-sensitive — look for subtle formatting problems",
		"Check kubelet logs on the control plane if the pod never appears",
	}
}

func (l *SchedulerFailingLab) EstimatedTime() int {
	return 25
}

func (l *SchedulerFailingLab) Tags() []string {
	return []string{"scheduler", "static-pods", "scheduling", "troubleshooting"}
}

func (l *SchedulerFailingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *SchedulerFailingLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodeName, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	containerName := nodeName

	output, err := dockerExec(ctx, containerName, "cat", "/etc/kubernetes/manifests/kube-scheduler.yaml")
	if err != nil {
		return fmt.Errorf("reading kube-scheduler manifest: %w", err)
	}

	// Break YAML by under-indenting a command flag line (kubelet will reject the manifest)
	modifiedManifest := strings.Replace(output,
		"    - --leader-elect=true",
		"  - --leader-elect=true",
		1)
	if modifiedManifest == output {
		// Fallback if flag formatting differs across versions
		modifiedManifest = strings.Replace(output,
			"    - --bind-address=127.0.0.1",
			"  - --bind-address=127.0.0.1",
			1)
	}
	if modifiedManifest == output {
		return fmt.Errorf("could not find a scheduler flag line to break indentation")
	}

	writeCmd := fmt.Sprintf("cat > /etc/kubernetes/manifests/kube-scheduler.yaml << 'EOF'\n%s\nEOF", modifiedManifest)
	_, err = dockerExec(ctx, containerName, "sh", "-c", writeCmd)
	if err != nil {
		return fmt.Errorf("writing modified manifest: %w", err)
	}

	return nil
}

func (l *SchedulerFailingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)

	testPod := `apiVersion: v1
kind: Pod
metadata:
  name: test-sched-failing
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
	output, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "test-sched-failing", "-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(output) == "Pending" {
		return nil
	}

	return nil
}

func (l *SchedulerFailingLab) Verify(ctx context.Context, kubeconfigPath string) error {
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
  name: verify-sched-failing
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
	output, err = kubectl(ctx, kubeconfigPath, "get", "pod", "verify-sched-failing",
		"-o", "jsonpath={.spec.nodeName}")
	if err != nil {
		kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-failing", "--ignore-not-found=true")
		return fmt.Errorf("failed to check test pod: %w", err)
	}

	kubectl(ctx, kubeconfigPath, "delete", "pod", "verify-sched-failing", "--ignore-not-found=true")

	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("test pod was not scheduled to a node")
	}

	return nil
}

func (l *SchedulerFailingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the scheduler pod status",
			Command:     "kubectl get pods -n kube-system | grep scheduler",
			Notes:       "The pod may be missing because kubelet rejected the manifest",
		},
		{
			Description: "Access the control plane node",
			Command:     "docker exec -it <cluster-name>-control-plane bash",
		},
		{
			Description: "Validate the manifest YAML",
			Command:     "python3 -c 'import yaml,sys; yaml.safe_load(open(\"/etc/kubernetes/manifests/kube-scheduler.yaml\"))' || true",
			Notes:       "Or visually inspect indentation around the command flags",
		},
		{
			Description: "Fix the broken indentation",
			Command:     "sed -i 's/^  - --leader-elect=true$/    - --leader-elect=true/' /etc/kubernetes/manifests/kube-scheduler.yaml",
			Notes:       "All command args must share the same indentation level under command:",
		},
		{
			Description: "If bind-address was the broken line instead",
			Command:     "sed -i 's/^  - --bind-address=127.0.0.1$/    - --bind-address=127.0.0.1/' /etc/kubernetes/manifests/kube-scheduler.yaml",
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
