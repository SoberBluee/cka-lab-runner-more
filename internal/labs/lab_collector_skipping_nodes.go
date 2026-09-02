package labs

import (
	"context"
	"fmt"
	"time"
)

func init() {
	Register(&CollectorSkippingNodesLab{})
}

type CollectorSkippingNodesLab struct {
	BaseLab
}

func (l *CollectorSkippingNodesLab) ID() string {
	return "collector_skipping_nodes"
}

func (l *CollectorSkippingNodesLab) Title() string {
	return "Log Collector Deployed But Collecting Nothing"
}

func (l *CollectorSkippingNodesLab) Category() Category {
	return CategoryWorkloads
}

func (l *CollectorSkippingNodesLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *CollectorSkippingNodesLab) Description() string {
	return `The 'log-collector' in namespace 'observability' is supposed to run on every node so
the platform team can see application logs. Dashboards have been empty since it was rolled
out, and the object reports no pods at all.

Someone hardened the nodes last week and someone else wrote the collector manifest against
a node pool layout that was never applied to this cluster. Both problems are still present.

Your task: get the collector running on every node. Do not remove the node markers the
security team applied.`
}

func (l *CollectorSkippingNodesLab) Hints() []string {
	return []string{
		"There is more than one reason this cannot be placed — fix one and it will still report zero",
		"kubectl get ds log-collector -n observability -o yaml — read the whole pod template, not just the container",
		"Placement can be constrained by node affinity as well as repelled by node markers",
		"kubectl get nodes --show-labels — do the labels the affinity demands exist anywhere?",
		"You can either relax the affinity or label the nodes to match it; both are valid answers",
	}
}

func (l *CollectorSkippingNodesLab) EstimatedTime() int {
	return 25
}

func (l *CollectorSkippingNodesLab) Tags() []string {
	return []string{"workloads", "node-coverage", "affinity", "troubleshooting"}
}

func (l *CollectorSkippingNodesLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *CollectorSkippingNodesLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if _, err := kubectl(ctx, kubeconfigPath, "taint", "nodes", node,
			"maintenance=scheduled:NoSchedule", "--overwrite"); err != nil {
			return fmt.Errorf("marking node %s: %w", node, err)
		}
	}

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: observability
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: log-collector
  namespace: observability
spec:
  selector:
    matchLabels:
      app: log-collector
  template:
    metadata:
      labels:
        app: log-collector
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: node-pool
                operator: In
                values:
                - edge
      containers:
      - name: collector
        image: busybox:1.28
        command: ["sh", "-c", "while true; do sleep 30; done"]
        resources:
          requests:
            cpu: 10m
            memory: 16Mi
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying log collector scenario: %w", err)
	}
	return nil
}

func (l *CollectorSkippingNodesLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)

	desired, _, err := daemonSetCounts(ctx, kubeconfigPath, "observability", "log-collector")
	if err != nil {
		return err
	}
	if desired != 0 {
		return fmt.Errorf("log-collector already targets %d node(s)", desired)
	}
	return nil
}

func (l *CollectorSkippingNodesLab) Verify(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if !nodeHasTaintKey(ctx, kubeconfigPath, node, "maintenance") {
			return fmt.Errorf("the maintenance marker was removed from node %s — restore it and make the collector tolerate it", node)
		}
	}

	return waitFor(ctx, 90*time.Second, func() error {
		desired, ready, err := daemonSetCounts(ctx, kubeconfigPath, "observability", "log-collector")
		if err != nil {
			return err
		}
		if desired < len(nodes) {
			return fmt.Errorf("log-collector targets %d of %d nodes", desired, len(nodes))
		}
		if ready != desired {
			return fmt.Errorf("log-collector is ready on %d of %d nodes", ready, desired)
		}
		return nil
	})
}

func (l *CollectorSkippingNodesLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm nothing is being targeted",
			Command:     "kubectl get ds log-collector -n observability",
		},
		{
			Description: "Read the full pod template",
			Command:     "kubectl get ds log-collector -n observability -o yaml | sed -n '/template:/,/containers:/p'",
			Notes:       "Note the required nodeAffinity on node-pool=edge",
		},
		{
			Description: "Check whether any node carries that label",
			Command:     "kubectl get nodes --show-labels | tr ',' '\\n' | grep node-pool",
			Notes:       "No output — the affinity can never be satisfied",
		},
		{
			Description: "Check the node markers as well",
			Command:     "kubectl describe nodes | grep -i -A3 taints",
			Notes:       "maintenance=scheduled:NoSchedule repels pods that do not tolerate it",
		},
		{
			Description: "Fix the affinity (option A: drop the requirement)",
			Command:     "kubectl patch ds log-collector -n observability --type json -p='[{\"op\":\"remove\",\"path\":\"/spec/template/spec/affinity\"}]'",
		},
		{
			Description: "Fix the affinity (option B: label the nodes instead)",
			Command:     "kubectl label nodes --all node-pool=edge",
		},
		{
			Description: "Add the missing toleration",
			Command: `kubectl patch ds log-collector -n observability --type merge -p '{"spec":{"template":{"spec":{"tolerations":[
  {"key":"maintenance","operator":"Equal","value":"scheduled","effect":"NoSchedule"}]}}}}'`,
		},
		{
			Description: "Confirm full coverage",
			Command:     "kubectl get ds log-collector -n observability && kubectl get pods -n observability -o wide",
		},
	}
}
