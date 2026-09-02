package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&DatabaseUnavailableLab{})
}

type DatabaseUnavailableLab struct {
	BaseLab
}

func (l *DatabaseUnavailableLab) ID() string {
	return "database_unavailable"
}

func (l *DatabaseUnavailableLab) Title() string {
	return "Database Pod Unavailable"
}

func (l *DatabaseUnavailableLab) Category() Category {
	return CategoryStorage
}

func (l *DatabaseUnavailableLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *DatabaseUnavailableLab) Description() string {
	return `A Pod named 'postgres' in namespace 'finance' is stuck in Pending.
The application team says the database never comes up.

Your task: Get the postgres pod to Running.`
}

func (l *DatabaseUnavailableLab) Hints() []string {
	return []string{
		"Describe the Pending pod and read Events",
		"Inspect related PersistentVolumeClaim and PersistentVolume objects",
		"Access modes on the claim and volume must be compatible",
		"Recreate the claim if a field cannot be patched in place",
	}
}

func (l *DatabaseUnavailableLab) EstimatedTime() int {
	return 20
}

func (l *DatabaseUnavailableLab) Tags() []string {
	return []string{"pv", "pvc", "accessmodes", "pending", "troubleshooting"}
}

func (l *DatabaseUnavailableLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *DatabaseUnavailableLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: finance
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: finance-db-pv
spec:
  capacity:
    storage: 2Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /mnt/finance-db
  storageClassName: manual
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  namespace: finance
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
  storageClassName: manual
---
apiVersion: v1
kind: Pod
metadata:
  name: postgres
  namespace: finance
spec:
  containers:
  - name: postgres
    image: nginx:alpine
    volumeMounts:
    - name: data
      mountPath: /var/lib/postgresql/data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: postgres-data
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying database scenario: %w", err)
	}
	return nil
}

func (l *DatabaseUnavailableLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "postgres", "-n", "finance",
		"-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(phase) == "Pending" {
		return nil
	}
	return fmt.Errorf("expected postgres Pending, got %q", phase)
}

func (l *DatabaseUnavailableLab) Verify(ctx context.Context, kubeconfigPath string) error {
	pvcPhase, err := kubectl(ctx, kubeconfigPath, "get", "pvc", "postgres-data", "-n", "finance",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check PVC: %w", err)
	}
	if strings.TrimSpace(pvcPhase) != "Bound" {
		return fmt.Errorf("PVC not Bound yet (status: %s)", pvcPhase)
	}

	phase, err := kubectl(ctx, kubeconfigPath, "get", "pod", "postgres", "-n", "finance",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pod: %w", err)
	}
	if strings.TrimSpace(phase) != "Running" {
		return fmt.Errorf("postgres pod not Running yet (status: %s)", phase)
	}
	return nil
}

func (l *DatabaseUnavailableLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the pod",
			Command:     "kubectl get pod postgres -n finance; kubectl describe pod postgres -n finance | tail -20",
		},
		{
			Description: "Compare PVC and PV access modes",
			Command:     "kubectl get pvc postgres-data -n finance -o yaml | grep -A3 accessModes; kubectl get pv finance-db-pv -o yaml | grep -A3 accessModes",
			Notes:       "PVC asks for ReadWriteMany but PV only offers ReadWriteOnce",
		},
		{
			Description: "Recreate the PVC with ReadWriteOnce",
			Command:     "kubectl delete pod postgres -n finance; kubectl delete pvc postgres-data -n finance",
			Notes:       "Then recreate PVC with accessModes: [ReadWriteOnce] and recreate the pod",
		},
		{
			Description: "Apply fixed PVC + pod",
			Command:     "kubectl apply -f - <<'EOF'\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: postgres-data\n  namespace: finance\nspec:\n  accessModes: [\"ReadWriteOnce\"]\n  resources:\n    requests:\n      storage: 1Gi\n  storageClassName: manual\n---\napiVersion: v1\nkind: Pod\nmetadata:\n  name: postgres\n  namespace: finance\nspec:\n  containers:\n  - name: postgres\n    image: nginx:alpine\n    volumeMounts:\n    - name: data\n      mountPath: /var/lib/postgresql/data\n  volumes:\n  - name: data\n    persistentVolumeClaim:\n      claimName: postgres-data\nEOF",
		},
		{
			Description: "Verify",
			Command:     "kubectl get pvc,pod -n finance",
		},
	}
}
