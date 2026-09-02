package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&ServiceUnreachableLab{})
}

type ServiceUnreachableLab struct {
	BaseLab
}

func (l *ServiceUnreachableLab) ID() string {
	return "service_unreachable"
}

func (l *ServiceUnreachableLab) Title() string {
	return "Service Unreachable"
}

func (l *ServiceUnreachableLab) Category() Category {
	return CategoryNetworking
}

func (l *ServiceUnreachableLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *ServiceUnreachableLab) Description() string {
	return `A Service named 'api' in namespace 'svc-lab' does not reach its backend pods.
Clients timing out when calling the Service DNS name.

Your task: Fix the Service so traffic reaches the application pods.`
}

func (l *ServiceUnreachableLab) Hints() []string {
	return []string{
		"Check Endpoints or EndpointSlices for the Service",
		"Compare Service selectors with pod labels",
		"A selector mismatch leaves the Service with no backends",
		"Test with kubectl get endpoints -n svc-lab",
	}
}

func (l *ServiceUnreachableLab) EstimatedTime() int {
	return 15
}

func (l *ServiceUnreachableLab) Tags() []string {
	return []string{"networking", "services", "selectors", "endpoints", "troubleshooting"}
}

func (l *ServiceUnreachableLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *ServiceUnreachableLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: svc-lab
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: svc-lab
spec:
  replicas: 2
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
        tier: backend
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
  namespace: svc-lab
spec:
  selector:
    app: web
  ports:
  - port: 80
    targetPort: 80
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying service scenario: %w", err)
	}
	return nil
}

func (l *ServiceUnreachableLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)
	return nil
}

func (l *ServiceUnreachableLab) Verify(ctx context.Context, kubeconfigPath string) error {
	eps, err := kubectl(ctx, kubeconfigPath, "get", "endpoints", "api", "-n", "svc-lab",
		"-o", "jsonpath={.subsets[*].addresses[*].ip}")
	if err != nil {
		return fmt.Errorf("failed to check endpoints: %w", err)
	}
	if strings.TrimSpace(eps) == "" {
		return fmt.Errorf("service still has no endpoints")
	}

	selector, err := kubectl(ctx, kubeconfigPath, "get", "svc", "api", "-n", "svc-lab",
		"-o", "jsonpath={.spec.selector.app}")
	if err != nil {
		return fmt.Errorf("failed to check service selector: %w", err)
	}
	if strings.TrimSpace(selector) != "api" {
		return fmt.Errorf("service selector app=%s does not match pods", selector)
	}

	pod, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "svc-lab",
		"-l", "app=api", "-o", "jsonpath={.items[0].metadata.name}")
	if err != nil || strings.TrimSpace(pod) == "" {
		return fmt.Errorf("api pod not found")
	}

	output, err := kubectl(ctx, kubeconfigPath, "run", "svc-test-client", "-n", "svc-lab",
		"--image=busybox:1.28", "--rm", "-i", "--restart=Never", "--",
		"wget", "-O-", "--timeout=5", "-q", "http://api")
	if err != nil {
		return fmt.Errorf("could not reach service api: %w", err)
	}
	if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") && len(output) < 5 {
		return fmt.Errorf("unexpected response from service")
	}
	return nil
}

func (l *ServiceUnreachableLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check Service and Endpoints",
			Command:     "kubectl get svc,endpoints -n svc-lab",
			Notes:       "Endpoints for api should be empty",
		},
		{
			Description: "Compare labels and selectors",
			Command:     "kubectl get pods -n svc-lab --show-labels; kubectl get svc api -n svc-lab -o yaml | grep -A3 selector",
			Notes:       "Pods have app=api but Service selects app=web",
		},
		{
			Description: "Fix the Service selector",
			Command:     "kubectl patch svc api -n svc-lab -p '{\"spec\":{\"selector\":{\"app\":\"api\"}}}'",
		},
		{
			Description: "Verify endpoints are populated",
			Command:     "kubectl get endpoints api -n svc-lab",
		},
		{
			Description: "Test connectivity",
			Command:     "kubectl run tmp --rm -i --restart=Never -n svc-lab --image=busybox:1.28 -- wget -O- --timeout=3 http://api",
		},
	}
}
