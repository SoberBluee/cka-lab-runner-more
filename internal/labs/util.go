package labs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// kubectl executes a kubectl command with the given kubeconfig
func kubectl(ctx context.Context, kubeconfigPath string, args ...string) (string, error) {
	fullArgs := append([]string{"--kubeconfig", kubeconfigPath}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// kubectlApply applies a YAML manifest
func kubectlApply(ctx context.Context, kubeconfigPath, yaml string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply failed: %s: %w", string(output), err)
	}
	return nil
}

// getControlPlaneNode returns the name of the control plane node
func getControlPlaneNode(ctx context.Context, kubeconfigPath string) (string, error) {
	output, err := kubectl(ctx, kubeconfigPath, "get", "nodes",
		"-l", "node-role.kubernetes.io/control-plane",
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", fmt.Errorf("getting control plane node: %w", err)
	}

	nodeName := strings.TrimSpace(output)
	if nodeName == "" {
		return "", fmt.Errorf("no control plane node found")
	}

	return nodeName, nil
}

// dockerExec executes a command inside a docker container (for kind nodes)
func dockerExec(ctx context.Context, containerName string, args ...string) (string, error) {
	fullArgs := append([]string{"exec", containerName}, args...)
	cmd := exec.CommandContext(ctx, "docker", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// dockerCp copies a file to/from a docker container
func dockerCp(ctx context.Context, src, dst string) error {
	cmd := exec.CommandContext(ctx, "docker", "cp", src, dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp failed: %s: %w", string(output), err)
	}
	return nil
}

// WaitForClusterReady waits for the cluster to be ready by checking if nodes are accessible
func WaitForClusterReady(ctx context.Context, kubeconfigPath string) error {
	for i := 0; i < 30; i++ {
		_, err := kubectl(ctx, kubeconfigPath, "get", "nodes")
		if err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("cluster did not become ready in time")
}

func waitDNSPodsReady(ctx context.Context, kubeconfigPath string) error {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		phases, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
			"-l", "k8s-app=kube-dns",
			"-o", "jsonpath={.items[*].status.phase}")
		if err == nil {
			fields := strings.Fields(phases)
			if len(fields) > 0 {
				allRunning := true
				for _, p := range fields {
					if p != "Running" {
						allRunning = false
						break
					}
				}
				if allRunning {
					ready, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
						"-l", "k8s-app=kube-dns",
						"-o", "jsonpath={.items[*].status.containerStatuses[*].ready}")
					if strings.Contains(ready, "true") && !strings.Contains(ready, "false") {
						return nil
					}
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("DNS pods not ready in time")
}
