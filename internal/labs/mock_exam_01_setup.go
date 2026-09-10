package labs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	mockExamChartContainer = "cka-mock-chart-repo"
	mockExamChartURL       = "http://127.0.0.1:18080"
)

func mockExamPreflight(ctx context.Context, kubeconfigPath string) error {
	for _, binary := range []string{"docker", "helm", "kubectl"} {
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("%s is required for mock_exam_01", binary)
		}
	}
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "sh", "-c",
		"test -f /etc/kubernetes/admin.conf && crictl --version >/dev/null"); err != nil {
		return fmt.Errorf("mock_exam_01 requires a Docker-based kind node with crictl: %w", err)
	}
	return nil
}

func setupMockExam01(ctx context.Context, kubeconfigPath string) error {
	if err := mockExamPreflight(ctx, kubeconfigPath); err != nil {
		return err
	}
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	cleanupMockExamHost(ctx, kubeconfigPath)

	if err := kubectlApply(ctx, kubeconfigPath, mockExamVPAAndGatewayCRDs); err != nil {
		return fmt.Errorf("installing exam CRDs: %w", err)
	}
	for _, crd := range []string{
		"verticalpodautoscalers.autoscaling.k8s.io",
		"verticalpodautoscalercheckpoints.autoscaling.k8s.io",
		"gatewayclasses.gateway.networking.k8s.io",
		"gateways.gateway.networking.k8s.io",
	} {
		if _, err := kubectl(ctx, kubeconfigPath, "wait", "--for=condition=Established",
			"crd/"+crd, "--timeout=60s"); err != nil {
			return fmt.Errorf("waiting for %s: %w", crd, err)
		}
	}
	if err := kubectlApply(ctx, kubeconfigPath, mockExamBaseResources); err != nil {
		return fmt.Errorf("creating exam resources: %w", err)
	}

	if err := setupMockExamNode(ctx, node); err != nil {
		return err
	}
	if err := setupMockExamChartRepo(ctx, kubeconfigPath); err != nil {
		return err
	}
	return nil
}

func setupMockExamNode(ctx context.Context, node string) error {
	script := `set -eu
mkdir -p /root/.kube
cp /etc/kubernetes/admin.conf /root/.kube/config
rm -f /root/vpa-crds.txt
if [ -f /etc/crictl.yaml ] && [ ! -f /etc/crictl.yaml.cka-mock-backup ]; then
  cp /etc/crictl.yaml /etc/crictl.yaml.cka-mock-backup
fi
cat >/etc/crictl.yaml <<'EOF'
runtime-endpoint: unix:///run/invalid-cri.sock
image-endpoint: unix:///run/invalid-cri.sock
timeout: 2
debug: false
EOF`
	if output, err := dockerExec(ctx, node, "sh", "-c", script); err != nil {
		return fmt.Errorf("preparing control-plane node: %s: %w", output, err)
	}

	file, err := os.CreateTemp("", "webapp-hpa-*.yaml")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(mockExamHPASkeleton); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := dockerCp(ctx, path, node+":/root/webapp-hpa.yaml"); err != nil {
		return fmt.Errorf("staging HPA manifest: %w", err)
	}
	return nil
}

func setupMockExamChartRepo(ctx context.Context, kubeconfigPath string) error {
	root := mockExamTempDir()
	repoDir := filepath.Join(root, "repo")
	chartDir := filepath.Join(root, "podinfo")
	if err := os.RemoveAll(root); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(chartDir, "templates"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(chartDir, "templates", "deployment.yaml"),
		[]byte(mockExamChartTemplate), 0644); err != nil {
		return err
	}
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return err
	}
	if err := packageMockExamChart(ctx, chartDir, repoDir, "6.11.1"); err != nil {
		return err
	}
	if _, err := runHostCommand(ctx, "helm", "repo", "index", repoDir, "--url", mockExamChartURL); err != nil {
		return fmt.Errorf("indexing initial chart repository: %w", err)
	}

	mount := repoDir + ":/usr/share/nginx/html:ro"
	if output, err := runHostCommand(ctx, "docker", "run", "-d", "--name", mockExamChartContainer,
		"-p", "127.0.0.1:18080:80", "-v", mount, "nginx:alpine"); err != nil {
		return fmt.Errorf("starting local chart repository: %s: %w", output, err)
	}
	if err := waitFor(ctx, 30*time.Second, func() error {
		_, err := runHostCommand(ctx, "helm", "repo", "add", "kk-mock1", mockExamChartURL, "--force-update")
		return err
	}); err != nil {
		return fmt.Errorf("adding local chart repository: %w", err)
	}
	_, _ = runHostCommand(ctx, "helm", "uninstall", "kk-mock1", "-n", "kk-ns",
		"--kubeconfig", kubeconfigPath)
	if output, err := runHostCommand(ctx, "helm", "install", "kk-mock1", "kk-mock1/podinfo",
		"-n", "kk-ns", "--version", "6.11.1", "--kubeconfig", kubeconfigPath); err != nil {
		return fmt.Errorf("installing initial chart release: %s: %w", output, err)
	}

	if err := packageMockExamChart(ctx, chartDir, repoDir, "6.11.2"); err != nil {
		return err
	}
	if _, err := runHostCommand(ctx, "helm", "repo", "index", repoDir, "--url", mockExamChartURL); err != nil {
		return fmt.Errorf("publishing chart update: %w", err)
	}
	return nil
}

func packageMockExamChart(ctx context.Context, chartDir, repoDir, version string) error {
	chart := fmt.Sprintf("apiVersion: v2\nname: podinfo\ndescription: Local mock-exam chart\ntype: application\nversion: %s\nappVersion: %q\n", version, version)
	if err := os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(chart), 0644); err != nil {
		return err
	}
	image := "nginx:alpine"
	if version == "6.11.1" {
		image = "nginx:mock-exam-old"
	}
	if err := os.WriteFile(filepath.Join(chartDir, "values.yaml"),
		[]byte("image: "+image+"\n"), 0644); err != nil {
		return err
	}
	if output, err := runHostCommand(ctx, "helm", "package", chartDir, "--destination", repoDir); err != nil {
		return fmt.Errorf("packaging chart %s: %s: %w", version, output, err)
	}
	return nil
}

func runHostCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func mockExamTempDir() string {
	return filepath.Join(os.TempDir(), "cka-lab-mock-exam")
}

func cleanupMockExamHost(ctx context.Context, kubeconfigPath string) {
	_, _ = runHostCommand(ctx, "helm", "uninstall", "kk-mock1", "-n", "kk-ns",
		"--kubeconfig", kubeconfigPath)
	_, _ = runHostCommand(ctx, "helm", "repo", "remove", "kk-mock1")
	_, _ = runHostCommand(ctx, "docker", "rm", "-f", mockExamChartContainer)
	_ = os.RemoveAll(mockExamTempDir())
}

func cleanupMockExamResources(ctx context.Context, kubeconfigPath string) error {
	cleanupMockExamHost(ctx, kubeconfigPath)
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "gatewayclass", "nginx",
		"--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pv", "pv-analytics",
		"--ignore-not-found=true", "--wait=false")

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	script := `set -eu
if [ -f /etc/crictl.yaml.cka-mock-backup ]; then
  mv /etc/crictl.yaml.cka-mock-backup /etc/crictl.yaml
else
  cat >/etc/crictl.yaml <<'EOF'
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 2
debug: false
EOF
fi
rm -f /root/vpa-crds.txt /root/webapp-hpa.yaml /root/gateway-crds.txt`
	_, _ = dockerExec(ctx, node, "sh", "-c", script)
	return nil
}
