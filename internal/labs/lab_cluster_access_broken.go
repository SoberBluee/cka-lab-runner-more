package labs

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func init() {
	Register(&ClusterAccessBrokenLab{})
}

type ClusterAccessBrokenLab struct {
	BaseLab
}

func (l *ClusterAccessBrokenLab) ID() string             { return "cluster_access_broken" }
func (l *ClusterAccessBrokenLab) Title() string          { return "Cluster Access Broken" }
func (l *ClusterAccessBrokenLab) Category() Category     { return CategoryControlPlane }
func (l *ClusterAccessBrokenLab) Difficulty() Difficulty { return DifficultyMedium }
func (l *ClusterAccessBrokenLab) EstimatedTime() int     { return 12 }
func (l *ClusterAccessBrokenLab) Hints() []string        { return nil }
func (l *ClusterAccessBrokenLab) Tags() []string {
	return []string{"troubleshooting", "kubeconfig", "authentication"}
}

func (l *ClusterAccessBrokenLab) Description() string {
	return `Set the context before doing any work:

  kubectl config use-context cka-lab

[Weight: 8%] | Time limit: 8–12 minutes

Context:
A kubeconfig file called admin.kubeconfig has been created at /opt/CKA/admin.kubeconfig
on the control-plane node. There is something wrong with the configuration.

Task:
1. Troubleshoot and fix /opt/CKA/admin.kubeconfig so it can talk to this cluster.
2. Success means: kubectl --kubeconfig=/opt/CKA/admin.kubeconfig get nodes works
   when run on the control-plane node.

Constraints:
- Fix the existing file in place (do not rename it).
- Do not break the node default admin.conf / cluster access used by the lab runner.
`
}

func (l *ClusterAccessBrokenLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *ClusterAccessBrokenLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return err
	}

	// Copy a working admin.conf then corrupt the server URL (clear, single fault).
	raw, err := dockerExec(ctx, node, "cat", "/etc/kubernetes/admin.conf")
	if err != nil {
		return fmt.Errorf("reading admin.conf: %w", err)
	}
	lines := strings.Split(raw, "\n")
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "server:") && strings.Contains(trimmed, "https://") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "server: https://127.0.0.1:1"
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("could not locate server: line in admin.conf to corrupt")
	}
	broken := strings.Join(lines, "\n")

	tmp, err := os.CreateTemp("", "admin.kubeconfig-*")
	if err != nil {
		return err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(broken); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := dockerCp(ctx, path, node+":/opt/CKA/admin.kubeconfig"); err != nil {
		return fmt.Errorf("staging broken kubeconfig: %w", err)
	}
	_, _ = dockerExec(ctx, node, "chmod", "600", "/opt/CKA/admin.kubeconfig")
	return nil
}

func (l *ClusterAccessBrokenLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	_, err = dockerExec(ctx, node, "kubectl", "--kubeconfig=/opt/CKA/admin.kubeconfig", "get", "nodes")
	if err == nil {
		return fmt.Errorf("admin.kubeconfig already works")
	}
	return nil
}

func (l *ClusterAccessBrokenLab) Verify(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	out, err := dockerExec(ctx, node, "kubectl", "--kubeconfig=/opt/CKA/admin.kubeconfig", "get", "nodes",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return fmt.Errorf("kubectl --kubeconfig=/opt/CKA/admin.kubeconfig get nodes failed: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("admin.kubeconfig get nodes returned no nodes")
	}

	// Minimal mutation: server should no longer point at the bogus port
	cfg, err := dockerExec(ctx, node, "cat", "/opt/CKA/admin.kubeconfig")
	if err != nil {
		return err
	}
	if strings.Contains(cfg, "https://127.0.0.1:1") {
		return fmt.Errorf("admin.kubeconfig still points at the broken server URL")
	}
	return nil
}

func (l *ClusterAccessBrokenLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Reproduce the failure on the control-plane",
			Command: `docker exec -it cka-lab-control-plane bash
kubectl --kubeconfig=/opt/CKA/admin.kubeconfig get nodes`,
			Notes: "Docs search: Organizing Cluster Access Using kubeconfig Files",
		},
		{
			Description: "Compare with a known-good kubeconfig",
			Command: `grep server /opt/CKA/admin.kubeconfig
grep server /etc/kubernetes/admin.conf`,
		},
		{
			Description: "Fix the server URL (or copy a working cluster stanza)",
			Command: `# Simplest fix on kind:
cp /etc/kubernetes/admin.conf /opt/CKA/admin.kubeconfig
# Or edit only the server: line to match admin.conf
kubectl --kubeconfig=/opt/CKA/admin.kubeconfig get nodes`,
		},
	}
}
