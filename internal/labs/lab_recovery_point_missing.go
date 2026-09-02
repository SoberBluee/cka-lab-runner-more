package labs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func init() {
	Register(&RecoveryPointMissingLab{})
}

type RecoveryPointMissingLab struct {
	BaseLab
}

const clusterSnapshotPath = "/var/lib/etcd/snapshot.db"

func (l *RecoveryPointMissingLab) ID() string {
	return "recovery_point_missing"
}

func (l *RecoveryPointMissingLab) Title() string {
	return "No Recovery Point For Cluster State"
}

func (l *RecoveryPointMissingLab) Category() Category {
	return CategoryControlPlane
}

func (l *RecoveryPointMissingLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *RecoveryPointMissingLab) Description() string {
	return `An auditor asked for the cluster's most recent recovery point and nobody could
produce one. Before the next change window this cluster needs a verifiable point-in-time
copy of all cluster state, taken with the cluster's own tooling.

Your task:
  1. Write a point-in-time copy of cluster state to /var/lib/etcd/snapshot.db on the
     control plane node.
  2. Record it in a ConfigMap named 'state-record' in kube-system with two keys:
     snapshot-path (the full path) and revision (the revision number the copy reports).

The copy must be a genuine one taken from the running datastore — a placeholder file
will not pass verification.`
}

func (l *RecoveryPointMissingLab) Hints() []string {
	return []string{
		"The datastore runs as a static pod in kube-system; its client tooling ships inside that pod",
		"kubectl -n kube-system get pod -l component=etcd -o yaml shows the cert paths and the data directory mount",
		"Client calls need --cacert, --cert and --key from /etc/kubernetes/pki/etcd/ plus --endpoints=https://127.0.0.1:2379",
		"Only the data directory is shared with the node's filesystem, so write the file inside /var/lib/etcd",
		"'snapshot status -w json' prints the hash, revision and key count of a saved file",
	}
}

func (l *RecoveryPointMissingLab) EstimatedTime() int {
	return 30
}

func (l *RecoveryPointMissingLab) Tags() []string {
	return []string{"control-plane", "datastore", "recovery", "procedure"}
}

func (l *RecoveryPointMissingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
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
`
	return kubectlApply(ctx, kubeconfigPath, seed)
}

func (l *RecoveryPointMissingLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	if _, err := dockerExec(ctx, node, "rm", "-f", clusterSnapshotPath); err != nil {
		return fmt.Errorf("clearing stale snapshot: %w", err)
	}

	_, _ = kubectl(ctx, kubeconfigPath, "delete", "configmap", "state-record",
		"-n", "kube-system", "--ignore-not-found=true")
	return nil
}

func (l *RecoveryPointMissingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "test", "-f", clusterSnapshotPath); err == nil {
		return fmt.Errorf("a snapshot file already exists at %s", clusterSnapshotPath)
	}
	return nil
}

func (l *RecoveryPointMissingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	sizeOutput, err := dockerExec(ctx, node, "stat", "-c", "%s", clusterSnapshotPath)
	if err != nil {
		return fmt.Errorf("no file at %s on the control plane node", clusterSnapshotPath)
	}
	size, _ := strconv.Atoi(strings.TrimSpace(sizeOutput))
	if size < 100000 {
		return fmt.Errorf("%s is only %d bytes — that is not a real snapshot of this cluster", clusterSnapshotPath, size)
	}

	recordedPath, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "state-record",
		"-n", "kube-system", "-o", "jsonpath={.data.snapshot-path}")
	if err != nil {
		return fmt.Errorf("state-record ConfigMap not found in kube-system: %w", err)
	}
	if !strings.Contains(strings.TrimSpace(recordedPath), clusterSnapshotPath) {
		return fmt.Errorf("state-record snapshot-path is %q, expected %s", strings.TrimSpace(recordedPath), clusterSnapshotPath)
	}

	recordedRevision, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "state-record",
		"-n", "kube-system", "-o", "jsonpath={.data.revision}")
	if err != nil {
		return fmt.Errorf("reading recorded revision: %w", err)
	}
	revision, convErr := strconv.ParseInt(strings.TrimSpace(recordedRevision), 10, 64)
	if convErr != nil || revision <= 0 {
		return fmt.Errorf("state-record revision is %q, expected the revision number from the snapshot status output", strings.TrimSpace(recordedRevision))
	}

	actual, ok := snapshotRevision(ctx, kubeconfigPath, clusterSnapshotPath)
	if ok && actual != revision {
		return fmt.Errorf("snapshot reports revision %d but state-record says %d", actual, revision)
	}

	return nil
}

// snapshotRevision reads the revision out of a saved snapshot using the tooling
// inside the etcd pod. The second return value is false when neither etcdutl nor
// etcdctl could report on the file.
func snapshotRevision(ctx context.Context, kubeconfigPath, path string) (int64, bool) {
	commands := [][]string{
		{"etcdutl", "snapshot", "status", path, "-w", "json"},
		{"etcdctl", "snapshot", "status", path, "-w", "json"},
	}

	for _, command := range commands {
		output, err := etcdExec(ctx, kubeconfigPath, command...)
		if err != nil {
			continue
		}
		start := strings.Index(output, "{")
		end := strings.LastIndex(output, "}")
		if start == -1 || end <= start {
			continue
		}
		var status struct {
			Revision int64 `json:"revision"`
		}
		if err := json.Unmarshal([]byte(output[start:end+1]), &status); err != nil {
			continue
		}
		if status.Revision > 0 {
			return status.Revision, true
		}
	}
	return 0, false
}

func (l *RecoveryPointMissingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Find the datastore pod and its certificate paths",
			Command:     "kubectl -n kube-system get pod -l component=etcd -o yaml | grep -E 'image:|--cert-file|--key-file|--trusted-ca-file|mountPath'",
		},
		{
			Description: "Take the snapshot from inside the datastore pod",
			Command: `kubectl -n kube-system exec etcd-<node> -- etcdctl snapshot save /var/lib/etcd/snapshot.db \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key`,
			Notes: "/var/lib/etcd is the only path shared with the node, so the file lands on the node too",
		},
		{
			Description: "Read the snapshot's revision",
			Command:     "kubectl -n kube-system exec etcd-<node> -- etcdutl snapshot status /var/lib/etcd/snapshot.db -w json",
			Notes:       "Older builds: etcdctl snapshot status <file> -w json (with ETCDCTL_API=3 exported for etcd 3.4)",
		},
		{
			Description: "Confirm the file exists on the node",
			Command:     "docker exec <cluster-name>-control-plane ls -lh /var/lib/etcd/snapshot.db",
		},
		{
			Description: "Record the recovery point",
			Command: `kubectl create configmap state-record -n kube-system \
  --from-literal=snapshot-path=/var/lib/etcd/snapshot.db \
  --from-literal=revision=<revision>`,
		},
	}
}
