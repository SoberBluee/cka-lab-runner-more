package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&EdgeAgentResolutionFailingLab{})
}

type EdgeAgentResolutionFailingLab struct {
	BaseLab
}

func (l *EdgeAgentResolutionFailingLab) ID() string {
	return "edge_agent_resolution_failing"
}

func (l *EdgeAgentResolutionFailingLab) Title() string {
	return "Edge Agent Name Resolution Failing"
}

func (l *EdgeAgentResolutionFailingLab) Category() Category {
	return CategoryDNS
}

func (l *EdgeAgentResolutionFailingLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *EdgeAgentResolutionFailingLab) Description() string {
	return `The 'agent' Deployment in namespace 'node-agents' monitors traffic on the node
interface and must keep running on the host network. It is Running, but it cannot resolve
any Service name, including kubernetes.default.svc.cluster.local.

Your task: Make the agent resolve cluster Service names while it keeps using the host network.`
}

func (l *EdgeAgentResolutionFailingLab) Hints() []string {
	return []string{
		"Read the pod spec before touching anything — note how the pod attaches to the network",
		"kubectl exec into the agent pod and check /etc/resolv.conf",
		"Pods that share the node network namespace do not get cluster DNS from the usual default",
		"There is a dnsPolicy value designed for exactly this combination; removing hostNetwork is not the fix",
	}
}

func (l *EdgeAgentResolutionFailingLab) EstimatedTime() int {
	return 20
}

func (l *EdgeAgentResolutionFailingLab) Tags() []string {
	return []string{"dns", "dnspolicy", "hostnetwork", "troubleshooting"}
}

func (l *EdgeAgentResolutionFailingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *EdgeAgentResolutionFailingLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: node-agents
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent
  namespace: node-agents
spec:
  replicas: 1
  selector:
    matchLabels:
      app: agent
  template:
    metadata:
      labels:
        app: agent
    spec:
      hostNetwork: true
      containers:
      - name: agent
        image: busybox:1.28
        command: ["sh", "-c", "sleep 3600"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying edge agent scenario: %w", err)
	}
	return nil
}

func (l *EdgeAgentResolutionFailingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	pod, err := waitForReadyPod(ctx, kubeconfigPath, "node-agents", "app=agent")
	if err != nil {
		return err
	}

	output, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "node-agents", pod,
		"--", "nslookup", "kubernetes.default.svc.cluster.local")
	if err == nil && dnsLookupAnswered(output) {
		return fmt.Errorf("agent pod already resolves cluster Service names")
	}
	return nil
}

func (l *EdgeAgentResolutionFailingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	pod, err := waitForReadyPod(ctx, kubeconfigPath, "node-agents", "app=agent")
	if err != nil {
		return err
	}

	spec, err := kubectl(ctx, kubeconfigPath, "get", "pod", pod, "-n", "node-agents",
		"-o", "jsonpath={.spec.hostNetwork} {.spec.dnsPolicy}")
	if err != nil {
		return fmt.Errorf("failed to read agent pod spec: %w", err)
	}
	fields := strings.Fields(spec)
	if len(fields) != 2 {
		return fmt.Errorf("unexpected pod spec output: %q", spec)
	}
	if fields[0] != "true" {
		return fmt.Errorf("the agent must keep hostNetwork: true, but it is %q", fields[0])
	}
	if fields[1] != "ClusterFirstWithHostNet" {
		return fmt.Errorf("agent pod dnsPolicy is %q, expected ClusterFirstWithHostNet", fields[1])
	}

	return waitFor(ctx, 60*time.Second, func() error {
		output, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "node-agents", pod,
			"--", "nslookup", "kubernetes.default.svc.cluster.local")
		if err != nil {
			return fmt.Errorf("agent pod still cannot resolve cluster Service names: %w", err)
		}
		if !dnsLookupAnswered(output) {
			return fmt.Errorf("lookup returned no answer: %s", output)
		}
		return nil
	})
}

func (l *EdgeAgentResolutionFailingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Reproduce the failure",
			Command:     "kubectl exec -n node-agents deploy/agent -- nslookup kubernetes.default.svc.cluster.local",
		},
		{
			Description: "Inspect the resolver the pod is using",
			Command:     "kubectl exec -n node-agents deploy/agent -- cat /etc/resolv.conf",
			Notes:       "It is the node's resolv.conf, so cluster Service records are not visible",
		},
		{
			Description: "Spot the reason in the pod spec",
			Command:     "kubectl get deploy agent -n node-agents -o yaml | grep -E 'hostNetwork|dnsPolicy'",
			Notes:       "With hostNetwork: true the default ClusterFirst policy is silently downgraded to Default",
		},
		{
			Description: "Set the policy that keeps cluster DNS on the host network",
			Command:     "kubectl patch deployment agent -n node-agents -p '{\"spec\":{\"template\":{\"spec\":{\"dnsPolicy\":\"ClusterFirstWithHostNet\"}}}}'",
			Notes:       "Keep hostNetwork: true — the agent still needs the node interface",
		},
		{
			Description: "Confirm the new pod resolves Service names",
			Command:     "kubectl exec -n node-agents deploy/agent -- nslookup kubernetes.default.svc.cluster.local",
		},
	}
}
