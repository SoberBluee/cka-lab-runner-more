package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&RBACSADeniedLab{})
}

type RBACSADeniedLab struct {
	BaseLab
}

func (l *RBACSADeniedLab) ID() string {
	return "rbac_sa_denied"
}

func (l *RBACSADeniedLab) Title() string {
	return "ServiceAccount Access Denied"
}

func (l *RBACSADeniedLab) Category() Category {
	return CategoryRBAC
}

func (l *RBACSADeniedLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *RBACSADeniedLab) Description() string {
	return `A CI ServiceAccount named 'deployer' in namespace 'ci' cannot create Deployments.
Jobs that use this ServiceAccount fail with Forbidden errors.

Your task: Fix RBAC so the deployer ServiceAccount can create Deployments in the ci namespace.`
}

func (l *RBACSADeniedLab) Hints() []string {
	return []string{
		"Use kubectl auth can-i with --as-system=serviceaccount:ci:deployer",
		"Check Roles and RoleBindings in the ci namespace",
		"Confirm the Role covers the deployments resource and apps API group",
		"The binding subject must reference the correct ServiceAccount",
	}
}

func (l *RBACSADeniedLab) EstimatedTime() int {
	return 20
}

func (l *RBACSADeniedLab) Tags() []string {
	return []string{"rbac", "serviceaccount", "roles", "deployments", "troubleshooting"}
}

func (l *RBACSADeniedLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *RBACSADeniedLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: ci
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: deployer
  namespace: ci
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: deployer-role
  namespace: ci
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: deployer-binding
  namespace: ci
subjects:
- kind: ServiceAccount
  name: deployer
  namespace: ci
roleRef:
  kind: Role
  name: deployer-role
  apiGroup: rbac.authorization.k8s.io
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying RBAC SA scenario: %w", err)
	}
	return nil
}

func (l *RBACSADeniedLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(3 * time.Second)
	return nil
}

func (l *RBACSADeniedLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "auth", "can-i", "create", "deployments",
		"-n", "ci", "--as=system:serviceaccount:ci:deployer")
	if err != nil {
		return fmt.Errorf("deployer SA still cannot create deployments: %w", err)
	}
	if strings.TrimSpace(output) != "yes" {
		return fmt.Errorf("deployer SA still cannot create deployments (got: %s)", output)
	}
	return nil
}

func (l *RBACSADeniedLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Test current permissions",
			Command:     "kubectl auth can-i create deployments -n ci --as=system:serviceaccount:ci:deployer",
			Notes:       "Should return no",
		},
		{
			Description: "Inspect the Role",
			Command:     "kubectl describe role deployer-role -n ci",
			Notes:       "Only core pods permissions exist; deployments live in apiGroup apps",
		},
		{
			Description: "Add deployments permissions",
			Command:     "kubectl apply -f - <<'EOF'\napiVersion: rbac.authorization.k8s.io/v1\nkind: Role\nmetadata:\n  name: deployer-role\n  namespace: ci\nrules:\n- apiGroups: [\"\"]\n  resources: [\"pods\"]\n  verbs: [\"get\", \"list\", \"watch\", \"create\"]\n- apiGroups: [\"apps\"]\n  resources: [\"deployments\"]\n  verbs: [\"get\", \"list\", \"watch\", \"create\", \"update\", \"patch\"]\nEOF",
		},
		{
			Description: "Verify",
			Command:     "kubectl auth can-i create deployments -n ci --as=system:serviceaccount:ci:deployer",
		},
	}
}
