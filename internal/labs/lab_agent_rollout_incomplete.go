package labs

import (
	"context"
	"fmt"
	"time"
)

func init() {
	Register(&AgentRolloutIncompleteLab{})
}

type AgentRolloutIncompleteLab struct {
	BaseLab
}

func (l *AgentRolloutIncompleteLab) ID() string {
	return "agent_rollout_incomplete"
}

func (l *AgentRolloutIncompleteLab) Title() string {
	return "Security Agent Covers Zero Nodes"
}

func (l *AgentRolloutIncompleteLab) Category() Category {
	return CategoryWorkloads
}

func (l *AgentRolloutIncompleteLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *AgentRolloutIncompleteLab) Description() string {
	return `Compliance requires the 'security-agent' in namespace 'platform-agents' to run on
every node in the cluster. The object exists and was accepted by the API server, but
'kubectl get ds -n platform-agents' reports DESIRED 0, CURRENT 0, READY 0 — the cluster
does not even want to place it anywhere.

The nodes carry a hardening marker that the security team applied deliberately; leave it
in place.

Your task: get the agent running on every node in the cluster.`
}

func (l *AgentRolloutIncompleteLab) Hints() []string {
	return []string{
		"DESIRED 0 is the clue: the controller decided no node is eligible, so there is nothing to describe on a pod",
		"kubectl describe ds security-agent -n platform-agents shows how many nodes were considered",
		"kubectl describe node <node> | grep -i -A3 taints — what would stop any pod from being placed there?",
		"Workloads that must run everywhere carry tolerations for the markers their nodes have",
		"Add spec.template.spec.tolerations matching the node's key, value and effect",
	}
}

func (l *AgentRolloutIncompleteLab) EstimatedTime() int {
	return 20
}

func (l *AgentRolloutIncompleteLab) Tags() []string {
	return []string{"workloads", "node-coverage", "nodes", "troubleshooting"}
}

func (l *AgentRolloutIncompleteLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *AgentRolloutIncompleteLab) Break(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if _, err := kubectl(ctx, kubeconfigPath, "taint", "nodes", node,
			"hardened=true:NoSchedule", "--overwrite"); err != nil {
			return fmt.Errorf("marking node %s: %w", node, err)
		}
	}

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: platform-agents
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: security-agent
  namespace: platform-agents
spec:
  selector:
    matchLabels:
      app: security-agent
  template:
    metadata:
      labels:
        app: security-agent
    spec:
      containers:
      - name: agent
        image: busybox:1.28
        command: ["sh", "-c", "while true; do sleep 30; done"]
        resources:
          requests:
            cpu: 10m
            memory: 16Mi
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying security agent scenario: %w", err)
	}
	return nil
}

func (l *AgentRolloutIncompleteLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(8 * time.Second)

	desired, _, err := daemonSetCounts(ctx, kubeconfigPath, "platform-agents", "security-agent")
	if err != nil {
		return err
	}
	if desired != 0 {
		return fmt.Errorf("security-agent already wants %d node(s)", desired)
	}
	return nil
}

func (l *AgentRolloutIncompleteLab) Verify(ctx context.Context, kubeconfigPath string) error {
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if !nodeHasTaintKey(ctx, kubeconfigPath, node, "hardened") {
			return fmt.Errorf("the hardening marker was removed from node %s — restore it and make the agent tolerate it", node)
		}
	}

	return waitFor(ctx, 90*time.Second, func() error {
		desired, ready, err := daemonSetCounts(ctx, kubeconfigPath, "platform-agents", "security-agent")
		if err != nil {
			return err
		}
		if desired < len(nodes) {
			return fmt.Errorf("security-agent targets %d of %d nodes", desired, len(nodes))
		}
		if ready != desired {
			return fmt.Errorf("security-agent is ready on %d of %d nodes", ready, desired)
		}
		return nil
	})
}

func (l *AgentRolloutIncompleteLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the controller wants zero nodes",
			Command:     "kubectl get ds security-agent -n platform-agents",
			Notes:       "DESIRED 0 means no node passed the eligibility check",
		},
		{
			Description: "Find what makes the nodes ineligible",
			Command:     "kubectl describe nodes | grep -i -A3 taints",
			Notes:       "hardened=true:NoSchedule",
		},
		{
			Description: "Add the toleration to the pod template",
			Command: `kubectl patch ds security-agent -n platform-agents --type merge -p '{"spec":{"template":{"spec":{"tolerations":[
  {"key":"hardened","operator":"Equal","value":"true","effect":"NoSchedule"}]}}}}'`,
			Notes: "Agents that must cover the whole fleet often use operator: Exists with no key to tolerate everything",
		},
		{
			Description: "Confirm coverage",
			Command:     "kubectl get ds security-agent -n platform-agents && kubectl get pods -n platform-agents -o wide",
			Notes:       "DESIRED and READY should both equal the node count",
		},
	}
}
