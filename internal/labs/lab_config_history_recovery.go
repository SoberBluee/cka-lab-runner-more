package labs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func init() {
	Register(&ConfigHistoryRecoveryLab{})
}

type ConfigHistoryRecoveryLab struct {
	BaseLab
}

const (
	priorStatePath   = "/var/lib/etcd/pre-change.db"
	rebuiltStateDir  = "/var/lib/etcd/restored"
	rebuiltMemberDir = "/var/lib/etcd/restored/member"
)

func (l *ConfigHistoryRecoveryLab) ID() string {
	return "config_history_recovery"
}

func (l *ConfigHistoryRecoveryLab) Title() string {
	return "Deleted Config Must Come Back From History"
}

func (l *ConfigHistoryRecoveryLab) Category() Category {
	return CategoryControlPlane
}

func (l *ConfigHistoryRecoveryLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *ConfigHistoryRecoveryLab) Description() string {
	return `A cleanup script deleted the 'ledger-config' ConfigMap from the default namespace
and nobody has the original values. A point-in-time copy of cluster state taken before the
deletion sits at /var/lib/etcd/pre-change.db on the control plane node.

You are not allowed to swing the live cluster onto that copy during business hours, so the
recovery has to be rehearsed and evidenced first.

Your task:
  1. Rebuild a datastore directory from /var/lib/etcd/pre-change.db at /var/lib/etcd/restored.
  2. Record the rehearsal in a ConfigMap named 'restore-record' in kube-system with keys
     restored-from (the file you used) and member-dir (the member directory you produced).`
}

func (l *ConfigHistoryRecoveryLab) Hints() []string {
	return []string{
		"The tooling that reads these files ships inside the datastore's own static pod",
		"Rebuilding a directory from a saved copy is a local, offline operation — it needs no endpoints and no certificates",
		"The target directory must not already exist, or the command refuses to run",
		"A successful rebuild produces a member/ directory containing snap/ and wal/",
		"Newer builds use etcdutl for this; older ones use etcdctl with ETCDCTL_API=3",
	}
}

func (l *ConfigHistoryRecoveryLab) EstimatedTime() int {
	return 30
}

func (l *ConfigHistoryRecoveryLab) Tags() []string {
	return []string{"control-plane", "datastore", "recovery", "procedure"}
}

func (l *ConfigHistoryRecoveryLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}

	seed := `apiVersion: v1
kind: ConfigMap
metadata:
  name: ledger-config
  namespace: default
data:
  ledger-mode: "append-only"
  retention: "7y"
  owner: "treasury"
`
	return kubectlApply(ctx, kubeconfigPath, seed)
}

func (l *ConfigHistoryRecoveryLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	if _, err := dockerExec(ctx, node, "rm", "-rf", rebuiltStateDir, priorStatePath); err != nil {
		return fmt.Errorf("clearing previous lab state: %w", err)
	}

	saveArgs := append([]string{"etcdctl", "snapshot", "save", priorStatePath}, etcdPKIArgs()...)
	if output, err := etcdExec(ctx, kubeconfigPath, saveArgs...); err != nil {
		return fmt.Errorf("creating the prior-state copy failed: %s: %w", strings.TrimSpace(output), err)
	}

	sizeOutput, err := dockerExec(ctx, node, "stat", "-c", "%s", priorStatePath)
	if err != nil {
		return fmt.Errorf("prior-state copy not visible on the node: %w", err)
	}
	if size, _ := strconv.Atoi(strings.TrimSpace(sizeOutput)); size < 100000 {
		return fmt.Errorf("prior-state copy is only %s bytes", strings.TrimSpace(sizeOutput))
	}

	if _, err := kubectl(ctx, kubeconfigPath, "delete", "configmap", "ledger-config",
		"-n", "default", "--ignore-not-found=true"); err != nil {
		return fmt.Errorf("deleting ledger-config: %w", err)
	}
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "configmap", "restore-record",
		"-n", "kube-system", "--ignore-not-found=true")

	return nil
}

func (l *ConfigHistoryRecoveryLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	if _, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "ledger-config", "-n", "default"); err == nil {
		return fmt.Errorf("ledger-config still exists")
	}
	return nil
}

func (l *ConfigHistoryRecoveryLab) Verify(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	if _, err := dockerExec(ctx, node, "test", "-d", rebuiltMemberDir+"/wal"); err != nil {
		return fmt.Errorf("no rebuilt datastore directory at %s (expected member/wal inside it)", rebuiltStateDir)
	}

	sizeOutput, err := dockerExec(ctx, node, "stat", "-c", "%s", rebuiltMemberDir+"/snap/db")
	if err != nil {
		return fmt.Errorf("%s/snap/db is missing — the rebuild did not complete", rebuiltMemberDir)
	}
	if size, _ := strconv.Atoi(strings.TrimSpace(sizeOutput)); size < 100000 {
		return fmt.Errorf("%s/snap/db is only %d bytes — the rebuild did not use the real copy", rebuiltMemberDir, size)
	}

	restoredFrom, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "restore-record",
		"-n", "kube-system", "-o", "jsonpath={.data.restored-from}")
	if err != nil {
		return fmt.Errorf("restore-record ConfigMap not found in kube-system: %w", err)
	}
	if !strings.Contains(strings.TrimSpace(restoredFrom), priorStatePath) {
		return fmt.Errorf("restore-record restored-from is %q, expected %s", strings.TrimSpace(restoredFrom), priorStatePath)
	}

	memberDir, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "restore-record",
		"-n", "kube-system", "-o", "jsonpath={.data.member-dir}")
	if err != nil {
		return fmt.Errorf("reading recorded member-dir: %w", err)
	}
	if !strings.Contains(strings.TrimSpace(memberDir), rebuiltMemberDir) {
		return fmt.Errorf("restore-record member-dir is %q, expected %s", strings.TrimSpace(memberDir), rebuiltMemberDir)
	}

	return nil
}

func (l *ConfigHistoryRecoveryLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the object is gone and the prior-state copy exists",
			Command:     "kubectl get cm ledger-config -n default; docker exec <cluster-name>-control-plane ls -lh /var/lib/etcd/pre-change.db",
		},
		{
			Description: "Inspect the copy before using it",
			Command:     "kubectl -n kube-system exec etcd-<node> -- etcdutl snapshot status /var/lib/etcd/pre-change.db -w table",
			Notes:       "Confirms the file is intact and shows the revision and key count it holds",
		},
		{
			Description: "Rebuild a datastore directory from the copy",
			Command: `kubectl -n kube-system exec etcd-<node> -- etcdutl snapshot restore /var/lib/etcd/pre-change.db \
  --data-dir=/var/lib/etcd/restored`,
			Notes: "Older builds: etcdctl snapshot restore <file> --data-dir=<dir>. The directory must not exist beforehand",
		},
		{
			Description: "Check the rebuild produced a member directory",
			Command:     "docker exec <cluster-name>-control-plane ls -R /var/lib/etcd/restored/member | head -20",
			Notes:       "You should see snap/ and wal/ — in a real recovery you would point etcd's --data-dir at this and restart the static pod",
		},
		{
			Description: "Record the rehearsal",
			Command: `kubectl create configmap restore-record -n kube-system \
  --from-literal=restored-from=/var/lib/etcd/pre-change.db \
  --from-literal=member-dir=/var/lib/etcd/restored/member`,
		},
	}
}
