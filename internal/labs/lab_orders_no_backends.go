package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&OrdersNoBackendsLab{})
}

// OrdersNoBackendsLab is a troubleshooting lab: Service selector mismatch.
type OrdersNoBackendsLab struct {
	BaseLab
}

func (l *OrdersNoBackendsLab) ID() string { return "orders_no_backends" }

func (l *OrdersNoBackendsLab) Title() string {
	return "Orders Service Has No Backends"
}

func (l *OrdersNoBackendsLab) Category() Category { return CategoryNetworking }

func (l *OrdersNoBackendsLab) Difficulty() Difficulty { return DifficultyMedium }

func (l *OrdersNoBackendsLab) EstimatedTime() int { return 7 }

func (l *OrdersNoBackendsLab) Tags() []string {
	return []string{"troubleshooting", "services", "endpoints"}
}

func (l *OrdersNoBackendsLab) Hints() []string { return nil }

func (l *OrdersNoBackendsLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace commerce

[Weight: 6%] | Time limit: 5–7 minutes

Context:
Clients calling Service orders in namespace commerce receive connection refused.
The orders application pods appear to be Running.

Task:
1. Restore Service orders so it has ready backends and answers HTTP on port 80.
2. Write the ready endpoint IP addresses for Service orders to /opt/CKA/orders-eps.txt
   (space-separated, no extra text). Use:
   kubectl -n commerce get endpointslices -l kubernetes.io/service-name=orders -o jsonpath='{.items[*].endpoints[?(@.conditions.ready==true)].addresses[*]}'
   (or equivalent that lists the ready addresses).
3. Delete any temporary debug Pods you created in namespace commerce.

Constraints:
- Work only in context cka-lab and namespace commerce (wrong placement = 0).
- Keep Service name orders and Service port 80 unchanged.
- Do not change the orders Deployment container image or replica count.
- Resource names, labels, and field values must match exactly as specified (case-sensitive).
- Do not leave temporary debug pods, Jobs, or test resources behind.`
}

func (l *OrdersNoBackendsLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *OrdersNoBackendsLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return fmt.Errorf("creating /opt/CKA: %w", err)
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/orders-eps.txt")

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: commerce
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders
  namespace: commerce
spec:
  replicas: 2
  selector:
    matchLabels:
      app: orders
  template:
    metadata:
      labels:
        app: orders
        tier: backend
    spec:
      containers:
      - name: orders
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: orders
  namespace: commerce
spec:
  selector:
    app: order
  ports:
  - port: 80
    targetPort: 80
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying orders scenario: %w", err)
	}
	return deploymentReady(ctx, kubeconfigPath, "commerce", "orders", 2, 120*time.Second)
}

func (l *OrdersNoBackendsLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)
	count, err := endpointAddressCount(ctx, kubeconfigPath, "commerce", "orders")
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("orders Service already has %d endpoint(s)", count)
	}
	return nil
}

func (l *OrdersNoBackendsLab) Verify(ctx context.Context, kubeconfigPath string) error {
	port, err := kubectl(ctx, kubeconfigPath, "get", "svc", "orders", "-n", "commerce",
		"-o", "jsonpath={.spec.ports[0].port}")
	if err != nil {
		return fmt.Errorf("Service orders missing: %w", err)
	}
	if strings.TrimSpace(port) != "80" {
		return fmt.Errorf("Service port must remain 80 (got %q)", strings.TrimSpace(port))
	}

	selector, err := kubectl(ctx, kubeconfigPath, "get", "svc", "orders", "-n", "commerce",
		"-o", "jsonpath={.spec.selector.app}")
	if err != nil {
		return fmt.Errorf("reading Service selector: %w", err)
	}
	if strings.TrimSpace(selector) != "orders" {
		return fmt.Errorf("Service selector app must be %q (got %q)", "orders", strings.TrimSpace(selector))
	}

	replicas, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "orders", "-n", "commerce",
		"-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return fmt.Errorf("reading Deployment replicas: %w", err)
	}
	if strings.TrimSpace(replicas) != "2" {
		return fmt.Errorf("orders Deployment replicas must remain 2 (got %q)", strings.TrimSpace(replicas))
	}

	image, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "orders", "-n", "commerce",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	if err != nil {
		return fmt.Errorf("reading Deployment image: %w", err)
	}
	if strings.TrimSpace(image) != "nginx:alpine" {
		return fmt.Errorf("orders image must remain nginx:alpine (got %q)", strings.TrimSpace(image))
	}

	if err := waitFor(ctx, 45*time.Second, func() error {
		count, err := endpointAddressCount(ctx, kubeconfigPath, "commerce", "orders")
		if err != nil {
			return err
		}
		if count < 2 {
			return fmt.Errorf("orders Service has %d ready endpoint(s), want 2", count)
		}
		return nil
	}); err != nil {
		return err
	}

	if err := assertNoDebugPods(ctx, kubeconfigPath, "commerce"); err != nil {
		return err
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	content, err := dockerExec(ctx, node, "cat", "/opt/CKA/orders-eps.txt")
	if err != nil {
		return fmt.Errorf("/opt/CKA/orders-eps.txt missing or unreadable: %w", err)
	}
	fields := strings.Fields(strings.TrimSpace(content))
	if len(fields) < 2 {
		return fmt.Errorf("/opt/CKA/orders-eps.txt must list at least 2 ready endpoint IPs (got %q)", strings.TrimSpace(content))
	}

	return waitFor(ctx, 30*time.Second, func() error {
		output, err := kubectl(ctx, kubeconfigPath, "run", "orders-verify-client", "-n", "commerce",
			"--image=busybox:1.28", "--rm", "-i", "--restart=Never", "--",
			"wget", "-qO-", "--timeout=5", "http://orders")
		if err != nil {
			return fmt.Errorf("HTTP to Service orders failed: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response from orders Service")
		}
		return nil
	})
}

func (l *OrdersNoBackendsLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm empty backends",
			Command:     "kubectl get svc,endpointslices -n commerce",
		},
		{
			Description: "Compare Service selector to Pod labels",
			Command:     "kubectl get svc orders -n commerce -o jsonpath='{.spec.selector}{\"\\n\"}'; kubectl get pods -n commerce --show-labels",
		},
		{
			Description: "Fix the Service selector imperatively",
			Command:     `kubectl patch svc orders -n commerce -p '{"spec":{"selector":{"app":"orders"}}}'`,
		},
		{
			Description: "Record ready endpoint addresses",
			Command: `kubectl -n commerce get endpointslices -l kubernetes.io/service-name=orders \
  -o jsonpath='{.items[*].endpoints[?(@.conditions.ready==true)].addresses[*]}' \
  | tee /tmp/orders-eps.txt
# Exam: write on the control plane under /opt/CKA. kind equivalent:
docker exec cka-lab-control-plane bash -c 'cat > /opt/CKA/orders-eps.txt' < /tmp/orders-eps.txt`,
			Notes: "Exam uses ssh to the control-plane node then sudo -i; this lab environment equivalent is docker exec -it <node> bash",
		},
		{
			Description: "Verify HTTP and clean debug pods",
			Command:     "kubectl run tmp --rm -it --restart=Never -n commerce --image=busybox:1.28 -- wget -qO- --timeout=3 http://orders; kubectl delete pod tmp -n commerce --ignore-not-found",
		},
	}
}
