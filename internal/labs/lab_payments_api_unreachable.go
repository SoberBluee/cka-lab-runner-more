package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&PaymentsAPIUnreachableLab{})
}

type PaymentsAPIUnreachableLab struct {
	BaseLab
}

func (l *PaymentsAPIUnreachableLab) ID() string {
	return "payments_api_unreachable"
}

func (l *PaymentsAPIUnreachableLab) Title() string {
	return "Storefront Checkout Calls Time Out"
}

func (l *PaymentsAPIUnreachableLab) Category() Category {
	return CategoryNetworking
}

func (l *PaymentsAPIUnreachableLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *PaymentsAPIUnreachableLab) Description() string {
	return `The storefront cannot complete checkouts. Its calls to
http://payments-api.payments.svc.cluster.local hang until they time out. The pods on both
sides are Running, the Service has endpoints, and DNS resolves the name correctly.

The 'payments' namespace was locked down last sprint and that lockdown must stay: anything
not explicitly permitted stays blocked.

Your task: let the storefront pods reach payments-api on TCP 80, without weakening the
lockdown for anyone else.`
}

func (l *PaymentsAPIUnreachableLab) Hints() []string {
	return []string{
		"Running pods, healthy endpoints and working DNS point at policy, not at the workload",
		"kubectl get networkpolicy -A and kubectl describe networkpolicy -n payments",
		"A policy that selects pods but lists no ingress rules denies all incoming traffic to them",
		"Cross-namespace rules use namespaceSelector; every namespace already has kubernetes.io/metadata.name as a label",
		"Copy the ingress example from the NetworkPolicy page in the docs and adapt the selectors — do not write it from memory",
	}
}

func (l *PaymentsAPIUnreachableLab) EstimatedTime() int {
	return 20
}

func (l *PaymentsAPIUnreachableLab) Tags() []string {
	return []string{"networking", "connectivity", "namespaces", "troubleshooting"}
}

func (l *PaymentsAPIUnreachableLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *PaymentsAPIUnreachableLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: payments
---
apiVersion: v1
kind: Namespace
metadata:
  name: storefront
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
  namespace: payments
spec:
  replicas: 1
  selector:
    matchLabels:
      app: payments-api
  template:
    metadata:
      labels:
        app: payments-api
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
  name: payments-api
  namespace: payments
spec:
  selector:
    app: payments-api
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-ingress
  namespace: payments
spec:
  podSelector: {}
  policyTypes:
  - Ingress
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: storefront
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
        command: ["sh", "-c", "while true; do sleep 30; done"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying payments scenario: %w", err)
	}

	if err := deploymentReady(ctx, kubeconfigPath, "payments", "payments-api", 1, 120*time.Second); err != nil {
		return err
	}
	return deploymentReady(ctx, kubeconfigPath, "storefront", "web", 1, 120*time.Second)
}

func (l *PaymentsAPIUnreachableLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)

	pod, err := podNameByLabel(ctx, kubeconfigPath, "storefront", "app=web")
	if err != nil {
		return err
	}
	if _, err := httpGetFromPod(ctx, kubeconfigPath, "storefront", pod,
		"http://payments-api.payments.svc.cluster.local", 5); err == nil {
		return fmt.Errorf("the storefront can already reach payments-api — the CNI may not be enforcing NetworkPolicies")
	}
	return nil
}

func (l *PaymentsAPIUnreachableLab) Verify(ctx context.Context, kubeconfigPath string) error {
	policies, err := kubectl(ctx, kubeconfigPath, "get", "networkpolicy", "-n", "payments",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return fmt.Errorf("listing policies in payments: %w", err)
	}
	if !strings.Contains(policies, "default-deny-ingress") {
		return fmt.Errorf("the default-deny-ingress policy was deleted — restore it and add an allow rule alongside it")
	}

	pod, err := podNameByLabel(ctx, kubeconfigPath, "storefront", "app=web")
	if err != nil {
		return err
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := httpGetFromPod(ctx, kubeconfigPath, "storefront", pod,
			"http://payments-api.payments.svc.cluster.local", 5)
		if err != nil {
			return fmt.Errorf("storefront still cannot reach payments-api: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response from payments-api: %q", strings.TrimSpace(output))
		}
		return nil
	})
}

func (l *PaymentsAPIUnreachableLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the workload side is healthy",
			Command:     "kubectl get pods,svc,endpoints -n payments",
			Notes:       "Endpoints are populated, so this is not a selector or readiness problem",
		},
		{
			Description: "Reproduce the timeout",
			Command:     "kubectl exec -n storefront deploy/web -- wget -qO- --timeout=3 http://payments-api.payments.svc.cluster.local",
		},
		{
			Description: "Find the policy that is dropping it",
			Command:     "kubectl describe networkpolicy -n payments",
			Notes:       "podSelector: {} with policyTypes: Ingress and no rules = deny all ingress in the namespace",
		},
		{
			Description: "Add an allow rule for the storefront namespace",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-storefront
  namespace: payments
spec:
  podSelector:
    matchLabels:
      app: payments-api
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: storefront
    ports:
    - protocol: TCP
      port: 80
EOF`,
			Notes: "Policies are additive: the deny-all stays, this one carves out the single path that is allowed",
		},
		{
			Description: "Confirm the call succeeds",
			Command:     "kubectl exec -n storefront deploy/web -- wget -qO- --timeout=3 http://payments-api.payments.svc.cluster.local",
		},
	}
}
