package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&AppCannotResolveServicesLab{})
}

type AppCannotResolveServicesLab struct {
	BaseLab
}

func (l *AppCannotResolveServicesLab) ID() string {
	return "app_cannot_resolve_services"
}

func (l *AppCannotResolveServicesLab) Title() string {
	return "Application Cannot Resolve Services"
}

func (l *AppCannotResolveServicesLab) Category() Category {
	return CategoryDNS
}

func (l *AppCannotResolveServicesLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *AppCannotResolveServicesLab) Description() string {
	return `The 'client' Deployment in namespace 'webstore' cannot reach the 'backend'
Service by name. Pods in other namespaces resolve Service names without any problem,
and the backend pods themselves are healthy.

Your task: Make the client pod resolve backend.webstore.svc.cluster.local again.`
}

func (l *AppCannotResolveServicesLab) Hints() []string {
	return []string{
		"Confirm the scope first: does a brand new test pod resolve the same name?",
		"kubectl exec into the client pod and read /etc/resolv.conf",
		"Compare that file with the one in a pod that works — the nameserver differs",
		"A pod only uses cluster DNS when its spec asks for it; check the pod spec fields that control resolver behaviour",
	}
}

func (l *AppCannotResolveServicesLab) EstimatedTime() int {
	return 20
}

func (l *AppCannotResolveServicesLab) Tags() []string {
	return []string{"dns", "dnspolicy", "pods", "troubleshooting"}
}

func (l *AppCannotResolveServicesLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *AppCannotResolveServicesLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: webstore
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend
  namespace: webstore
spec:
  replicas: 1
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      containers:
      - name: backend
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: backend
  namespace: webstore
spec:
  selector:
    app: backend
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: client
  namespace: webstore
spec:
  replicas: 1
  selector:
    matchLabels:
      app: client
  template:
    metadata:
      labels:
        app: client
    spec:
      dnsPolicy: Default
      containers:
      - name: client
        image: busybox:1.28
        command: ["sh", "-c", "sleep 3600"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying webstore scenario: %w", err)
	}
	return nil
}

func (l *AppCannotResolveServicesLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	pod, err := waitForReadyPod(ctx, kubeconfigPath, "webstore", "app=client")
	if err != nil {
		return err
	}

	output, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "webstore", pod,
		"--", "nslookup", "backend.webstore.svc.cluster.local")
	if err == nil && dnsLookupAnswered(output) {
		return fmt.Errorf("client pod already resolves the backend Service")
	}
	return nil
}

func (l *AppCannotResolveServicesLab) Verify(ctx context.Context, kubeconfigPath string) error {
	pod, err := waitForReadyPod(ctx, kubeconfigPath, "webstore", "app=client")
	if err != nil {
		return err
	}

	policy, err := kubectl(ctx, kubeconfigPath, "get", "pod", pod, "-n", "webstore",
		"-o", "jsonpath={.spec.dnsPolicy}")
	if err != nil {
		return fmt.Errorf("failed to read pod dnsPolicy: %w", err)
	}
	if strings.TrimSpace(policy) != "ClusterFirst" {
		return fmt.Errorf("client pod dnsPolicy is %q, expected ClusterFirst", strings.TrimSpace(policy))
	}

	return waitFor(ctx, 60*time.Second, func() error {
		output, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "webstore", pod,
			"--", "nslookup", "backend.webstore.svc.cluster.local")
		if err != nil {
			return fmt.Errorf("client pod still cannot resolve the backend Service: %w", err)
		}
		if !dnsLookupAnswered(output) {
			return fmt.Errorf("lookup returned no answer: %s", output)
		}
		return nil
	})
}

func (l *AppCannotResolveServicesLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Reproduce the failure inside the client pod",
			Command:     "kubectl exec -n webstore deploy/client -- nslookup backend.webstore.svc.cluster.local",
		},
		{
			Description: "Prove cluster DNS itself is healthy",
			Command:     "kubectl run tmp --rm -i --restart=Never --image=busybox:1.28 -- nslookup backend.webstore.svc.cluster.local",
			Notes:       "This works, so the problem is the client pod, not CoreDNS",
		},
		{
			Description: "Compare the resolver configuration",
			Command:     "kubectl exec -n webstore deploy/client -- cat /etc/resolv.conf",
			Notes:       "The nameserver is the node resolver, not the kube-dns ClusterIP, and the cluster search domains are missing",
		},
		{
			Description: "Find the cause in the pod spec",
			Command:     "kubectl get deploy client -n webstore -o yaml | grep dnsPolicy",
			Notes:       "dnsPolicy: Default means 'inherit the node resolver', which is not the same as the Kubernetes default",
		},
		{
			Description: "Switch the Deployment to cluster DNS",
			Command:     "kubectl patch deployment client -n webstore -p '{\"spec\":{\"template\":{\"spec\":{\"dnsPolicy\":\"ClusterFirst\"}}}}'",
			Notes:       "This rolls out a new pod; the old one keeps the old resolv.conf",
		},
		{
			Description: "Verify resolution from the new pod",
			Command:     "kubectl exec -n webstore deploy/client -- nslookup backend.webstore.svc.cluster.local",
		},
	}
}
