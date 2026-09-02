package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&AgentIdentityLostLab{})
}

type AgentIdentityLostLab struct {
	BaseLab
}

func (l *AgentIdentityLostLab) ID() string {
	return "agent_identity_lost"
}

func (l *AgentIdentityLostLab) Title() string {
	return "Edge Agent Cannot Find Its Credentials"
}

func (l *AgentIdentityLostLab) Category() Category {
	return CategoryRBAC
}

func (l *AgentIdentityLostLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *AgentIdentityLostLab) Description() string {
	return `The 'edge-agent' Deployment in namespace 'edge' starts up and immediately logs:

  unable to load in-cluster credentials: open
  /var/run/secrets/kubernetes.io/serviceaccount/token: no such file or directory

The permissions team already prepared everything this workload should be allowed to do,
and they insist the grant is correct.

Your task: make the pod's credentials available inside the container at the standard path,
without changing what the identity is permitted to do.`
}

func (l *AgentIdentityLostLab) Hints() []string {
	return []string{
		"kubectl exec -n edge deploy/edge-agent -- ls /var/run/secrets/kubernetes.io/serviceaccount confirms the symptom",
		"The credential directory is normally injected automatically — something turned that off",
		"kubectl get sa edge-agent -n edge -o yaml and kubectl get deploy edge-agent -n edge -o yaml both have a field that controls this",
		"automountServiceAccountToken can be set on the ServiceAccount and overridden on the pod spec",
		"The pod spec value wins when both are set",
		"Pods that already exist keep whatever was injected at creation time — a rollout restart is needed",
	}
}

func (l *AgentIdentityLostLab) EstimatedTime() int {
	return 15
}

func (l *AgentIdentityLostLab) Tags() []string {
	return []string{"rbac", "identity", "api-access", "troubleshooting"}
}

func (l *AgentIdentityLostLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *AgentIdentityLostLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: edge
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: edge-agent
  namespace: edge
automountServiceAccountToken: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: edge-agent-role
  namespace: edge
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: edge-agent-binding
  namespace: edge
subjects:
- kind: ServiceAccount
  name: edge-agent
  namespace: edge
roleRef:
  kind: Role
  name: edge-agent-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: edge-agent
  namespace: edge
spec:
  replicas: 1
  selector:
    matchLabels:
      app: edge-agent
  template:
    metadata:
      labels:
        app: edge-agent
    spec:
      serviceAccountName: edge-agent
      containers:
      - name: agent
        image: busybox:1.28
        command: ["sh", "-c", "while true; do sleep 30; done"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying edge agent scenario: %w", err)
	}
	return deploymentReady(ctx, kubeconfigPath, "edge", "edge-agent", 1, 90*time.Second)
}

func (l *AgentIdentityLostLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	pod, err := podNameByLabel(ctx, kubeconfigPath, "edge", "app=edge-agent")
	if err != nil {
		return err
	}
	if _, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "edge", pod, "--",
		"test", "-s", "/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return fmt.Errorf("the credential token is already mounted")
	}
	return nil
}

func (l *AgentIdentityLostLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if !authCanI(ctx, kubeconfigPath, "system:serviceaccount:edge:edge-agent", "list", "configmaps", "edge") {
		return fmt.Errorf("the edge-agent identity lost its configmap access — restore the Role and binding")
	}

	return waitFor(ctx, 90*time.Second, func() error {
		pod, err := podNameByLabel(ctx, kubeconfigPath, "edge", "app=edge-agent")
		if err != nil {
			return err
		}

		podSA, err := kubectl(ctx, kubeconfigPath, "get", "pod", pod, "-n", "edge",
			"-o", "jsonpath={.spec.serviceAccountName}")
		if err != nil {
			return fmt.Errorf("reading pod identity: %w", err)
		}
		if strings.TrimSpace(podSA) != "edge-agent" {
			return fmt.Errorf("pod runs as %q, expected edge-agent", strings.TrimSpace(podSA))
		}

		if _, err := kubectl(ctx, kubeconfigPath, "exec", "-n", "edge", pod, "--",
			"test", "-s", "/var/run/secrets/kubernetes.io/serviceaccount/token"); err != nil {
			return fmt.Errorf("no credential token inside the running pod yet")
		}
		return nil
	})
}

func (l *AgentIdentityLostLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Reproduce the symptom inside the pod",
			Command:     "kubectl exec -n edge deploy/edge-agent -- ls /var/run/secrets/kubernetes.io/serviceaccount",
			Notes:       "The directory does not exist at all",
		},
		{
			Description: "Confirm the identity and its grant are fine",
			Command:     "kubectl auth can-i list configmaps -n edge --as=system:serviceaccount:edge:edge-agent",
			Notes:       "yes — this is not an RBAC problem, the pod simply has no credentials to present",
		},
		{
			Description: "Find what suppressed the injection",
			Command:     "kubectl get sa edge-agent -n edge -o yaml | grep automount",
			Notes:       "automountServiceAccountToken: false on the ServiceAccount",
		},
		{
			Description: "Re-enable it (option A: on the ServiceAccount)",
			Command:     `kubectl patch sa edge-agent -n edge -p '{"automountServiceAccountToken":true}'`,
		},
		{
			Description: "Restart the workload so a new pod picks the setting up",
			Command:     "kubectl rollout restart deployment edge-agent -n edge",
			Notes:       "Patching the ServiceAccount alone does not change pods that already exist",
		},
		{
			Description: "Re-enable it (option B: override on the pod template)",
			Command:     `kubectl patch deployment edge-agent -n edge --type merge -p '{"spec":{"template":{"spec":{"automountServiceAccountToken":true}}}}'`,
			Notes:       "The pod spec value takes precedence over the ServiceAccount setting",
		},
		{
			Description: "Confirm the credentials are mounted in the new pod",
			Command:     "kubectl exec -n edge deploy/edge-agent -- ls /var/run/secrets/kubernetes.io/serviceaccount",
			Notes:       "token, ca.crt and namespace should all be present",
		},
	}
}
