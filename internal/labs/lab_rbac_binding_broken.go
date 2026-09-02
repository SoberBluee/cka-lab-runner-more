package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&RBACBindingBrokenLab{})
}

type RBACBindingBrokenLab struct {
	BaseLab
}

func (l *RBACBindingBrokenLab) ID() string {
	return "rbac_binding_broken"
}

func (l *RBACBindingBrokenLab) Title() string {
	return "Broken Role Binding"
}

func (l *RBACBindingBrokenLab) Category() Category {
	return CategoryRBAC
}

func (l *RBACBindingBrokenLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *RBACBindingBrokenLab) Description() string {
	return `User 'alice' should be able to list Secrets in namespace 'secure-apps',
but kubectl auth checks keep returning no even though a Role exists.

Your task: Fix the RBAC binding so alice can list Secrets in secure-apps.`
}

func (l *RBACBindingBrokenLab) Hints() []string {
	return []string{
		"List Roles and RoleBindings in the secure-apps namespace",
		"Inspect roleRef and subjects carefully",
		"A RoleBinding that points at the wrong Role name grants nothing useful",
		"Use kubectl auth can-i list secrets -n secure-apps --as=alice",
	}
}

func (l *RBACBindingBrokenLab) EstimatedTime() int {
	return 20
}

func (l *RBACBindingBrokenLab) Tags() []string {
	return []string{"rbac", "rolebinding", "secrets", "troubleshooting"}
}

func (l *RBACBindingBrokenLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *RBACBindingBrokenLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: secure-apps
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secret-reader
  namespace: secure-apps
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: alice-secrets
  namespace: secure-apps
subjects:
- kind: User
  name: alice
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: secret-reader-v2
  apiGroup: rbac.authorization.k8s.io
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying broken rolebinding scenario: %w", err)
	}
	return nil
}

func (l *RBACBindingBrokenLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(3 * time.Second)
	return nil
}

func (l *RBACBindingBrokenLab) Verify(ctx context.Context, kubeconfigPath string) error {
	output, err := kubectl(ctx, kubeconfigPath, "auth", "can-i", "list", "secrets",
		"-n", "secure-apps", "--as=alice")
	if err != nil {
		return fmt.Errorf("alice still cannot list secrets: %w", err)
	}
	if strings.TrimSpace(output) != "yes" {
		return fmt.Errorf("alice still cannot list secrets (got: %s)", output)
	}

	ref, err := kubectl(ctx, kubeconfigPath, "get", "rolebinding", "alice-secrets", "-n", "secure-apps",
		"-o", "jsonpath={.roleRef.name}")
	if err != nil {
		return fmt.Errorf("failed to check rolebinding: %w", err)
	}
	if strings.TrimSpace(ref) != "secret-reader" {
		return fmt.Errorf("rolebinding still references %s", ref)
	}
	return nil
}

func (l *RBACBindingBrokenLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Test permissions",
			Command:     "kubectl auth can-i list secrets -n secure-apps --as=alice",
			Notes:       "Should return no",
		},
		{
			Description: "List RBAC objects",
			Command:     "kubectl get role,rolebinding -n secure-apps",
		},
		{
			Description: "Inspect the RoleBinding",
			Command:     "kubectl get rolebinding alice-secrets -n secure-apps -o yaml",
			Notes:       "roleRef.name is secret-reader-v2 but the Role is secret-reader",
		},
		{
			Description: "Fix roleRef",
			Command:     "kubectl delete rolebinding alice-secrets -n secure-apps && kubectl create rolebinding alice-secrets --role=secret-reader --user=alice -n secure-apps",
			Notes:       "roleRef is immutable; recreate the RoleBinding",
		},
		{
			Description: "Verify",
			Command:     "kubectl auth can-i list secrets -n secure-apps --as=alice",
		},
	}
}
