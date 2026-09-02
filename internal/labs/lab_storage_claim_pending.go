package labs

import (
	"context"
	"fmt"
	"time"
)

func init() {
	Register(&StorageClaimPendingLab{})
}

type StorageClaimPendingLab struct {
	BaseLab
}

func (l *StorageClaimPendingLab) ID() string {
	return "storage_claim_pending"
}

func (l *StorageClaimPendingLab) Title() string {
	return "Storage Claim Not Binding"
}

func (l *StorageClaimPendingLab) Category() Category {
	return CategoryStorage
}

func (l *StorageClaimPendingLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *StorageClaimPendingLab) Description() string {
	return `A PersistentVolumeClaim named 'db-data' in namespace 'storage-lab' is stuck Pending.
An application needs this volume before it can start.

Your task: Fix the storage configuration so the claim binds successfully.`
}

func (l *StorageClaimPendingLab) Hints() []string {
	return []string{
		"Describe the PVC and read its Events",
		"Compare the PVC with available PersistentVolumes",
		"Check storageClassName, access modes, and capacity carefully",
		"Both the claim and the volume must agree on the StorageClass name",
	}
}

func (l *StorageClaimPendingLab) EstimatedTime() int {
	return 20
}

func (l *StorageClaimPendingLab) Tags() []string {
	return []string{"storage", "pv", "pvc", "storageclass", "troubleshooting"}
}

func (l *StorageClaimPendingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *StorageClaimPendingLab) Break(ctx context.Context, kubeconfigPath string) error {
	ns := `apiVersion: v1
kind: Namespace
metadata:
  name: storage-lab
`
	if err := kubectlApply(ctx, kubeconfigPath, ns); err != nil {
		return fmt.Errorf("creating namespace: %w", err)
	}

	pv := `apiVersion: v1
kind: PersistentVolume
metadata:
  name: db-pv
spec:
  capacity:
    storage: 2Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /mnt/db-data
  storageClassName: manual
`
	if err := kubectlApply(ctx, kubeconfigPath, pv); err != nil {
		return fmt.Errorf("creating PV: %w", err)
	}

	pvc := `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: db-data
  namespace: storage-lab
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: exam-storage
`
	if err := kubectlApply(ctx, kubeconfigPath, pvc); err != nil {
		return fmt.Errorf("creating PVC: %w", err)
	}

	return nil
}

func (l *StorageClaimPendingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)
	return nil
}

func (l *StorageClaimPendingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "get", "pvc", "db-data", "-n", "storage-lab",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check PVC: %w", err)
	}
	if output != "Bound" {
		return fmt.Errorf("PVC is not bound yet (current status: %s)", output)
	}

	vol, err := kubectl(ctx, kubeconfigPath, "get", "pvc", "db-data", "-n", "storage-lab",
		"-o", "jsonpath={.spec.volumeName}")
	if err != nil {
		return fmt.Errorf("failed to check PVC volume name: %w", err)
	}
	if vol == "" {
		return fmt.Errorf("PVC does not have a bound volume")
	}
	return nil
}

func (l *StorageClaimPendingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check the PVC",
			Command:     "kubectl get pvc -n storage-lab",
			Notes:       "db-data should be Pending",
		},
		{
			Description: "Describe the PVC",
			Command:     "kubectl describe pvc db-data -n storage-lab",
		},
		{
			Description: "Inspect available PVs",
			Command:     "kubectl get pv db-pv -o yaml",
			Notes:       "Note storageClassName: manual",
		},
		{
			Description: "Compare StorageClass names",
			Command:     "kubectl get pvc db-data -n storage-lab -o jsonpath='{.spec.storageClassName}'",
			Notes:       "PVC requests 'exam-storage' but PV offers 'manual'",
		},
		{
			Description: "Fix the PVC storageClassName",
			Command:     "kubectl delete pvc db-data -n storage-lab && kubectl apply -f - <<'EOF'\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: db-data\n  namespace: storage-lab\nspec:\n  accessModes: [\"ReadWriteOnce\"]\n  resources:\n    requests:\n      storage: 1Gi\n  storageClassName: manual\nEOF",
			Notes:       "storageClassName is often immutable; recreate the PVC with storageClassName: manual",
		},
		{
			Description: "Verify the PVC is Bound",
			Command:     "kubectl get pvc db-data -n storage-lab",
		},
	}
}
