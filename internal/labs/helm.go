package labs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// helm runs the helm CLI with the given kubeconfig.
func helm(ctx context.Context, kubeconfigPath string, args ...string) (string, error) {
	fullArgs := append([]string{"--kubeconfig", kubeconfigPath}, args...)
	cmd := exec.CommandContext(ctx, "helm", fullArgs...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func requireHelm(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "helm", "version", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm is required for this lab but was not found or failed (%s): %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func helmReleaseStatus(ctx context.Context, kubeconfigPath, release, namespace string) (string, error) {
	output, err := helm(ctx, kubeconfigPath, "status", release, "-n", namespace, "-o", "json")
	if err != nil {
		return "", fmt.Errorf("helm status %s: %w", release, err)
	}
	// Prefer jsonpath via helm get / status; parse lightly
	if strings.Contains(output, `"status":"deployed"`) || strings.Contains(output, `"status": "deployed"`) {
		return "deployed", nil
	}
	// helm status --show-desc text fallback
	text, err := helm(ctx, kubeconfigPath, "status", release, "-n", namespace)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "STATUS:")), nil
		}
		if strings.HasPrefix(line, "STATUS:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "STATUS:")), nil
		}
	}
	return strings.TrimSpace(output), nil
}

func helmReleaseRevision(ctx context.Context, kubeconfigPath, release, namespace string) (int, error) {
	hist, err := helm(ctx, kubeconfigPath, "history", release, "-n", namespace, "--max", "1", "-o", "json")
	if err == nil {
		if rev := extractJSONInt(hist, "revision"); rev > 0 {
			return rev, nil
		}
	}

	list, err := helm(ctx, kubeconfigPath, "list", "-n", namespace, "-f", "^"+release+"$", "-o", "json")
	if err == nil {
		if rev := extractJSONInt(list, "revision"); rev > 0 {
			return rev, nil
		}
		if rev := extractJSONInt(list, "version"); rev > 0 {
			return rev, nil
		}
	}

	status, err := helm(ctx, kubeconfigPath, "status", release, "-n", namespace, "-o", "json")
	if err != nil {
		return 0, fmt.Errorf("could not determine revision for release %s/%s: %w", namespace, release, err)
	}
	if rev := extractJSONInt(status, "revision"); rev > 0 {
		return rev, nil
	}
	if rev := extractJSONInt(status, "version"); rev > 0 {
		return rev, nil
	}
	return 0, fmt.Errorf("could not determine revision for release %s/%s", namespace, release)
}

func extractJSONInt(blob, key string) int {
	needle := `"` + key + `":`
	idx := strings.Index(blob, needle)
	if idx == -1 {
		needle = `"` + key + `": `
		idx = strings.Index(blob, needle)
	}
	if idx == -1 {
		return 0
	}
	rest := blob[idx+len(needle):]
	rest = strings.TrimLeft(rest, " ")
	n := 0
	for _, c := range rest {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func ensureOptCKA(ctx context.Context, kubeconfigPath string) (string, error) {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return "", err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return "", fmt.Errorf("creating /opt/CKA on node: %w", err)
	}
	return node, nil
}

// syncChartToExamPaths materializes the embedded chart to the host mirror and
// copies it onto the control-plane node under /opt/CKA/charts.
func syncChartToExamPaths(ctx context.Context, kubeconfigPath string) (hostChartDir string, err error) {
	hostChartDir, err = materializeCKAWebappChart(hostChartMirrorRoot)
	if err != nil {
		return "", err
	}
	node, err := ensureOptCKA(ctx, kubeconfigPath)
	if err != nil {
		return "", err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA/charts"); err != nil {
		return "", err
	}
	// Replace remote chart directory
	_, _ = dockerExec(ctx, node, "rm", "-rf", "/opt/CKA/charts/"+ckaWebappChartName)
	if err := dockerCp(ctx, hostChartDir, node+":/opt/CKA/charts/"); err != nil {
		return "", fmt.Errorf("copying chart to node: %w", err)
	}
	return hostChartDir, nil
}
