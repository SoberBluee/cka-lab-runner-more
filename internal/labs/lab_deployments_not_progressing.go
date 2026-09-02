package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&DeploymentsNotProgressingLab{})
}

type DeploymentsNotProgressingLab struct {
	BaseLab
}

const brokenControllerManagerTag = "v1.27.0-lab"

func (l *DeploymentsNotProgressingLab) ID() string {
	return "deployments_not_progressing"
}

func (l *DeploymentsNotProgressingLab) Title() string {
	return "New Deployments Never Create Pods"
}

func (l *DeploymentsNotProgressingLab) Category() Category {
	return CategoryWorkloads
}

func (l *DeploymentsNotProgressingLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *DeploymentsNotProgressingLab) Description() string {
	return `A release went out this morning. Since then every new Deployment in the cluster
sits at 0/2 forever. 'kubectl get deploy -n shop' shows the checkout Deployment exists,
'kubectl get pods -n shop' returns nothing at all, and 'kubectl describe' shows no
scheduling events — as if nobody ever asked for the pods.

Your task: find out why workload objects are not being acted on, repair it, and get the
'checkout' Deployment in namespace 'shop' running its 2 replicas.`
}

func (l *DeploymentsNotProgressingLab) Hints() []string {
	return []string{
		"No ReplicaSet and no events means the object was accepted but nothing reconciled it",
		"kubectl get pods -n kube-system — check the health of every control plane component, not just the API server",
		"kubectl describe pod -n kube-system <component> tells you why a static pod cannot start",
		"Static pod definitions live in /etc/kubernetes/manifests on the control plane node; compare the image tags between components",
		"The kubelet reloads a static pod automatically when you edit its manifest file",
	}
}

func (l *DeploymentsNotProgressingLab) EstimatedTime() int {
	return 30
}

func (l *DeploymentsNotProgressingLab) Tags() []string {
	return []string{"workloads", "control-plane", "static-pods", "troubleshooting"}
}

func (l *DeploymentsNotProgressingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *DeploymentsNotProgressingLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	script := fmt.Sprintf(
		"sed -i 's|image: registry.k8s.io/kube-controller-manager:.*|image: registry.k8s.io/kube-controller-manager:%s|' /etc/kubernetes/manifests/kube-controller-manager.yaml",
		brokenControllerManagerTag)
	if _, err := dockerExec(ctx, node, "sh", "-c", script); err != nil {
		return fmt.Errorf("editing controller manager manifest: %w", err)
	}

	// Wait for the kubelet to pick up the edited manifest before creating the
	// workload, otherwise the old controller manager reconciles it first.
	_ = waitFor(ctx, 90*time.Second, func() error {
		image, err := staticPodImage(ctx, kubeconfigPath, "kube-controller-manager")
		if err != nil {
			return nil
		}
		if strings.Contains(image, brokenControllerManagerTag) {
			return nil
		}
		return fmt.Errorf("controller manager still running the previous image")
	})

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: shop
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
    spec:
      containers:
      - name: checkout
        image: nginx:alpine
        ports:
        - containerPort: 80
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying checkout deployment: %w", err)
	}
	return nil
}

func (l *DeploymentsNotProgressingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(15 * time.Second)

	pods, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "shop",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return fmt.Errorf("listing shop pods: %w", err)
	}
	if len(strings.Fields(pods)) > 0 {
		return fmt.Errorf("checkout pods were created — the controller manager is still reconciling")
	}
	return nil
}

func (l *DeploymentsNotProgressingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	apiserverImage, err := staticPodImage(ctx, kubeconfigPath, "kube-apiserver")
	if err != nil {
		return err
	}
	wantTag := imageTag(apiserverImage)

	// The kubelet needs a moment to notice the edited manifest and recreate the
	// static pod, so give the component time to converge before judging it.
	if err := waitFor(ctx, 45*time.Second, func() error {
		cmImage, err := staticPodImage(ctx, kubeconfigPath, "kube-controller-manager")
		if err != nil {
			return fmt.Errorf("controller manager static pod is not registered yet: %w", err)
		}
		if gotTag := imageTag(cmImage); gotTag != wantTag {
			return fmt.Errorf("controller manager image tag is %q but the API server runs %q", gotTag, wantTag)
		}

		ready, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
			"-l", "component=kube-controller-manager",
			"-o", "jsonpath={.items[0].status.containerStatuses[0].ready}")
		if err != nil || strings.TrimSpace(ready) != "true" {
			return fmt.Errorf("controller manager container is not ready yet")
		}
		return nil
	}); err != nil {
		return err
	}

	return deploymentReady(ctx, kubeconfigPath, "shop", "checkout", 2, 60*time.Second)
}

func (l *DeploymentsNotProgressingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the workload object exists but nothing reconciled it",
			Command:     "kubectl get deploy,rs,pods -n shop",
			Notes:       "A Deployment with no ReplicaSet means the controller manager is not running",
		},
		{
			Description: "Check control plane component health",
			Command:     "kubectl get pods -n kube-system -l tier=control-plane",
			Notes:       "kube-controller-manager will be ImagePullBackOff or missing",
		},
		{
			Description: "Read the failure reason",
			Command:     "kubectl describe pod -n kube-system -l component=kube-controller-manager | tail -20",
			Notes:       "The image tag it is trying to pull does not exist",
		},
		{
			Description: "Find the version the rest of the control plane runs",
			Command:     "docker exec <cluster-name>-control-plane grep image: /etc/kubernetes/manifests/kube-apiserver.yaml",
		},
		{
			Description: "Fix the controller manager manifest to that tag",
			Command:     "docker exec <cluster-name>-control-plane sed -i 's|kube-controller-manager:.*|kube-controller-manager:<correct-tag>|' /etc/kubernetes/manifests/kube-controller-manager.yaml",
			Notes:       "Editing the file is enough — the kubelet reloads static pods on change; no restart command needed",
		},
		{
			Description: "Watch the component recover",
			Command:     "kubectl get pods -n kube-system -l component=kube-controller-manager -w",
		},
		{
			Description: "Confirm the workload reconciles",
			Command:     "kubectl get deploy,pods -n shop",
			Notes:       "The checkout Deployment reaches 2/2 with no further action",
		},
	}
}
