package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&NetPolNamespaceIsolationLab{})
}

type NetPolNamespaceIsolationLab struct {
	BaseLab
}

func (l *NetPolNamespaceIsolationLab) ID() string {
	return "netpol_namespace_isolation"
}

func (l *NetPolNamespaceIsolationLab) Title() string {
	return "Cross-Namespace Traffic Blocked"
}

func (l *NetPolNamespaceIsolationLab) Category() Category {
	return CategoryNetworking
}

func (l *NetPolNamespaceIsolationLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *NetPolNamespaceIsolationLab) Description() string {
	return `A client pod in namespace 'frontend-ns' cannot reach a Service in 'backend-ns'.
Both namespaces have Running pods, but cross-namespace HTTP calls time out.

Your task: Fix the NetworkPolicy so frontend-ns can reach the backend Service.`
}

func (l *NetPolNamespaceIsolationLab) Hints() []string {
	return []string{
		"Inspect NetworkPolicies in backend-ns",
		"Cross-namespace rules use namespaceSelector (and often require namespace labels)",
		"Check labels on both namespaces and pods",
		"podSelector alone only matches pods in the same namespace",
	}
}

func (l *NetPolNamespaceIsolationLab) EstimatedTime() int {
	return 25
}

func (l *NetPolNamespaceIsolationLab) Tags() []string {
	return []string{"networking", "network-policy", "namespaces", "selectors", "troubleshooting"}
}

func (l *NetPolNamespaceIsolationLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *NetPolNamespaceIsolationLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: frontend-ns
  labels:
    purpose: frontend
---
apiVersion: v1
kind: Namespace
metadata:
  name: backend-ns
  labels:
    purpose: backend
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: backend-ns
spec:
  replicas: 1
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
      - name: api
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: api
  namespace: backend-ns
spec:
  selector:
    app: api
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: frontend-ns
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: web
        image: busybox:1.28
        command: ["sh", "-c", "sleep 3600"]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend
  namespace: backend-ns
spec:
  podSelector:
    matchLabels:
      app: api
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          purpose: client
    ports:
    - protocol: TCP
      port: 80
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying cross-namespace netpol scenario: %w", err)
	}
	return nil
}

func (l *NetPolNamespaceIsolationLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(12 * time.Second)
	return nil
}

func (l *NetPolNamespaceIsolationLab) Verify(ctx context.Context, kubeconfigPath string) error {
	webPod, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "frontend-ns",
		"-l", "app=web", "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil || strings.TrimSpace(webPod) == "" {
		return fmt.Errorf("frontend web pod not found")
	}

	output, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "frontend-ns", strings.TrimSpace(webPod),
		"--", "wget", "-O-", "--timeout=5", "-q", "http://api.backend-ns.svc.cluster.local")
	if err != nil {
		return fmt.Errorf("frontend cannot reach backend api: %w", err)
	}
	if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") && len(output) < 5 {
		return fmt.Errorf("unexpected response from backend api")
	}
	return nil
}

func (l *NetPolNamespaceIsolationLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Test cross-namespace connectivity",
			Command:     "kubectl exec -n frontend-ns deploy/web -- wget -O- --timeout=2 http://api.backend-ns.svc.cluster.local",
			Notes:       "Should time out",
		},
		{
			Description: "Inspect namespace labels",
			Command:     "kubectl get ns frontend-ns backend-ns --show-labels",
			Notes:       "frontend-ns has purpose=frontend",
		},
		{
			Description: "Inspect the NetworkPolicy",
			Command:     "kubectl describe networkpolicy allow-frontend -n backend-ns",
			Notes:       "Ingress allows namespaceSelector purpose=client (wrong label)",
		},
		{
			Description: "Fix the namespaceSelector",
			Command:     "kubectl patch networkpolicy allow-frontend -n backend-ns --type=json -p '[{\"op\":\"replace\",\"path\":\"/spec/ingress/0/from/0/namespaceSelector/matchLabels/purpose\",\"value\":\"frontend\"}]'",
		},
		{
			Description: "Verify connectivity",
			Command:     "kubectl exec -n frontend-ns deploy/web -- wget -O- --timeout=3 http://api.backend-ns.svc.cluster.local",
		},
	}
}
