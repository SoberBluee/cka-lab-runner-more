package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&CacheServiceDownLab{})
}

type CacheServiceDownLab struct {
	BaseLab
}

func (l *CacheServiceDownLab) ID() string {
	return "cache_service_down"
}

func (l *CacheServiceDownLab) Title() string {
	return "Cache Service Down"
}

func (l *CacheServiceDownLab) Category() Category {
	return CategoryStorage
}

func (l *CacheServiceDownLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *CacheServiceDownLab) Description() string {
	return `Deployment 'redis' in namespace 'caching' never becomes Ready.
Its pods stay Pending.

Your task: Fix the issue so redis pods are Running.`
}

func (l *CacheServiceDownLab) Hints() []string {
	return []string{
		"Describe a Pending pod and inspect Events",
		"Check whether any claim the pod depends on is Bound",
		"Compare requested capacity with what is available",
		"A claim that asks for more capacity than any matching volume will stay Pending",
	}
}

func (l *CacheServiceDownLab) EstimatedTime() int {
	return 20
}

func (l *CacheServiceDownLab) Tags() []string {
	return []string{"pv", "pvc", "capacity", "pending", "troubleshooting"}
}

func (l *CacheServiceDownLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *CacheServiceDownLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: caching
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: caching-redis-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /mnt/caching-redis
  storageClassName: manual
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-data
  namespace: caching
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: manual
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: caching
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: nginx:alpine
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: redis-data
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying cache scenario: %w", err)
	}
	return nil
}

func (l *CacheServiceDownLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "caching", "-l", "app=redis",
		"-o", "jsonpath={.items[*].status.phase}")
	if strings.Contains(phase, "Pending") {
		return nil
	}
	return fmt.Errorf("expected redis pods Pending, got %q", phase)
}

func (l *CacheServiceDownLab) Verify(ctx context.Context, kubeconfigPath string) error {
	pvcPhase, err := kubectl(ctx, kubeconfigPath, "get", "pvc", "redis-data", "-n", "caching",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check PVC: %w", err)
	}
	if strings.TrimSpace(pvcPhase) != "Bound" {
		return fmt.Errorf("PVC not Bound yet (status: %s)", pvcPhase)
	}

	phase, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "caching", "-l", "app=redis",
		"-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pods: %w", err)
	}
	for _, p := range strings.Fields(phase) {
		if p != "Running" {
			return fmt.Errorf("redis pods not all Running (got: %s)", phase)
		}
	}
	if len(strings.Fields(phase)) == 0 {
		return fmt.Errorf("no redis pods found")
	}
	return nil
}

func (l *CacheServiceDownLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check pods and describe",
			Command:     "kubectl get pods -n caching; kubectl describe pod -n caching -l app=redis | tail -25",
		},
		{
			Description: "Compare claim size vs volume capacity",
			Command:     "kubectl get pvc redis-data -n caching -o yaml | grep -A3 requests; kubectl get pv caching-redis-pv -o yaml | grep -A3 capacity",
			Notes:       "PVC requests 10Gi but PV only has 1Gi",
		},
		{
			Description: "Recreate the PVC requesting 1Gi (or less)",
			Command:     "kubectl delete deploy redis -n caching; kubectl delete pvc redis-data -n caching",
		},
		{
			Description: "Apply fixed resources",
			Command:     "kubectl apply -f - <<'EOF'\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: redis-data\n  namespace: caching\nspec:\n  accessModes: [\"ReadWriteOnce\"]\n  resources:\n    requests:\n      storage: 1Gi\n  storageClassName: manual\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: redis\n  namespace: caching\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: redis\n  template:\n    metadata:\n      labels:\n        app: redis\n    spec:\n      containers:\n      - name: redis\n        image: nginx:alpine\n        volumeMounts:\n        - name: data\n          mountPath: /data\n      volumes:\n      - name: data\n        persistentVolumeClaim:\n          claimName: redis-data\nEOF",
		},
		{
			Description: "Verify",
			Command:     "kubectl get pvc,pods -n caching",
		},
	}
}
