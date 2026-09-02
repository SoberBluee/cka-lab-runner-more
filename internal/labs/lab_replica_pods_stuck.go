package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&ReplicaPodsStuckLab{})
}

type ReplicaPodsStuckLab struct {
	BaseLab
}

func (l *ReplicaPodsStuckLab) ID() string {
	return "replica_pods_stuck"
}

func (l *ReplicaPodsStuckLab) Title() string {
	return "Ordered Pods Stuck"
}

func (l *ReplicaPodsStuckLab) Category() Category {
	return CategoryStorage
}

func (l *ReplicaPodsStuckLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *ReplicaPodsStuckLab) Description() string {
	return `A StatefulSet named 'zk' in namespace 'messaging' shows 0 ready replicas.
The first pod never leaves Pending.

Your task: Fix the StatefulSet so at least one pod becomes Running.`
}

func (l *ReplicaPodsStuckLab) Hints() []string {
	return []string{
		"Check the StatefulSet and its pods",
		"StatefulSets often create a PVC per pod via volumeClaimTemplates",
		"Inspect those PVCs — if they stay Pending, the pods will too",
		"Compare the StorageClass requested by the template with what exists in the cluster",
	}
}

func (l *ReplicaPodsStuckLab) EstimatedTime() int {
	return 25
}

func (l *ReplicaPodsStuckLab) Tags() []string {
	return []string{"statefulset", "pvc", "storageclass", "troubleshooting"}
}

func (l *ReplicaPodsStuckLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *ReplicaPodsStuckLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: messaging
---
apiVersion: v1
kind: Service
metadata:
  name: zk-hs
  namespace: messaging
spec:
  clusterIP: None
  selector:
    app: zk
  ports:
  - port: 2888
    name: server
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: zk
  namespace: messaging
spec:
  serviceName: zk-hs
  replicas: 1
  selector:
    matchLabels:
      app: zk
  template:
    metadata:
      labels:
        app: zk
    spec:
      containers:
      - name: zk
        image: nginx:alpine
        ports:
        - containerPort: 80
          name: client
        volumeMounts:
        - name: datadir
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: datadir
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: fast-ssd
      resources:
        requests:
          storage: 1Gi
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying statefulset scenario: %w", err)
	}
	return nil
}

func (l *ReplicaPodsStuckLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "messaging", "-l", "app=zk",
		"-o", "jsonpath={.items[*].status.phase}")
	if strings.Contains(phase, "Pending") || strings.TrimSpace(phase) == "" {
		return nil
	}
	return fmt.Errorf("expected zk pods Pending, got %q", phase)
}

func (l *ReplicaPodsStuckLab) Verify(ctx context.Context, kubeconfigPath string) error {
	phase, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "messaging", "-l", "app=zk",
		"-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pods: %w", err)
	}
	if !strings.Contains(phase, "Running") {
		return fmt.Errorf("zk pods not Running yet (got: %s)", phase)
	}

	pvcPhase, err := kubectl(ctx, kubeconfigPath, "get", "pvc", "-n", "messaging",
		"-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check PVCs: %w", err)
	}
	if !strings.Contains(pvcPhase, "Bound") {
		return fmt.Errorf("expected a Bound PVC in messaging, got %q", pvcPhase)
	}
	return nil
}

func (l *ReplicaPodsStuckLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check StatefulSet and pods",
			Command:     "kubectl get sts,pods -n messaging; kubectl describe pod -n messaging -l app=zk | tail -25",
		},
		{
			Description: "Check PVCs created by the StatefulSet",
			Command:     "kubectl get pvc -n messaging; kubectl describe pvc -n messaging | tail -30",
			Notes:       "datadir-zk-0 stays Pending",
		},
		{
			Description: "Compare StorageClasses",
			Command:     "kubectl get sc; kubectl get sts zk -n messaging -o yaml | grep -A8 volumeClaimTemplates",
			Notes:       "volumeClaimTemplates requests storageClassName: fast-ssd which does not exist",
		},
		{
			Description: "Fix by using an existing StorageClass (or omit it / use manual + matching PV)",
			Command:     "kubectl delete sts zk -n messaging; kubectl delete pvc -n messaging --all",
			Notes:       "Then recreate the StatefulSet with storageClassName set to an existing class (e.g. standard) or create a matching PV + use storageClassName: manual",
		},
		{
			Description: "Example fix with standard StorageClass (kind)",
			Command:     "kubectl apply -f - <<'EOF'\napiVersion: apps/v1\nkind: StatefulSet\nmetadata:\n  name: zk\n  namespace: messaging\nspec:\n  serviceName: zk-hs\n  replicas: 1\n  selector:\n    matchLabels:\n      app: zk\n  template:\n    metadata:\n      labels:\n        app: zk\n    spec:\n      containers:\n      - name: zk\n        image: nginx:alpine\n        volumeMounts:\n        - name: datadir\n          mountPath: /data\n  volumeClaimTemplates:\n  - metadata:\n      name: datadir\n    spec:\n      accessModes: [\"ReadWriteOnce\"]\n      storageClassName: standard\n      resources:\n        requests:\n          storage: 1Gi\nEOF",
		},
		{
			Description: "Verify",
			Command:     "kubectl get sts,pods,pvc -n messaging",
		},
	}
}
