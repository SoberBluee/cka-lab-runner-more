package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&AlphaMysqlDownLab{})
}

type AlphaMysqlDownLab struct {
	BaseLab
}

func (l *AlphaMysqlDownLab) ID() string             { return "alpha_mysql_down" }
func (l *AlphaMysqlDownLab) Title() string          { return "Alpha MySQL Down" }
func (l *AlphaMysqlDownLab) Category() Category     { return CategoryStorage }
func (l *AlphaMysqlDownLab) Difficulty() Difficulty { return DifficultyHard }
func (l *AlphaMysqlDownLab) EstimatedTime() int     { return 20 }
func (l *AlphaMysqlDownLab) Hints() []string        { return nil }
func (l *AlphaMysqlDownLab) Tags() []string {
	return []string{"troubleshooting", "storage", "pvc", "mysql"}
}

func (l *AlphaMysqlDownLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace alpha

[Weight: 20%] | Time limit: 12–20 minutes

Context:
A Deployment named alpha-mysql has been deployed in namespace alpha. The Pods are
not running.

Task:
1. Troubleshoot and fix the Deployment so it becomes Ready.
2. The Deployment must use PersistentVolume alpha-pv mounted at /var/lib/mysql.
3. The container must set environment variable MYSQL_ALLOW_EMPTY_PASSWORD=1.

Constraints:
- Do not alter the PersistentVolume alpha-pv (name, capacity, accessModes,
  storageClassName, and hostPath must remain unchanged).
- Work only in context cka-lab and namespace alpha.
`
}

func (l *AlphaMysqlDownLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *AlphaMysqlDownLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	_, _ = dockerExec(ctx, node, "mkdir", "-p", "/mnt/alpha-mysql")
	_, _ = dockerExec(ctx, node, "chmod", "777", "/mnt/alpha-mysql")

	_, _ = kubectl(ctx, kubeconfigPath, "delete", "ns", "alpha", "--ignore-not-found=true", "--wait=false")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pv", "alpha-pv", "--ignore-not-found=true", "--wait=false")
	_ = waitFor(ctx, 60*time.Second, func() error {
		out, err := kubectl(ctx, kubeconfigPath, "get", "ns", "alpha", "--ignore-not-found=true")
		if err != nil {
			return nil
		}
		if strings.Contains(out, "alpha") {
			return fmt.Errorf("namespace alpha still present")
		}
		return nil
	})

	// Primary fault: PVC accessModes incompatible with PV (RWX vs RWO).
	// Deployment already has correct mountPath and env so fixing the claim is enough.
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: alpha
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: alpha-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: manual
  hostPath:
    path: /mnt/alpha-mysql
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mysql-data
  namespace: alpha
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
  storageClassName: manual
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: alpha-mysql
  namespace: alpha
spec:
  replicas: 1
  selector:
    matchLabels:
      app: alpha-mysql
  template:
    metadata:
      labels:
        app: alpha-mysql
    spec:
      containers:
      - name: mysql
        image: mysql:8.0
        env:
        - name: MYSQL_ALLOW_EMPTY_PASSWORD
          value: "1"
        volumeMounts:
        - name: data
          mountPath: /var/lib/mysql
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mysql-data
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating alpha-mysql scenario: %w", err)
	}
	time.Sleep(5 * time.Second)
	return nil
}

func (l *AlphaMysqlDownLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	ready, _ := kubectl(ctx, kubeconfigPath, "get", "deploy", "alpha-mysql", "-n", "alpha",
		"-o", "jsonpath={.status.readyReplicas}")
	if ready != "" && ready != "0" {
		return fmt.Errorf("alpha-mysql already has ready replicas")
	}
	return nil
}

func (l *AlphaMysqlDownLab) Verify(ctx context.Context, kubeconfigPath string) error {
	// PV invariants
	cap, _ := kubectl(ctx, kubeconfigPath, "get", "pv", "alpha-pv", "-o", "jsonpath={.spec.capacity.storage}")
	mode, _ := kubectl(ctx, kubeconfigPath, "get", "pv", "alpha-pv", "-o", "jsonpath={.spec.accessModes[0]}")
	sc, _ := kubectl(ctx, kubeconfigPath, "get", "pv", "alpha-pv", "-o", "jsonpath={.spec.storageClassName}")
	path, _ := kubectl(ctx, kubeconfigPath, "get", "pv", "alpha-pv", "-o", "jsonpath={.spec.hostPath.path}")
	if strings.TrimSpace(cap) != "1Gi" || strings.TrimSpace(mode) != "ReadWriteOnce" ||
		strings.TrimSpace(sc) != "manual" || strings.TrimSpace(path) != "/mnt/alpha-mysql" {
		return fmt.Errorf("alpha-pv was altered — restore the original PersistentVolume")
	}

	boundPV, err := kubectl(ctx, kubeconfigPath, "get", "pvc", "mysql-data", "-n", "alpha",
		"-o", "jsonpath={.spec.volumeName}")
	if err != nil {
		return fmt.Errorf("PVC mysql-data not found: %w", err)
	}
	if strings.TrimSpace(boundPV) != "alpha-pv" {
		return fmt.Errorf("PVC must bind to alpha-pv (got %q)", strings.TrimSpace(boundPV))
	}
	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pvc", "mysql-data", "-n", "alpha",
		"-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(phase) != "Bound" {
		return fmt.Errorf("PVC mysql-data is not Bound")
	}

	mount, _ := kubectl(ctx, kubeconfigPath, "get", "deploy", "alpha-mysql", "-n", "alpha",
		"-o", "jsonpath={.spec.template.spec.containers[0].volumeMounts[0].mountPath}")
	if strings.TrimSpace(mount) != "/var/lib/mysql" {
		return fmt.Errorf("mountPath must be /var/lib/mysql (got %q)", strings.TrimSpace(mount))
	}
	env, _ := kubectl(ctx, kubeconfigPath, "get", "deploy", "alpha-mysql", "-n", "alpha",
		"-o", `jsonpath={.spec.template.spec.containers[0].env[?(@.name=="MYSQL_ALLOW_EMPTY_PASSWORD")].value}`)
	if strings.TrimSpace(env) != "1" {
		return fmt.Errorf("MYSQL_ALLOW_EMPTY_PASSWORD must be 1")
	}

	return waitFor(ctx, 120*time.Second, func() error {
		ready, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "alpha-mysql", "-n", "alpha",
			"-o", "jsonpath={.status.readyReplicas}")
		if err != nil {
			return err
		}
		if strings.TrimSpace(ready) != "1" {
			return fmt.Errorf("alpha-mysql not Ready yet")
		}
		return nil
	})
}

func (l *AlphaMysqlDownLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Inspect why the Pod is Pending",
			Command:     `kubectl get deploy,pod,pvc,pv -n alpha; kubectl describe pod -n alpha -l app=alpha-mysql`,
			Notes: "Docs search: Persistent Volumes; Configure a Pod to Use a PersistentVolume for Storage",
		},
		{
			Description: "Fix the PVC accessModes to match alpha-pv (do not edit the PV)",
			Command: `kubectl delete pvc mysql-data -n alpha
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mysql-data
  namespace: alpha
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: manual
EOF
kubectl get pvc,pv -n alpha
kubectl rollout status deploy/alpha-mysql -n alpha`,
		},
	}
}
