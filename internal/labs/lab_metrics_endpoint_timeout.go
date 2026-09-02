package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&MetricsEndpointTimeoutLab{})
}

type MetricsEndpointTimeoutLab struct {
	BaseLab
}

func (l *MetricsEndpointTimeoutLab) ID() string {
	return "metrics_endpoint_timeout"
}

func (l *MetricsEndpointTimeoutLab) Title() string {
	return "Scraper Times Out On One Target Only"
}

func (l *MetricsEndpointTimeoutLab) Category() Category {
	return CategoryNetworking
}

func (l *MetricsEndpointTimeoutLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *MetricsEndpointTimeoutLab) Description() string {
	return `The scraper pod in namespace 'metrics' collects from several targets. Every target
works except 'web' in the same namespace: those scrapes time out. Both pods are Running and
the web Service has endpoints.

Whoever set this up left a comment saying traffic to web is "restricted to the scraper", and
that restriction has to stay in force — only the scraper may reach it.

Your task: make the scrape of http://web.metrics.svc.cluster.local succeed while keeping the
restriction that limits access to the scraper.`
}

func (l *MetricsEndpointTimeoutLab) Hints() []string {
	return []string{
		"One target failing while others work means the difference is target-specific, not cluster-wide",
		"kubectl describe networkpolicy -n metrics — read the whole rule, including the ports block",
		"An ingress rule permits its 'from' peers only on the ports it lists",
		"Compare the port in the policy with the port the Service and container actually use",
		"Fix the port in the policy rather than deleting the policy",
	}
}

func (l *MetricsEndpointTimeoutLab) EstimatedTime() int {
	return 20
}

func (l *MetricsEndpointTimeoutLab) Tags() []string {
	return []string{"networking", "connectivity", "ports", "troubleshooting"}
}

func (l *MetricsEndpointTimeoutLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *MetricsEndpointTimeoutLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: metrics
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: metrics
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
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: metrics
spec:
  selector:
    app: web
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scraper
  namespace: metrics
spec:
  replicas: 1
  selector:
    matchLabels:
      app: scraper
  template:
    metadata:
      labels:
        app: scraper
    spec:
      containers:
      - name: scraper
        image: busybox:1.28
        command: ["sh", "-c", "while true; do sleep 30; done"]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: web-allow-scraper
  namespace: metrics
spec:
  podSelector:
    matchLabels:
      app: web
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: scraper
    ports:
    - protocol: TCP
      port: 8080
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying metrics scenario: %w", err)
	}

	if err := deploymentReady(ctx, kubeconfigPath, "metrics", "web", 1, 120*time.Second); err != nil {
		return err
	}
	return deploymentReady(ctx, kubeconfigPath, "metrics", "scraper", 1, 120*time.Second)
}

func (l *MetricsEndpointTimeoutLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)

	pod, err := podNameByLabel(ctx, kubeconfigPath, "metrics", "app=scraper")
	if err != nil {
		return err
	}
	if _, err := httpGetFromPod(ctx, kubeconfigPath, "metrics", pod,
		"http://web.metrics.svc.cluster.local", 5); err == nil {
		return fmt.Errorf("the scraper can already reach web — the CNI may not be enforcing NetworkPolicies")
	}
	return nil
}

func (l *MetricsEndpointTimeoutLab) Verify(ctx context.Context, kubeconfigPath string) error {
	selector, err := kubectl(ctx, kubeconfigPath, "get", "networkpolicy", "web-allow-scraper",
		"-n", "metrics", "-o", "jsonpath={.spec.ingress[0].from[0].podSelector.matchLabels.app}")
	if err != nil {
		return fmt.Errorf("the web-allow-scraper policy is gone — keep the restriction and fix its rule instead: %w", err)
	}
	if strings.TrimSpace(selector) != "scraper" {
		return fmt.Errorf("the policy no longer restricts ingress to the scraper (from selector is %q)", strings.TrimSpace(selector))
	}

	ports, err := kubectl(ctx, kubeconfigPath, "get", "networkpolicy", "web-allow-scraper",
		"-n", "metrics", "-o", "jsonpath={.spec.ingress[0].ports[0].port}")
	if err != nil || strings.TrimSpace(ports) == "" {
		return fmt.Errorf("the policy no longer names a port — keep the rule specific and point it at the port web actually serves")
	}

	pod, err := podNameByLabel(ctx, kubeconfigPath, "metrics", "app=scraper")
	if err != nil {
		return err
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := httpGetFromPod(ctx, kubeconfigPath, "metrics", pod,
			"http://web.metrics.svc.cluster.local", 5)
		if err != nil {
			return fmt.Errorf("the scrape still times out: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response from web: %q", strings.TrimSpace(output))
		}
		return nil
	})
}

func (l *MetricsEndpointTimeoutLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Reproduce the failing scrape",
			Command:     "kubectl exec -n metrics deploy/scraper -- wget -qO- --timeout=3 http://web.metrics.svc.cluster.local",
		},
		{
			Description: "Check the target really is listening",
			Command:     "kubectl get svc,endpoints -n metrics web",
			Notes:       "Service port 80, endpoints on container port 80",
		},
		{
			Description: "Read the restriction in full",
			Command:     "kubectl describe networkpolicy web-allow-scraper -n metrics",
			Notes:       "The 'from' peer is right, but the rule only permits TCP 8080",
		},
		{
			Description: "Correct the port in the rule",
			Command:     `kubectl patch networkpolicy web-allow-scraper -n metrics --type json -p='[{"op":"replace","path":"/spec/ingress/0/ports/0/port","value":80}]'`,
		},
		{
			Description: "Confirm the scrape succeeds",
			Command:     "kubectl exec -n metrics deploy/scraper -- wget -qO- --timeout=3 http://web.metrics.svc.cluster.local",
		},
	}
}
