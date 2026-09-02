package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&InventorySyncUnauthorizedLab{})
}

type InventorySyncUnauthorizedLab struct {
	BaseLab
}

func (l *InventorySyncUnauthorizedLab) ID() string {
	return "inventory_sync_unauthorized"
}

func (l *InventorySyncUnauthorizedLab) Title() string {
	return "Inventory Sync Rejected By The API"
}

func (l *InventorySyncUnauthorizedLab) Category() Category {
	return CategoryRBAC
}

func (l *InventorySyncUnauthorizedLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *InventorySyncUnauthorizedLab) Description() string {
	return `The 'inventory-sync' Deployment in namespace 'warehouse' talks to the Kubernetes API
to build a list of the pods it should watch. Its logs are full of:

  pods is forbidden: User "system:serviceaccount:warehouse:default" cannot list
  resource "pods" in API group "" in the namespace "warehouse"

Security will not widen what the shared 'default' identity in this namespace can do.

Your task: make the workload run under the 'sync-agent' identity that was created for it,
and make sure that identity can list pods in 'warehouse'. The 'default' ServiceAccount
must be left with no extra access.`
}

func (l *InventorySyncUnauthorizedLab) Hints() []string {
	return []string{
		"The error message tells you which identity the pod is using — compare it with the ServiceAccounts that exist in the namespace",
		"kubectl get sa,role,rolebinding -n warehouse",
		"kubectl auth can-i list pods -n warehouse --as=system:serviceaccount:warehouse:sync-agent",
		"A pod uses spec.serviceAccountName; changing it on a Deployment triggers a new rollout",
		"Check the verbs on the existing Role, not just that a Role exists — 'get' does not imply 'list'",
	}
}

func (l *InventorySyncUnauthorizedLab) EstimatedTime() int {
	return 20
}

func (l *InventorySyncUnauthorizedLab) Tags() []string {
	return []string{"rbac", "identity", "api-access", "troubleshooting"}
}

func (l *InventorySyncUnauthorizedLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *InventorySyncUnauthorizedLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: warehouse
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sync-agent
  namespace: warehouse
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: sync-agent-role
  namespace: warehouse
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: sync-agent-binding
  namespace: warehouse
subjects:
- kind: ServiceAccount
  name: sync-agent
  namespace: warehouse
roleRef:
  kind: Role
  name: sync-agent-role
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inventory-sync
  namespace: warehouse
spec:
  replicas: 1
  selector:
    matchLabels:
      app: inventory-sync
  template:
    metadata:
      labels:
        app: inventory-sync
    spec:
      containers:
      - name: sync
        image: busybox:1.28
        command: ["sh", "-c", "while true; do sleep 30; done"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying inventory sync scenario: %w", err)
	}
	return nil
}

func (l *InventorySyncUnauthorizedLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)

	if authCanI(ctx, kubeconfigPath, "system:serviceaccount:warehouse:sync-agent", "list", "pods", "warehouse") {
		return fmt.Errorf("sync-agent can already list pods")
	}
	return nil
}

func (l *InventorySyncUnauthorizedLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if !authCanI(ctx, kubeconfigPath, "system:serviceaccount:warehouse:sync-agent", "list", "pods", "warehouse") {
		return fmt.Errorf("sync-agent still cannot list pods in warehouse")
	}

	if authCanI(ctx, kubeconfigPath, "system:serviceaccount:warehouse:default", "list", "pods", "warehouse") {
		return fmt.Errorf("the default ServiceAccount in warehouse was granted pod access — grant it to sync-agent instead")
	}

	specSA, err := kubectl(ctx, kubeconfigPath, "get", "deployment", "inventory-sync", "-n", "warehouse",
		"-o", "jsonpath={.spec.template.spec.serviceAccountName}")
	if err != nil {
		return fmt.Errorf("reading inventory-sync deployment: %w", err)
	}
	if strings.TrimSpace(specSA) != "sync-agent" {
		return fmt.Errorf("inventory-sync pod template uses ServiceAccount %q, expected sync-agent", strings.TrimSpace(specSA))
	}

	return waitFor(ctx, 60*time.Second, func() error {
		podSA, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "warehouse",
			"-l", "app=inventory-sync", "--field-selector=status.phase=Running",
			"-o", "jsonpath={.items[0].spec.serviceAccountName}")
		if err != nil || strings.TrimSpace(podSA) != "sync-agent" {
			return fmt.Errorf("no running inventory-sync pod is using the sync-agent identity yet")
		}
		return nil
	})
}

func (l *InventorySyncUnauthorizedLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm which identity the pod runs as",
			Command:     "kubectl get pod -n warehouse -l app=inventory-sync -o jsonpath='{.items[0].spec.serviceAccountName}'",
			Notes:       "It is 'default', which is what the error message named",
		},
		{
			Description: "See what exists for this workload",
			Command:     "kubectl get sa,role,rolebinding -n warehouse",
			Notes:       "sync-agent and its Role and binding are already there",
		},
		{
			Description: "Check what the identity is actually allowed to do",
			Command:     "kubectl auth can-i list pods -n warehouse --as=system:serviceaccount:warehouse:sync-agent",
			Notes:       "no — the Role only grants 'get'",
		},
		{
			Description: "Add the missing verbs to the Role",
			Command:     `kubectl patch role sync-agent-role -n warehouse --type json -p='[{"op":"replace","path":"/rules/0/verbs","value":["get","list","watch"]}]'`,
		},
		{
			Description: "Point the workload at its own identity",
			Command:     `kubectl patch deployment inventory-sync -n warehouse --type merge -p '{"spec":{"template":{"spec":{"serviceAccountName":"sync-agent"}}}}'`,
		},
		{
			Description: "Verify both halves of the fix",
			Command:     "kubectl auth can-i list pods -n warehouse --as=system:serviceaccount:warehouse:sync-agent && kubectl get pod -n warehouse -l app=inventory-sync -o jsonpath='{.items[0].spec.serviceAccountName}'",
		},
	}
}
