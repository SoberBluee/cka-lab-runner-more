package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&OutboundSyncFailingLab{})
}

type OutboundSyncFailingLab struct {
	BaseLab
}

func (l *OutboundSyncFailingLab) ID() string {
	return "outbound_sync_failing"
}

func (l *OutboundSyncFailingLab) Title() string {
	return "Sync Worker Cannot Resolve Or Reach Its Vendor"
}

func (l *OutboundSyncFailingLab) Category() Category {
	return CategoryNetworking
}

func (l *OutboundSyncFailingLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *OutboundSyncFailingLab) Description() string {
	return `The 'sync-worker' pod in namespace 'sync' pulls a feed from
http://vendor-api.vendor.svc.cluster.local every minute. Since the namespace was locked
down, every attempt fails — and it fails before any connection is even attempted, with a
name resolution error.

The vendor side is healthy and other namespaces can reach it. The lockdown in 'sync' is
deliberate and must stay: outbound traffic is denied unless it is explicitly permitted.

Your task: permit exactly what the worker needs so it can fetch the feed by DNS name again.`
}

func (l *OutboundSyncFailingLab) Hints() []string {
	return []string{
		"'bad address' or 'unknown host' means the failure happened before the HTTP request — resolve the name first, connect second",
		"kubectl get networkpolicy -n sync -o yaml — check which policyTypes are listed",
		"An egress lockdown also blocks the pod's DNS queries to the cluster DNS service",
		"Cluster DNS lives in kube-system and listens on port 53 over both UDP and TCP",
		"You need two egress permissions: one to the DNS service, one to the vendor namespace on TCP 80",
	}
}

func (l *OutboundSyncFailingLab) EstimatedTime() int {
	return 25
}

func (l *OutboundSyncFailingLab) Tags() []string {
	return []string{"networking", "connectivity", "name-resolution", "troubleshooting"}
}

func (l *OutboundSyncFailingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return waitDNSPodsReady(ctx, kubeconfigPath)
}

func (l *OutboundSyncFailingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: vendor
---
apiVersion: v1
kind: Namespace
metadata:
  name: sync
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vendor-api
  namespace: vendor
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vendor-api
  template:
    metadata:
      labels:
        app: vendor-api
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
  name: vendor-api
  namespace: vendor
spec:
  selector:
    app: vendor-api
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sync-worker
  namespace: sync
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sync-worker
  template:
    metadata:
      labels:
        app: sync-worker
    spec:
      containers:
      - name: worker
        image: busybox:1.28
        command: ["sh", "-c", "while true; do sleep 30; done"]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-egress
  namespace: sync
spec:
  podSelector: {}
  policyTypes:
  - Egress
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying outbound sync scenario: %w", err)
	}

	if err := deploymentReady(ctx, kubeconfigPath, "vendor", "vendor-api", 1, 120*time.Second); err != nil {
		return err
	}
	return deploymentReady(ctx, kubeconfigPath, "sync", "sync-worker", 1, 120*time.Second)
}

func (l *OutboundSyncFailingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)

	pod, err := podNameByLabel(ctx, kubeconfigPath, "sync", "app=sync-worker")
	if err != nil {
		return err
	}
	if _, err := httpGetFromPod(ctx, kubeconfigPath, "sync", pod,
		"http://vendor-api.vendor.svc.cluster.local", 5); err == nil {
		return fmt.Errorf("the sync worker can already reach the vendor — the CNI may not be enforcing NetworkPolicies")
	}
	return nil
}

func (l *OutboundSyncFailingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	policies, err := kubectl(ctx, kubeconfigPath, "get", "networkpolicy", "-n", "sync",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return fmt.Errorf("listing policies in sync: %w", err)
	}
	if !strings.Contains(policies, "default-deny-egress") {
		return fmt.Errorf("the default-deny-egress policy was deleted — restore it and add allow rules alongside it")
	}

	pod, err := podNameByLabel(ctx, kubeconfigPath, "sync", "app=sync-worker")
	if err != nil {
		return err
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := httpGetFromPod(ctx, kubeconfigPath, "sync", pod,
			"http://vendor-api.vendor.svc.cluster.local", 5)
		if err != nil {
			return fmt.Errorf("sync-worker still cannot fetch the feed by name: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response from vendor-api: %q", strings.TrimSpace(output))
		}
		return nil
	})
}

func (l *OutboundSyncFailingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Separate resolution from connectivity",
			Command: `kubectl exec -n sync deploy/sync-worker -- nslookup vendor-api.vendor.svc.cluster.local
kubectl exec -n sync deploy/sync-worker -- wget -qO- --timeout=3 http://vendor-api.vendor.svc.cluster.local`,
			Notes: "The lookup itself fails, so the HTTP test never gets a chance",
		},
		{
			Description: "Find the lockdown",
			Command:     "kubectl get networkpolicy -n sync -o yaml",
			Notes:       "podSelector: {} with policyTypes: Egress and no rules denies all outbound traffic, DNS included",
		},
		{
			Description: "Permit DNS and the vendor namespace",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-sync-egress
  namespace: sync
spec:
  podSelector:
    matchLabels:
      app: sync-worker
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
      podSelector:
        matchLabels:
          k8s-app: kube-dns
    ports:
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 53
  - to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: vendor
    ports:
    - protocol: TCP
      port: 80
EOF`,
			Notes: "Note the two separate 'to' blocks — one for DNS, one for the vendor. Forgetting DNS is the classic mistake",
		},
		{
			Description: "Confirm resolution and the fetch both work",
			Command: `kubectl exec -n sync deploy/sync-worker -- nslookup vendor-api.vendor.svc.cluster.local
kubectl exec -n sync deploy/sync-worker -- wget -qO- --timeout=3 http://vendor-api.vendor.svc.cluster.local`,
		},
	}
}
