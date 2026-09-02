package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&WebrootMissingLab{})
}

type WebrootMissingLab struct {
	BaseLab
}

func (l *WebrootMissingLab) ID() string {
	return "webroot_missing"
}

func (l *WebrootMissingLab) Title() string {
	return "Web Content Not Serving"
}

func (l *WebrootMissingLab) Category() Category {
	return CategoryStorage
}

func (l *WebrootMissingLab) Difficulty() Difficulty {
	return DifficultyEasy
}

func (l *WebrootMissingLab) Description() string {
	return `Pod 'nginx' in namespace 'public-site' never becomes Ready.
It looks like a mount/configuration problem rather than an image pull issue.

Your task: Fix the pod so it reaches Running.`
}

func (l *WebrootMissingLab) Hints() []string {
	return []string{
		"Describe the pod and check Events / container state",
		"Compare volumeMounts names with volumes names in the pod spec",
		"Every volumeMount.name must match a volumes[].name entry",
		"Recreate the pod after fixing the names",
	}
}

func (l *WebrootMissingLab) EstimatedTime() int {
	return 15
}

func (l *WebrootMissingLab) Tags() []string {
	return []string{"volumes", "mounts", "pods", "troubleshooting"}
}

func (l *WebrootMissingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *WebrootMissingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: public-site
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: public-site-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  hostPath:
    path: /mnt/public-site
  storageClassName: manual
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: site-content
  namespace: public-site
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
  name: nginx
  namespace: public-site
spec:
  containers:
  - name: nginx
    image: nginx:alpine
    volumeMounts:
    - name: html
      mountPath: /usr/share/nginx/html
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: site-content
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying webroot scenario: %w", err)
	}
	return nil
}

func (l *WebrootMissingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.status.phase}")
	ready, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.status.containerStatuses[0].ready}")
	if strings.TrimSpace(phase) != "Running" || strings.TrimSpace(ready) != "true" {
		return nil
	}
	return fmt.Errorf("expected nginx not Ready, got phase=%q ready=%q", phase, ready)
}

func (l *WebrootMissingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	phase, err := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to check pod: %w", err)
	}
	if strings.TrimSpace(phase) != "Running" {
		return fmt.Errorf("nginx pod not Running yet (status: %s)", phase)
	}

	mountName, err := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.spec.containers[0].volumeMounts[0].name}")
	if err != nil {
		return fmt.Errorf("failed to check volumeMount: %w", err)
	}
	volName, err := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.spec.volumes[0].name}")
	if err != nil {
		return fmt.Errorf("failed to check volume: %w", err)
	}
	if strings.TrimSpace(mountName) != strings.TrimSpace(volName) {
		return fmt.Errorf("volumeMount name %q does not match volume name %q", mountName, volName)
	}
	return nil
}

func (l *WebrootMissingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check pod status",
			Command:     "kubectl get pod nginx -n public-site; kubectl describe pod nginx -n public-site | tail -30",
			Notes:       "Look for CreateContainerConfigError / cannot find volume \"html\"",
		},
		{
			Description: "Compare mount and volume names",
			Command:     "kubectl get pod nginx -n public-site -o yaml | grep -A6 'volumeMounts\\|volumes:'",
			Notes:       "volumeMounts uses name: html but volumes defines name: data",
		},
		{
			Description: "Recreate the pod with matching names",
			Command:     "kubectl delete pod nginx -n public-site && kubectl apply -f - <<'EOF'\napiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx\n  namespace: public-site\nspec:\n  containers:\n  - name: nginx\n    image: nginx:alpine\n    volumeMounts:\n    - name: data\n      mountPath: /usr/share/nginx/html\n  volumes:\n  - name: data\n    persistentVolumeClaim:\n      claimName: site-content\nEOF",
		},
		{
			Description: "Verify",
			Command:     "kubectl get pod nginx -n public-site",
		},
	}
}
