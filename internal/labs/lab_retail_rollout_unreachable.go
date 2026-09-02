package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&RetailRolloutUnreachableLab{})
}

type RetailRolloutUnreachableLab struct {
	BaseLab
}

// labProxySelectorKey is the node label this lab adds to the kube-proxy pod
// template so no node matches it. Cleanup keys off the same constant.
const labProxySelectorKey = "cka-lab/proxy"

func (l *RetailRolloutUnreachableLab) ID() string {
	return "retail_rollout_unreachable"
}

func (l *RetailRolloutUnreachableLab) Title() string {
	return "Only Newly Created Services Are Unreachable"
}

func (l *RetailRolloutUnreachableLab) Category() Category {
	return CategoryNetworking
}

func (l *RetailRolloutUnreachableLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *RetailRolloutUnreachableLab) Description() string {
	return `This morning's rollout in namespace 'retail' cannot be reached. From the 'shopper'
pod in that namespace:

  - the name catalog-api.retail.svc.cluster.local resolves fine
  - the Service has healthy endpoints
  - calling the backing pod IP directly returns the page
  - calling the Service ClusterIP hangs until it times out

Services that already existed before this morning still work normally, which is why nobody
noticed sooner.

Your task: work out which part of the cluster stopped doing its job and restore it, so the
call to the Service succeeds.`
}

func (l *RetailRolloutUnreachableLab) Hints() []string {
	return []string{
		"Working DNS plus healthy endpoints plus a reachable pod IP narrows this down to Service load balancing itself",
		"Something has to translate a ClusterIP into a pod IP on every node — and it does that per node, continuously",
		"Old Services keep working because their rules were programmed before it stopped; new ones never got any",
		"kubectl get pods -n kube-system -o wide — is there one running on each node that should be?",
		"kubectl get ds -n kube-system and kubectl describe ds <name> -n kube-system: why does it target zero nodes?",
	}
}

func (l *RetailRolloutUnreachableLab) EstimatedTime() int {
	return 30
}

func (l *RetailRolloutUnreachableLab) Tags() []string {
	return []string{"networking", "services", "node-components", "troubleshooting"}
}

func (l *RetailRolloutUnreachableLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *RetailRolloutUnreachableLab) Break(ctx context.Context, kubeconfigPath string) error {
	if _, err := kubectl(ctx, kubeconfigPath, "patch", "daemonset", "kube-proxy", "-n", "kube-system",
		"--type", "merge", "-p",
		fmt.Sprintf(`{"spec":{"template":{"spec":{"nodeSelector":{%q:"enabled"}}}}}`, labProxySelectorKey)); err != nil {
		return fmt.Errorf("patching kube-proxy: %w", err)
	}

	if err := waitFor(ctx, 90*time.Second, func() error {
		desired, _, err := daemonSetCounts(ctx, kubeconfigPath, "kube-system", "kube-proxy")
		if err != nil {
			return err
		}
		if desired != 0 {
			return fmt.Errorf("kube-proxy still targets %d node(s)", desired)
		}
		pods, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "kube-system",
			"-l", "k8s-app=kube-proxy", "-o", "jsonpath={.items[*].metadata.name}")
		if err != nil {
			return err
		}
		if len(strings.Fields(pods)) > 0 {
			return fmt.Errorf("kube-proxy pods are still terminating")
		}
		return nil
	}); err != nil {
		return fmt.Errorf("waiting for the node component to stop: %w", err)
	}

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: retail
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog-api
  namespace: retail
spec:
  replicas: 1
  selector:
    matchLabels:
      app: catalog-api
  template:
    metadata:
      labels:
        app: catalog-api
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
  name: catalog-api
  namespace: retail
spec:
  selector:
    app: catalog-api
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: v1
kind: Pod
metadata:
  name: shopper
  namespace: retail
  labels:
    app: shopper
spec:
  containers:
  - name: shopper
    image: busybox:1.28
    command: ["sh", "-c", "while true; do sleep 30; done"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying retail scenario: %w", err)
	}
	return deploymentReady(ctx, kubeconfigPath, "retail", "catalog-api", 1, 120*time.Second)
}

func (l *RetailRolloutUnreachableLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)

	if _, err := httpGetFromPod(ctx, kubeconfigPath, "retail", "shopper",
		"http://catalog-api.retail.svc.cluster.local", 5); err == nil {
		return fmt.Errorf("the new Service is already reachable")
	}
	return nil
}

func (l *RetailRolloutUnreachableLab) Verify(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	if err := waitFor(ctx, 60*time.Second, func() error {
		desired, ready, err := daemonSetCounts(ctx, kubeconfigPath, "kube-system", "kube-proxy")
		if err != nil {
			return err
		}
		if desired < len(nodes) {
			return fmt.Errorf("the node component targets %d of %d nodes", desired, len(nodes))
		}
		if ready != desired {
			return fmt.Errorf("the node component is ready on %d of %d nodes", ready, desired)
		}
		return nil
	}); err != nil {
		return err
	}

	return waitFor(ctx, 45*time.Second, func() error {
		output, err := httpGetFromPod(ctx, kubeconfigPath, "retail", "shopper",
			"http://catalog-api.retail.svc.cluster.local", 5)
		if err != nil {
			return fmt.Errorf("the Service ClusterIP still does not answer: %w", err)
		}
		if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
			return fmt.Errorf("unexpected response from catalog-api: %q", strings.TrimSpace(output))
		}
		return nil
	})
}

func (l *RetailRolloutUnreachableLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the symptom pattern",
			Command: `kubectl exec -n retail shopper -- nslookup catalog-api.retail.svc.cluster.local
kubectl get endpoints catalog-api -n retail
POD_IP=$(kubectl get pod -n retail -l app=catalog-api -o jsonpath='{.items[0].status.podIP}')
kubectl exec -n retail shopper -- wget -qO- --timeout=3 http://$POD_IP | head -3
kubectl exec -n retail shopper -- wget -qO- --timeout=3 http://catalog-api.retail.svc.cluster.local`,
			Notes: "DNS, endpoints and pod IP all fine; only the ClusterIP path fails",
		},
		{
			Description: "Check the per-node components",
			Command:     "kubectl get pods -n kube-system -o wide | grep -i proxy",
			Notes:       "No kube-proxy pod is running anywhere",
		},
		{
			Description: "Find why it targets no nodes",
			Command:     "kubectl get ds kube-proxy -n kube-system -o jsonpath='{.spec.template.spec.nodeSelector}'",
			Notes:       "A nodeSelector was added for a label no node carries",
		},
		{
			Description: "Remove the bogus constraint, keeping the legitimate one",
			Command:     `kubectl patch ds kube-proxy -n kube-system --type json -p='[{"op":"remove","path":"/spec/template/spec/nodeSelector/cka-lab~1proxy"}]'`,
			Notes:       "~1 escapes the / in the label key. Do not drop the whole nodeSelector — kubernetes.io/os=linux belongs there",
		},
		{
			Description: "Confirm it is back on every node",
			Command:     "kubectl get ds kube-proxy -n kube-system && kubectl get pods -n kube-system -o wide | grep -i proxy",
		},
		{
			Description: "Confirm the Service answers",
			Command:     "kubectl exec -n retail shopper -- wget -qO- --timeout=3 http://catalog-api.retail.svc.cluster.local | head -3",
			Notes:       "Service rules are reprogrammed within seconds of the component starting",
		},
	}
}
