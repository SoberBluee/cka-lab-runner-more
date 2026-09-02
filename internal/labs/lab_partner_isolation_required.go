package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&PartnerIsolationRequiredLab{})
}

type PartnerIsolationRequiredLab struct {
	BaseLab
}

func (l *PartnerIsolationRequiredLab) ID() string {
	return "partner_isolation_required"
}

func (l *PartnerIsolationRequiredLab) Title() string {
	return "Orders API Is Open To Every Namespace"
}

func (l *PartnerIsolationRequiredLab) Category() Category {
	return CategoryNetworking
}

func (l *PartnerIsolationRequiredLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *PartnerIsolationRequiredLab) Description() string {
	return `An audit found that the 'orders' workload in namespace 'api' accepts connections
from anywhere in the cluster. Two partner namespaces call it today: 'trusted', which is
under contract, and 'guest', which is not and should never have had access.

Your task: restrict incoming traffic to the orders pods so that:
  - clients in the 'trusted' namespace can still reach them on TCP 80
  - clients in the 'guest' namespace are blocked
  - nothing else in the cluster can reach them either

Both partner namespaces already run a client pod you can test from.`
}

func (l *PartnerIsolationRequiredLab) Hints() []string {
	return []string{
		"Nothing is broken yet — you are writing the restriction, so start from the docs example rather than memory",
		"kubectl get ns --show-labels — namespaceSelector matches on namespace labels, and metadata.name is always there",
		"A policy that selects the orders pods and lists one ingress rule denies every peer the rule does not name",
		"Test both directions after applying: the allowed client must still work and the blocked one must time out",
		"If both clients still succeed after a correct policy, your CNI is not enforcing NetworkPolicies",
	}
}

func (l *PartnerIsolationRequiredLab) EstimatedTime() int {
	return 25
}

func (l *PartnerIsolationRequiredLab) Tags() []string {
	return []string{"networking", "isolation", "namespaces", "authoring"}
}

func (l *PartnerIsolationRequiredLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *PartnerIsolationRequiredLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: api
---
apiVersion: v1
kind: Namespace
metadata:
  name: trusted
  labels:
    partner: contracted
---
apiVersion: v1
kind: Namespace
metadata:
  name: guest
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders
  namespace: api
spec:
  replicas: 1
  selector:
    matchLabels:
      app: orders
  template:
    metadata:
      labels:
        app: orders
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
  namespace: api
spec:
  selector:
    app: orders
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: client
  namespace: trusted
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
        command: ["sh", "-c", "while true; do sleep 30; done"]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: client
  namespace: guest
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
        command: ["sh", "-c", "while true; do sleep 30; done"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying partner isolation scenario: %w", err)
	}

	if err := deploymentReady(ctx, kubeconfigPath, "api", "orders", 1, 120*time.Second); err != nil {
		return err
	}
	if err := deploymentReady(ctx, kubeconfigPath, "trusted", "client", 1, 120*time.Second); err != nil {
		return err
	}
	return deploymentReady(ctx, kubeconfigPath, "guest", "client", 1, 120*time.Second)
}

func (l *PartnerIsolationRequiredLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)

	pod, err := podNameByLabel(ctx, kubeconfigPath, "guest", "app=client")
	if err != nil {
		return err
	}
	if _, err := httpGetFromPod(ctx, kubeconfigPath, "guest", pod,
		"http://orders.api.svc.cluster.local", 5); err != nil {
		return fmt.Errorf("the guest namespace cannot reach orders even before the exercise starts: %w", err)
	}
	return nil
}

func (l *PartnerIsolationRequiredLab) Verify(ctx context.Context, kubeconfigPath string) error {
	trustedPod, err := podNameByLabel(ctx, kubeconfigPath, "trusted", "app=client")
	if err != nil {
		return err
	}
	guestPod, err := podNameByLabel(ctx, kubeconfigPath, "guest", "app=client")
	if err != nil {
		return err
	}

	policies, err := kubectl(ctx, kubeconfigPath, "get", "networkpolicy", "-n", "api",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return fmt.Errorf("listing policies in api: %w", err)
	}
	if len(strings.Fields(policies)) == 0 {
		return fmt.Errorf("no NetworkPolicy exists in the api namespace yet")
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := httpGetFromPod(ctx, kubeconfigPath, "trusted", trustedPod,
			"http://orders.api.svc.cluster.local", 5)
		if err != nil {
			return fmt.Errorf("the trusted client can no longer reach orders: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response for the trusted client: %q", strings.TrimSpace(output))
		}

		if _, err := httpGetFromPod(ctx, kubeconfigPath, "guest", guestPod,
			"http://orders.api.svc.cluster.local", 5); err == nil {
			return fmt.Errorf("the guest client can still reach orders")
		}
		return nil
	})
}

func (l *PartnerIsolationRequiredLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the current wide-open behaviour",
			Command: `kubectl exec -n trusted deploy/client -- wget -qO- --timeout=3 http://orders.api.svc.cluster.local | head -3
kubectl exec -n guest deploy/client -- wget -qO- --timeout=3 http://orders.api.svc.cluster.local | head -3`,
			Notes: "Both succeed today",
		},
		{
			Description: "Look at the labels you can select on",
			Command:     "kubectl get ns trusted guest --show-labels",
			Notes:       "trusted carries partner=contracted; both carry kubernetes.io/metadata.name",
		},
		{
			Description: "Write the restriction",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: orders-allow-trusted
  namespace: api
spec:
  podSelector:
    matchLabels:
      app: orders
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          partner: contracted
    ports:
    - protocol: TCP
      port: 80
EOF`,
			Notes: "Selecting the orders pods at all makes every unnamed peer denied — you do not need a separate deny rule",
		},
		{
			Description: "Prove both halves of the requirement",
			Command: `kubectl exec -n trusted deploy/client -- wget -qO- --timeout=3 http://orders.api.svc.cluster.local | head -3
kubectl exec -n guest deploy/client -- wget -qO- --timeout=3 http://orders.api.svc.cluster.local`,
			Notes: "The first must return the page, the second must time out",
		},
	}
}
