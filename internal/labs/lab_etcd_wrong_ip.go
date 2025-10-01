package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&EtcdWrongIPLab{})
}

type EtcdWrongIPLab struct{}

func (l *EtcdWrongIPLab) ID() string {
	return "etcd_wrong_ip"
}

func (l *EtcdWrongIPLab) Title() string {
	return "Etcd Wrong IP Address"
}

func (l *EtcdWrongIPLab) Category() Category {
	return CategoryControlPlane
}

func (l *EtcdWrongIPLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *EtcdWrongIPLab) Description() string {
	return `The Kubernetes API server cannot communicate with etcd.
The cluster is in a broken state and the API server is not responding properly.

Your task: Fix the etcd configuration so the API server can communicate with it again.`
}

func (l *EtcdWrongIPLab) Hints() []string {
	return []string{
		"Check the static pod manifests in /etc/kubernetes/manifests on the control plane node",
		"Look for etcd-related configuration in the kube-apiserver manifest",
		"The etcd servers are configured via the --etcd-servers flag",
		"You may need to access the control plane node using docker exec",
	}
}

func (l *EtcdWrongIPLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	// Wait for cluster to be ready
	for i := 0; i < 30; i++ {
		_, err := kubectl(ctx, kubeconfigPath, "get", "nodes")
		if err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("cluster did not become ready in time")
}

func (l *EtcdWrongIPLab) Break(ctx context.Context, kubeconfigPath string) error {
	// Get the control plane node name
	nodeName, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	// For kind clusters, the node name is also the container name
	containerName := nodeName

	// Read the current kube-apiserver manifest
	output, err := dockerExec(ctx, containerName, "cat", "/etc/kubernetes/manifests/kube-apiserver.yaml")
	if err != nil {
		return fmt.Errorf("reading kube-apiserver manifest: %w", err)
	}

	// Replace the etcd-servers IP with a wrong one
	// Change https://127.0.0.1:2379 to https://127.0.0.2:2379
	modifiedManifest := strings.ReplaceAll(output, "https://127.0.0.1:2379", "https://127.0.0.2:2379")

	// Write the modified manifest back
	writeCmd := fmt.Sprintf("cat > /etc/kubernetes/manifests/kube-apiserver.yaml << 'EOF'\n%s\nEOF", modifiedManifest)
	_, err = dockerExec(ctx, containerName, "sh", "-c", writeCmd)
	if err != nil {
		return fmt.Errorf("writing modified manifest: %w", err)
	}

	return nil
}

func (l *EtcdWrongIPLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	// Wait a bit for the API server to restart and fail
	time.Sleep(10 * time.Second)

	// Try to query the API - it should fail or be very slow
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := kubectl(ctx, kubeconfigPath, "get", "nodes")
	if err != nil {
		// Expected - the API server is broken
		return nil
	}

	// If it succeeded, the lab might not be fully broken yet
	return nil
}

func (l *EtcdWrongIPLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Access the control plane node",
			Command:     "docker exec -it <cluster-name>-control-plane bash",
			Notes:       "For kind clusters, the container name is typically <cluster-name>-control-plane",
		},
		{
			Description: "Check the kube-apiserver manifest",
			Command:     "cat /etc/kubernetes/manifests/kube-apiserver.yaml | grep etcd-servers",
			Notes:       "Look for the --etcd-servers flag and verify the IP address",
		},
		{
			Description: "Identify the incorrect IP address",
			Notes:       "The etcd server IP should be 127.0.0.1:2379, not 127.0.0.2:2379",
		},
		{
			Description: "Fix the kube-apiserver manifest",
			Command:     "sed -i 's|https://127.0.0.2:2379|https://127.0.0.1:2379|g' /etc/kubernetes/manifests/kube-apiserver.yaml",
			Notes:       "The kubelet will automatically detect the change and restart the API server",
		},
		{
			Description: "Wait for the API server to restart",
			Command:     "kubectl get nodes",
			Notes:       "This may take 30-60 seconds. Once it succeeds, the cluster is fixed.",
		},
		{
			Description: "Verify the cluster is healthy",
			Command:     "kubectl get pods -A",
			Notes:       "All system pods should be running",
		},
	}
}
