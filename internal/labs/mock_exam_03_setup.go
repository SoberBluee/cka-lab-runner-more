package labs

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const mockExam03TaintKey = "workload"

func mockExam03Preflight(ctx context.Context, kubeconfigPath string) error {
	for _, binary := range []string{"docker", "kubectl"} {
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("%s is required for mock_exam_03", binary)
		}
	}
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "sh", "-c", "test -d /etc/kubernetes"); err != nil {
		return fmt.Errorf("mock_exam_03 requires a Docker-based kind control-plane node: %w", err)
	}
	return nil
}

func setupMockExam03(ctx context.Context, kubeconfigPath string) error {
	if err := mockExam03Preflight(ctx, kubeconfigPath); err != nil {
		return err
	}
	_ = cleanupMockExam03Resources(ctx, kubeconfigPath)

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return fmt.Errorf("creating /opt/CKA: %w", err)
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/practice-node.txt")

	// Ensure a clean hostPath for the static PV
	_, _ = dockerExec(ctx, node, "mkdir", "-p", "/mnt/practice-data")
	_, _ = dockerExec(ctx, node, "chmod", "777", "/mnt/practice-data")

	if err := kubectlApply(ctx, kubeconfigPath, mockExam03BaseResources); err != nil {
		return fmt.Errorf("creating exam resources: %w", err)
	}

	// Wait for broken/pending objects to exist
	_ = waitFor(ctx, 60*time.Second, func() error {
		_, err := kubectl(ctx, kubeconfigPath, "get", "pod", "broken-web", "-n", "practice")
		return err
	})

	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if _, err := kubectl(ctx, kubeconfigPath, "taint", "nodes", n,
			"workload=batch:NoSchedule", "--overwrite"); err != nil {
			return fmt.Errorf("tainting node %s: %w", n, err)
		}
	}

	// Give batch-worker a moment to show Pending
	time.Sleep(5 * time.Second)
	return nil
}

func cleanupMockExam03Resources(ctx context.Context, kubeconfigPath string) error {
	for _, ns := range []string{"practice", "shop"} {
		_, _ = kubectl(ctx, kubeconfigPath, "delete", "ns", ns,
			"--ignore-not-found=true", "--wait=false")
	}
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pv", "practice-pv",
		"--ignore-not-found=true", "--wait=false")

	nodes, _ := nodeNames(ctx, kubeconfigPath)
	for _, n := range nodes {
		_, _ = kubectl(ctx, kubeconfigPath, "taint", "nodes", n, mockExam03TaintKey+"-")
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err == nil {
		_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/practice-node.txt")
	}

	// Wait for namespaces to go away so re-create is clean
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		out, err := kubectl(ctx, kubeconfigPath, "get", "ns",
			"-o", "jsonpath={.items[*].metadata.name}")
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if !strings.Contains(out, "practice") && !strings.Contains(out, "shop") {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}

const mockExam03BaseResources = `apiVersion: v1
kind: Namespace
metadata:
  name: practice
---
apiVersion: v1
kind: Namespace
metadata:
  name: shop
---
apiVersion: v1
kind: Pod
metadata:
  name: broken-web
  namespace: practice
  labels:
    app: broken-web
spec:
  containers:
  - name: web
    image: busybox:1.28
    command: ["sh", "-c", "echo boom; exit 1"]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout
  namespace: shop
spec:
  replicas: 2
  selector:
    matchLabels:
      app: checkout
  template:
    metadata:
      labels:
        app: checkout
        tier: frontend
    spec:
      containers:
      - name: web
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: checkout
  namespace: shop
spec:
  selector:
    app: checkouts
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: practice-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: manual
  hostPath:
    path: /mnt/practice-data
---
apiVersion: v1
kind: Pod
metadata:
  name: data-pod
  namespace: practice
spec:
  containers:
  - name: app
    image: nginx:alpine
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: app-data
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: batch-worker
  namespace: practice
spec:
  replicas: 1
  selector:
    matchLabels:
      app: batch-worker
  template:
    metadata:
      labels:
        app: batch-worker
    spec:
      # Tolerates kind control-plane taints so only the exam taint blocks scheduling.
      tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      containers:
      - name: worker
        image: busybox:1.28
        command: ["sh", "-c", "while true; do sleep 30; done"]
`
