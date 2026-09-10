package labs

import (
	"context"
	"fmt"
	"os/exec"
)

func mockExam02Preflight(ctx context.Context, kubeconfigPath string) error {
	for _, binary := range []string{"docker", "kubectl"} {
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("%s is required for mock_exam_02", binary)
		}
	}
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "sh", "-c", "test -f /etc/kubernetes/admin.conf"); err != nil {
		return fmt.Errorf("mock_exam_02 requires a Docker-based kind control-plane node: %w", err)
	}
	return nil
}

func setupMockExam02(ctx context.Context, kubeconfigPath string) error {
	if err := mockExam02Preflight(ctx, kubeconfigPath); err != nil {
		return err
	}
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	_ = cleanupMockExam02Resources(ctx, kubeconfigPath)

	if err := kubectlApply(ctx, kubeconfigPath, mockExam02CRDs); err != nil {
		return fmt.Errorf("installing exam CRDs: %w", err)
	}
	for _, crd := range []string{
		"verticalpodautoscalers.autoscaling.k8s.io",
		"verticalpodautoscalercheckpoints.autoscaling.k8s.io",
		"gatewayclasses.gateway.networking.k8s.io",
		"gateways.gateway.networking.k8s.io",
		"httproutes.gateway.networking.k8s.io",
		"widgets.ops.exam.local",
	} {
		if _, err := kubectl(ctx, kubeconfigPath, "wait", "--for=condition=Established",
			"crd/"+crd, "--timeout=60s"); err != nil {
			return fmt.Errorf("waiting for %s: %w", crd, err)
		}
	}
	if err := kubectlApply(ctx, kubeconfigPath, mockExam02BaseResources); err != nil {
		return fmt.Errorf("creating exam resources: %w", err)
	}
	return setupMockExam02Node(ctx, node)
}

func setupMockExam02Node(ctx context.Context, node string) error {
	script := `set -eu
mkdir -p /root/.kube
cp /etc/kubernetes/admin.conf /root/.kube/config
rm -f /root/gateway-crds.txt`
	if output, err := dockerExec(ctx, node, "sh", "-c", script); err != nil {
		return fmt.Errorf("preparing control-plane node: %s: %w", output, err)
	}
	return nil
}

func cleanupMockExam02Resources(ctx context.Context, kubeconfigPath string) error {
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "gatewayclass", "nginx",
		"--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "widget", "storefront", "-n", "ops",
		"--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "hpa", "checkout-hpa",
		"--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "verticalpodautoscaler", "billing-vpa",
		"--ignore-not-found=true", "--wait=false")

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/root/gateway-crds.txt")
	return nil
}
