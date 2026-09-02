package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&DispatchCallsRefusedLab{})
}

type DispatchCallsRefusedLab struct {
	BaseLab
}

func (l *DispatchCallsRefusedLab) ID() string {
	return "dispatch_calls_refused"
}

func (l *DispatchCallsRefusedLab) Title() string {
	return "Dispatch Calls Refused Although Pods Run"
}

func (l *DispatchCallsRefusedLab) Category() Category {
	return CategoryNetworking
}

func (l *DispatchCallsRefusedLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *DispatchCallsRefusedLab) Description() string {
	return `Callers of http://dispatch-api.dispatch.svc.cluster.local get connection refused.
Both dispatch-api pods show as Running with no restarts and their container logs look
normal. The Service exists, its selector matches the pod labels, and DNS resolves the name.

Your task: find out why the Service has nothing to send traffic to, fix it, and make the
call from the 'probe' pod in the same namespace succeed.`
}

func (l *DispatchCallsRefusedLab) Hints() []string {
	return []string{
		"kubectl get endpoints dispatch-api -n dispatch — a Service with no endpoints refuses connections instantly",
		"Running is not the same as Ready: look at the READY column in kubectl get pods, not just STATUS",
		"Only Ready pods are added to a Service's endpoint list",
		"kubectl describe pod -n dispatch <pod> — the probe failure message names the port it tried",
		"Compare the probe's port with the port the container actually listens on",
	}
}

func (l *DispatchCallsRefusedLab) EstimatedTime() int {
	return 20
}

func (l *DispatchCallsRefusedLab) Tags() []string {
	return []string{"networking", "services", "endpoints", "troubleshooting"}
}

func (l *DispatchCallsRefusedLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *DispatchCallsRefusedLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: dispatch
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dispatch-api
  namespace: dispatch
spec:
  replicas: 2
  selector:
    matchLabels:
      app: dispatch-api
  template:
    metadata:
      labels:
        app: dispatch-api
    spec:
      containers:
      - name: api
        image: nginx:alpine
        ports:
        - containerPort: 80
        readinessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 2
          periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: dispatch-api
  namespace: dispatch
spec:
  selector:
    app: dispatch-api
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: v1
kind: Pod
metadata:
  name: probe
  namespace: dispatch
  labels:
    app: probe
spec:
  containers:
  - name: probe
    image: busybox:1.28
    command: ["sh", "-c", "while true; do sleep 30; done"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying dispatch scenario: %w", err)
	}
	return nil
}

func (l *DispatchCallsRefusedLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(20 * time.Second)

	count, err := endpointAddressCount(ctx, kubeconfigPath, "dispatch", "dispatch-api")
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("dispatch-api already has %d endpoint(s)", count)
	}
	return nil
}

func (l *DispatchCallsRefusedLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if err := waitFor(ctx, 45*time.Second, func() error {
		count, err := endpointAddressCount(ctx, kubeconfigPath, "dispatch", "dispatch-api")
		if err != nil {
			return err
		}
		if count < 2 {
			return fmt.Errorf("dispatch-api has %d ready endpoint(s), expected 2", count)
		}
		return nil
	}); err != nil {
		return err
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := httpGetFromPod(ctx, kubeconfigPath, "dispatch", "probe",
			"http://dispatch-api.dispatch.svc.cluster.local", 5)
		if err != nil {
			return fmt.Errorf("the probe pod still cannot reach dispatch-api: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response from dispatch-api: %q", strings.TrimSpace(output))
		}
		return nil
	})
}

func (l *DispatchCallsRefusedLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Check what the Service can actually route to",
			Command:     "kubectl get endpoints dispatch-api -n dispatch",
			Notes:       "<none> — the Service has no backends, which is why connections are refused rather than timing out",
		},
		{
			Description: "Look at pod readiness, not just phase",
			Command:     "kubectl get pods -n dispatch",
			Notes:       "0/1 READY while STATUS is Running",
		},
		{
			Description: "Read why the pods never became Ready",
			Command:     "kubectl describe pod -n dispatch -l app=dispatch-api | grep -A3 -i readiness",
			Notes:       "The readiness probe is hitting port 8080; nginx listens on 80",
		},
		{
			Description: "Point the probe at the right port",
			Command:     `kubectl patch deployment dispatch-api -n dispatch --type json -p='[{"op":"replace","path":"/spec/template/spec/containers/0/readinessProbe/httpGet/port","value":80}]'`,
		},
		{
			Description: "Confirm endpoints appear",
			Command:     "kubectl get pods,endpoints -n dispatch",
		},
		{
			Description: "Confirm the call succeeds",
			Command:     "kubectl exec -n dispatch probe -- wget -qO- --timeout=3 http://dispatch-api.dispatch.svc.cluster.local | head -3",
		},
	}
}
