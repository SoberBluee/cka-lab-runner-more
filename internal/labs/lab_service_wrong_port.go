package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&ServiceWrongPortLab{})
}

type ServiceWrongPortLab struct {
	BaseLab
}

func (l *ServiceWrongPortLab) ID() string {
	return "service_wrong_port"
}

func (l *ServiceWrongPortLab) Title() string {
	return "Service Port Mismatch"
}

func (l *ServiceWrongPortLab) Category() Category {
	return CategoryNetworking
}

func (l *ServiceWrongPortLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *ServiceWrongPortLab) Description() string {
	return `A Service named 'web' in namespace 'port-lab' has Endpoints, but clients
cannot successfully fetch the application over HTTP.

Your task: Fix the Service configuration so HTTP requests succeed.`
}

func (l *ServiceWrongPortLab) Hints() []string {
	return []string{
		"Confirm the Service has Endpoints",
		"Check which port the container is listening on",
		"Compare Service port and targetPort with the containerPort",
		"A wrong targetPort produces timeouts even when selectors are correct",
	}
}

func (l *ServiceWrongPortLab) EstimatedTime() int {
	return 15
}

func (l *ServiceWrongPortLab) Tags() []string {
	return []string{"networking", "services", "ports", "troubleshooting"}
}

func (l *ServiceWrongPortLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *ServiceWrongPortLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: port-lab
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: port-lab
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
  namespace: port-lab
spec:
  selector:
    app: web
  ports:
  - port: 80
    targetPort: 8080
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying port mismatch scenario: %w", err)
	}
	return nil
}

func (l *ServiceWrongPortLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(10 * time.Second)
	return nil
}

func (l *ServiceWrongPortLab) Verify(ctx context.Context, kubeconfigPath string) error {
	target, err := kubectl(ctx, kubeconfigPath, "get", "svc", "web", "-n", "port-lab",
		"-o", "jsonpath={.spec.ports[0].targetPort}")
	if err != nil {
		return fmt.Errorf("failed to check service targetPort: %w", err)
	}
	if strings.TrimSpace(target) != "80" {
		return fmt.Errorf("service targetPort is %s, expected 80", target)
	}

	output, err := kubectl(ctx, kubeconfigPath, "run", "port-test-client", "-n", "port-lab",
		"--image=busybox:1.28", "--rm", "-i", "--restart=Never", "--",
		"wget", "-O-", "--timeout=5", "-q", "http://web")
	if err != nil {
		return fmt.Errorf("could not reach service web: %w", err)
	}
	if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") && len(output) < 5 {
		return fmt.Errorf("unexpected response from service")
	}
	return nil
}

func (l *ServiceWrongPortLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check Service and Endpoints",
			Command:     "kubectl get svc,endpoints web -n port-lab",
			Notes:       "Endpoints exist, so the selector is fine",
		},
		{
			Description: "Check container port",
			Command:     "kubectl get deploy web -n port-lab -o yaml | grep -A2 containerPort",
			Notes:       "Container listens on 80",
		},
		{
			Description: "Check Service targetPort",
			Command:     "kubectl get svc web -n port-lab -o yaml | grep -A5 ports",
			Notes:       "targetPort is 8080 — mismatch",
		},
		{
			Description: "Fix targetPort",
			Command:     "kubectl patch svc web -n port-lab --type=json -p '[{\"op\":\"replace\",\"path\":\"/spec/ports/0/targetPort\",\"value\":80}]'",
		},
		{
			Description: "Test connectivity",
			Command:     "kubectl run tmp --rm -i --restart=Never -n port-lab --image=busybox:1.28 -- wget -O- --timeout=3 http://web",
		},
	}
}
