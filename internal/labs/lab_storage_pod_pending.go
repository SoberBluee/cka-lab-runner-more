package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&StoragePodPendingLab{})
}

type StoragePodPendingLab struct {
	BaseLab
}

func (l *StoragePodPendingLab) ID() string {
	return "storage_pod_pending"
}

func (l *StoragePodPendingLab) Title() string {
	return "Pod Waiting on Volume"
}

func (l *StoragePodPendingLab) Category() Category {
	return CategoryStorage
}

func (l *StoragePodPendingLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *StoragePodPendingLab) Description() string {
	return `A pod named 'app' in namespace 'vol-lab' is stuck Pending and cannot start.
Storage resources exist in the namespace, but the pod never becomes Ready.

Your task: Fix the configuration so the pod can mount its volume and run.`
}

func (l *StoragePodPendingLab) Hints() []string {
	return []string{
		"Describe the pod and read Events carefully",
		"Check PersistentVolumeClaims in the namespace",
		"Compare the volume claimName in the pod with actual PVC names",
		"A typo in claimName will leave the pod Pending forever",
	}
}

func (l *StoragePodPendingLab) EstimatedTime() int {
	return 15
}

func (l *StoragePodPendingLab) Tags() []string {
	return []string{"storage", "pvc", "pods", "volumes", "troubleshooting"}
}

func (l *StoragePodPendingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *StoragePodPendingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: vol-lab
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: app-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /mnt/app-data
  storageClassName: manual
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  namespace: vol-lab
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: manual
---
apiVersion: v1
kind: Pod
metadata:
  name: app
  namespace: vol-lab
spec:
  containers:
  - name: app
    image: nginx:alpine
    volumeMounts:
    - name: data
      mountPath: /usr/share/nginx/html
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: app-storage
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying broken storage pod scenario: %w", err)
	}
	return nil
}

func (l *StoragePodPendingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	return nil
}

func (l *StoragePodPendingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pod", "app", "-n", "vol-lab",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pod: %w", err)
	}
	if strings.TrimSpace(output) != "Running" {
		return fmt.Errorf("pod is not running yet (status: %s)", output)
	}

	claim, err := kubectl(ctx, kubeconfigPath, "get", "pod", "app", "-n", "vol-lab",
		"-o", "jsonpath={.spec.volumes[0].persistentVolumeClaim.claimName}")
	if err != nil {
		return fmt.Errorf("failed to check pod volume: %w", err)
	}
	if strings.TrimSpace(claim) != "app-data" {
		return fmt.Errorf("pod is not using PVC app-data (got: %s)", claim)
	}
	return nil
}

func (l *StoragePodPendingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the pod status",
			Command:     "kubectl get pod app -n vol-lab",
			Notes:       "It should be Pending",
		},
		{
			Description: "Describe the pod",
			Command:     "kubectl describe pod app -n vol-lab",
			Notes:       "Look for a missing PersistentVolumeClaim error",
		},
		{
			Description: "List PVCs",
			Command:     "kubectl get pvc -n vol-lab",
			Notes:       "The PVC is named app-data",
		},
		{
			Description: "Check the pod volume claimName",
			Command:     "kubectl get pod app -n vol-lab -o yaml | grep -A3 persistentVolumeClaim",
			Notes:       "claimName is app-storage (typo)",
		},
		{
			Description: "Fix the claimName",
			Command:     "kubectl delete pod app -n vol-lab",
			Notes:       "Recreate the pod with claimName: app-data (edit and re-apply, or kubectl replace)",
		},
		{
			Description: "Example fixed pod recreate",
			Command:     "kubectl run unused --dry-run=client -o yaml 2>/dev/null; kubectl apply -f - <<'EOF'\napiVersion: v1\nkind: Pod\nmetadata:\n  name: app\n  namespace: vol-lab\nspec:\n  containers:\n  - name: app\n    image: nginx:alpine\n    volumeMounts:\n    - name: data\n      mountPath: /usr/share/nginx/html\n  volumes:\n  - name: data\n    persistentVolumeClaim:\n      claimName: app-data\nEOF",
		},
		{
			Description: "Verify the pod is Running",
			Command:     "kubectl get pod app -n vol-lab",
		},
	}
}
