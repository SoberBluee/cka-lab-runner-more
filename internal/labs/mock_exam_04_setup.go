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

func mockExam04Preflight(ctx context.Context, kubeconfigPath string) error {
	for _, binary := range []string{"docker", "kubectl", "helm", "openssl"} {
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("%s is required for mock_exam_04", binary)
		}
	}
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "sh", "-c", "test -d /etc/kubernetes/manifests"); err != nil {
		return fmt.Errorf("mock_exam_04 requires a Docker-based kind control-plane node: %w", err)
	}
	return nil
}

func setupMockExam04(ctx context.Context, kubeconfigPath string) error {
	if err := mockExam04Preflight(ctx, kubeconfigPath); err != nil {
		return err
	}
	_ = cleanupMockExam04Resources(ctx, kubeconfigPath)

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return fmt.Errorf("creating /opt/CKA: %w", err)
	}

	if err := kubectlApply(ctx, kubeconfigPath, mockExam04CRDs); err != nil {
		return fmt.Errorf("installing exam CRDs: %w", err)
	}
	for _, crd := range []string{
		"gatewayclasses.gateway.networking.k8s.io",
		"gateways.gateway.networking.k8s.io",
		"httproutes.gateway.networking.k8s.io",
	} {
		if _, err := kubectl(ctx, kubeconfigPath, "wait", "--for=condition=Established",
			"crd/"+crd, "--timeout=60s"); err != nil {
			return fmt.Errorf("waiting for %s: %w", crd, err)
		}
	}

	if err := kubectlApply(ctx, kubeconfigPath, mockExam04BaseResources); err != nil {
		return fmt.Errorf("creating exam resources: %w", err)
	}

	if err := setupMockExam04TLSSecret(ctx, kubeconfigPath); err != nil {
		return err
	}
	if err := setupMockExam04NodeFiles(ctx, node); err != nil {
		return err
	}
	if err := setupMockExam04HelmReleases(ctx, kubeconfigPath); err != nil {
		return err
	}
	return nil
}

func setupMockExam04TLSSecret(ctx context.Context, kubeconfigPath string) error {
	dir, err := os.MkdirTemp("", "mock04-tls-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	key := filepath.Join(dir, "tls.key")
	crt := filepath.Join(dir, "tls.crt")
	if out, err := runHostCommand(ctx, "openssl", "req", "-x509", "-nodes", "-newkey", "rsa:2048",
		"-keyout", key, "-out", crt, "-days", "365",
		"-subj", "/CN=shop.exam.local"); err != nil {
		return fmt.Errorf("generating TLS material: %s: %w", out, err)
	}
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "secret", "shop-tls", "-n", "edge-gw",
		"--ignore-not-found=true")
	if _, err := kubectl(ctx, kubeconfigPath, "create", "secret", "tls", "shop-tls",
		"-n", "edge-gw", "--cert="+crt, "--key="+key); err != nil {
		return fmt.Errorf("creating shop-tls secret: %w", err)
	}
	return nil
}

func setupMockExam04NodeFiles(ctx context.Context, node string) error {
	dir, err := os.MkdirTemp("", "mock04-files-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	keyPath := filepath.Join(dir, "maria.key")
	csrPath := filepath.Join(dir, "maria.csr")
	if out, err := runHostCommand(ctx, "openssl", "genrsa", "-out", keyPath, "2048"); err != nil {
		return fmt.Errorf("generating maria.key: %s: %w", out, err)
	}
	if out, err := runHostCommand(ctx, "openssl", "req", "-new", "-key", keyPath, "-out", csrPath,
		"-subj", "/CN=maria/O=developers"); err != nil {
		return fmt.Errorf("generating maria.csr: %s: %w", out, err)
	}

	files := map[string]string{
		"maria.key":       keyPath,
		"maria.csr":       csrPath,
		"worker-hpa.yaml": "",
		"netpol-1.yaml":   "",
		"netpol-2.yaml":   "",
		"netpol-3.yaml":   "",
	}
	if err := os.WriteFile(filepath.Join(dir, "worker-hpa.yaml"), []byte(mockExam04HPASkeleton), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "netpol-1.yaml"), []byte(mockExam04Netpol1), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "netpol-2.yaml"), []byte(mockExam04Netpol2), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "netpol-3.yaml"), []byte(mockExam04Netpol3), 0644); err != nil {
		return err
	}
	files["worker-hpa.yaml"] = filepath.Join(dir, "worker-hpa.yaml")
	files["netpol-1.yaml"] = filepath.Join(dir, "netpol-1.yaml")
	files["netpol-2.yaml"] = filepath.Join(dir, "netpol-2.yaml")
	files["netpol-3.yaml"] = filepath.Join(dir, "netpol-3.yaml")

	_, _ = dockerExec(ctx, node, "rm", "-f",
		"/opt/CKA/maria.key", "/opt/CKA/maria.csr", "/opt/CKA/worker-hpa.yaml",
		"/opt/CKA/netpol-1.yaml", "/opt/CKA/netpol-2.yaml", "/opt/CKA/netpol-3.yaml",
		"/opt/CKA/dns.svc", "/opt/CKA/dns.pod",
		"/etc/kubernetes/manifests/priority-web.yaml")

	for name, src := range files {
		if err := dockerCp(ctx, src, node+":/opt/CKA/"+name); err != nil {
			return fmt.Errorf("staging /opt/CKA/%s: %w", name, err)
		}
	}
	return nil
}

func setupMockExam04HelmReleases(ctx context.Context, kubeconfigPath string) error {
	chartDir, err := materializeCKAWebappChart(hostChartMirrorRoot)
	if err != nil {
		return err
	}

	_, _ = helm(ctx, kubeconfigPath, "uninstall", "safe-portal", "-n", "charts-safe")
	_, _ = helm(ctx, kubeconfigPath, "uninstall", "legacy-color", "-n", "charts-legacy")
	_, _ = helm(ctx, kubeconfigPath, "uninstall", "docs-site", "-n", "charts-safe")

	if out, err := helm(ctx, kubeconfigPath, "install", "safe-portal", chartDir,
		"-n", "charts-safe", "--set", "image.repository=nginx", "--set", "image.tag=alpine",
		"--wait", "--timeout", "120s"); err != nil {
		return fmt.Errorf("installing safe-portal: %s: %w", out, err)
	}
	if out, err := helm(ctx, kubeconfigPath, "install", "docs-site", chartDir,
		"-n", "charts-safe", "--set", "image.repository=nginx", "--set", "image.tag=1.25-alpine",
		"--set", "fullnameOverride=docs-site", "--wait", "--timeout", "120s"); err != nil {
		return fmt.Errorf("installing docs-site: %s: %w", out, err)
	}
	if out, err := helm(ctx, kubeconfigPath, "install", "legacy-color", chartDir,
		"-n", "charts-legacy", "--set", "image.repository=nginx", "--set", "image.tag=1.14-alpine",
		"--wait", "--timeout", "180s"); err != nil {
		return fmt.Errorf("installing legacy-color: %s: %w", out, err)
	}
	return nil
}

func cleanupMockExam04Resources(ctx context.Context, kubeconfigPath string) error {
	_, _ = helm(ctx, kubeconfigPath, "uninstall", "safe-portal", "-n", "charts-safe")
	_, _ = helm(ctx, kubeconfigPath, "uninstall", "legacy-color", "-n", "charts-legacy")
	_, _ = helm(ctx, kubeconfigPath, "uninstall", "docs-site", "-n", "charts-safe")

	_, _ = kubectl(ctx, kubeconfigPath, "delete", "csr", "maria-developer", "--ignore-not-found=true")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "storageclass", "disk-local", "--ignore-not-found=true")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "gateway", "edge-gateway", "-n", "edge-gw",
		"--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "gatewayclass", "cka-gateway",
		"--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "deploy", "web-rollout", "--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pod", "dns-probe", "--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "svc", "dns-probe-svc", "--ignore-not-found=true", "--wait=false")

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err == nil {
		_, _ = dockerExec(ctx, node, "rm", "-f",
			"/etc/kubernetes/manifests/priority-web.yaml",
			"/opt/CKA/maria.key", "/opt/CKA/maria.csr", "/opt/CKA/worker-hpa.yaml",
			"/opt/CKA/netpol-1.yaml", "/opt/CKA/netpol-2.yaml", "/opt/CKA/netpol-3.yaml",
			"/opt/CKA/dns.svc", "/opt/CKA/dns.pod")
		// Give kubelet a moment to remove the static pod mirror
		time.Sleep(2 * time.Second)
		_, _ = kubectl(ctx, kubeconfigPath, "delete", "pod", "-l", "app=priority-web",
			"--ignore-not-found=true", "--wait=false", "--force", "--grace-period=0")
		_, _ = kubectl(ctx, kubeconfigPath, "delete", "pod", "priority-web-"+node,
			"--ignore-not-found=true", "--wait=false", "--force", "--grace-period=0")
	}

	for _, ns := range []string{
		"publish", "edge", "team-dev", "api", "edge-gw",
		"charts-safe", "charts-legacy", "frontend", "backend", "databases",
	} {
		_, _ = kubectl(ctx, kubeconfigPath, "delete", "ns", ns, "--ignore-not-found=true", "--wait=false")
	}

	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		out, err := kubectl(ctx, kubeconfigPath, "get", "ns", "-o", "jsonpath={.items[*].metadata.name}")
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		remaining := false
		for _, ns := range []string{"publish", "edge", "team-dev", "api", "edge-gw", "charts-safe", "charts-legacy", "frontend", "backend", "databases"} {
			if strings.Contains(out, ns) {
				remaining = true
				break
			}
		}
		if !remaining {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}
