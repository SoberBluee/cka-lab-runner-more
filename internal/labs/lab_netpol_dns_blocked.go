package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&NetPolDNSBlockedLab{})
}

type NetPolDNSBlockedLab struct {
	BaseLab
}

func (l *NetPolDNSBlockedLab) ID() string {
	return "netpol_dns_blocked"
}

func (l *NetPolDNSBlockedLab) Title() string {
	return "DNS Resolution Blocked"
}

func (l *NetPolDNSBlockedLab) Category() Category {
	return CategoryNetworking
}

func (l *NetPolDNSBlockedLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *NetPolDNSBlockedLab) Description() string {
	return `Pods in namespace 'shop' cannot resolve Service DNS names.
A backend Service exists and pods are Running, but wget/nslookup fails with DNS errors.

Your task: Fix network policy configuration so pods can resolve DNS and reach the backend Service.`
}

func (l *NetPolDNSBlockedLab) Hints() []string {
	return []string{
		"Check NetworkPolicies in the shop namespace",
		"Default-deny policies often block DNS egress unless explicitly allowed",
		"CoreDNS runs in kube-system and listens on UDP/TCP port 53",
		"You may need both DNS egress and backend ingress/egress rules",
	}
}

func (l *NetPolDNSBlockedLab) EstimatedTime() int {
	return 25
}

func (l *NetPolDNSBlockedLab) Tags() []string {
	return []string{"networking", "network-policy", "dns", "egress", "troubleshooting"}
}

func (l *NetPolDNSBlockedLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *NetPolDNSBlockedLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: shop
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  namespace: shop
spec:
  replicas: 1
  selector:
    matchLabels:
      app: catalog
  template:
    metadata:
      labels:
        app: catalog
    spec:
      containers:
      - name: catalog
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: catalog
  namespace: shop
spec:
  selector:
    app: catalog
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: client
  namespace: shop
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
      containers:
      - name: client
        image: busybox:1.28
        command: ["sh", "-c", "sleep 3600"]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: shop
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying dns-blocked netpol scenario: %w", err)
	}
	return nil
}

func (l *NetPolDNSBlockedLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(12 * time.Second)
	return nil
}

func (l *NetPolDNSBlockedLab) Verify(ctx context.Context, kubeconfigPath string) error {
	clientPod, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "shop",
		"-l", "app=client", "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil || strings.TrimSpace(clientPod) == "" {
		return fmt.Errorf("client pod not found")
	}

	output, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "shop", strings.TrimSpace(clientPod),
		"--", "wget", "-O-", "--timeout=5", "-q", "http://catalog")
	if err != nil {
		return fmt.Errorf("client cannot reach catalog (DNS or netpol still broken): %w", err)
	}
	if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") && len(output) < 5 {
		return fmt.Errorf("unexpected response from catalog")
	}
	return nil
}

func (l *NetPolDNSBlockedLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm pods are running",
			Command:     "kubectl get pods -n shop",
		},
		{
			Description: "Test DNS / HTTP from client",
			Command:     "kubectl exec -n shop deploy/client -- nslookup catalog; kubectl exec -n shop deploy/client -- wget -O- --timeout=2 http://catalog",
			Notes:       "DNS should fail under default-deny egress",
		},
		{
			Description: "Inspect NetworkPolicies",
			Command:     "kubectl get networkpolicy -n shop -o yaml",
		},
		{
			Description: "Allow DNS egress + access to catalog",
			Command:     "kubectl apply -f - <<'EOF'\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: allow-client-egress\n  namespace: shop\nspec:\n  podSelector:\n    matchLabels:\n      app: client\n  policyTypes:\n  - Egress\n  egress:\n  - to:\n    - namespaceSelector: {}\n    ports:\n    - protocol: UDP\n      port: 53\n    - protocol: TCP\n      port: 53\n  - to:\n    - podSelector:\n        matchLabels:\n          app: catalog\n    ports:\n    - protocol: TCP\n      port: 80\n---\napiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: allow-catalog-ingress\n  namespace: shop\nspec:\n  podSelector:\n    matchLabels:\n      app: catalog\n  policyTypes:\n  - Ingress\n  ingress:\n  - from:\n    - podSelector:\n        matchLabels:\n          app: client\n    ports:\n    - protocol: TCP\n      port: 80\nEOF",
			Notes:       "Alternatively delete default-deny-all for a quicker fix in the exam if allowed by the task wording",
		},
		{
			Description: "Verify connectivity",
			Command:     "kubectl exec -n shop deploy/client -- wget -O- --timeout=3 http://catalog",
		},
	}
}
