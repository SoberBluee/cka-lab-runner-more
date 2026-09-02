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
	return `Pod 'nginx' in namespace 'public-site' is Running, but the site content
volume is not mounted where the web server expects it.

Your task: Fix the pod configuration so the volume is mounted at the correct path.`
}

func (l *WebrootMissingLab) Hints() []string {
	return []string{
		"Inspect the pod spec carefully — focus on volumeMounts",
		"nginx serves files from /usr/share/nginx/html by default",
		"Compare mountPath with where the application actually reads files",
		"Pods usually need to be deleted and recreated after fixing mountPath",
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
	// API server rejects volumeMount names that don't match volumes[].name,
	// so break mountPath instead (valid YAML, wrong runtime config).
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
    - name: data
      mountPath: /var/www/html
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
	mountPath, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.spec.containers[0].volumeMounts[0].mountPath}")
	if strings.TrimSpace(mountPath) != "/usr/share/nginx/html" {
		return nil
	}
	return fmt.Errorf("expected broken mountPath, got %q", mountPath)
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

	mountPath, err := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.spec.containers[0].volumeMounts[0].mountPath}")
	if err != nil {
		return fmt.Errorf("failed to check mountPath: %w", err)
	}
	if strings.TrimSpace(mountPath) != "/usr/share/nginx/html" {
		return fmt.Errorf("volume still mounted at %q, expected /usr/share/nginx/html", mountPath)
	}

	mountName, err := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.spec.containers[0].volumeMounts[0].name}")
	if err != nil {
		return fmt.Errorf("failed to check volumeMount name: %w", err)
	}
	volName, err := kubectl(ctx, kubeconfigPath, "get", "pod", "nginx", "-n", "public-site",
		"-o", "jsonpath={.spec.volumes[0].name}")
	if err != nil {
		return fmt.Errorf("failed to check volume name: %w", err)
	}
	if strings.TrimSpace(mountName) != strings.TrimSpace(volName) {
		return fmt.Errorf("volumeMount name %q does not match volume name %q", mountName, volName)
	}
	return nil
}

func (l *WebrootMissingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Inspect the pod mounts",
			Command:     "kubectl get pod nginx -n public-site -o yaml | grep -A6 volumeMounts",
			Notes:       "mountPath is /var/www/html but nginx serves from /usr/share/nginx/html",
		},
		{
			Description: "Recreate the pod with the correct mountPath",
			Command:     "kubectl delete pod nginx -n public-site && kubectl apply -f - <<'EOF'\napiVersion: v1\nkind: Pod\nmetadata:\n  name: nginx\n  namespace: public-site\nspec:\n  containers:\n  - name: nginx\n    image: nginx:alpine\n    volumeMounts:\n    - name: data\n      mountPath: /usr/share/nginx/html\n  volumes:\n  - name: data\n    persistentVolumeClaim:\n      claimName: site-content\nEOF",
		},
		{
			Description: "Verify",
			Command:     "kubectl get pod nginx -n public-site; kubectl get pod nginx -n public-site -o jsonpath='{.spec.containers[0].volumeMounts[0].mountPath}{\"\\n\"}'",
		},
	}
}
