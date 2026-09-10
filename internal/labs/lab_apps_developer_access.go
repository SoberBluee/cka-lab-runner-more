package labs

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(&AppsDeveloperAccessLab{})
}

// AppsDeveloperAccessLab is a creation lab: SA + Role + RoleBinding from scratch.
type AppsDeveloperAccessLab struct {
	BaseLab
}

func (l *AppsDeveloperAccessLab) ID() string { return "apps_developer_access" }

func (l *AppsDeveloperAccessLab) Title() string {
	return "Onboard Developer API Access"
}

func (l *AppsDeveloperAccessLab) Category() Category { return CategoryRBAC }

func (l *AppsDeveloperAccessLab) Difficulty() Difficulty { return DifficultyMedium }

func (l *AppsDeveloperAccessLab) EstimatedTime() int { return 8 }

func (l *AppsDeveloperAccessLab) Tags() []string {
	return []string{"creation", "rbac", "serviceaccount"}
}

func (l *AppsDeveloperAccessLab) Hints() []string { return nil }

func (l *AppsDeveloperAccessLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace apps

[Weight: 5%] | Time limit: 6–8 minutes

Context:
A new developer has been added to the apps team and needs API access limited to
application workloads in namespace apps. Namespace apps already exists.

Task:
1. Create ServiceAccount named dev in namespace apps.
2. Create Role named dev-role in namespace apps that allows create, list, and watch
   on pods and deployments (only).
3. Create RoleBinding named dev-rolebinding in namespace apps that binds Role
   dev-role to ServiceAccount dev.
4. Write the exact output of the following command (trimmed) to /opt/CKA/rbac-check.txt:
   kubectl auth can-i create deployments -n apps --as=system:serviceaccount:apps:dev
5. Delete any temporary debug Pods you created in namespace apps.

Constraints:
- Work only in context cka-lab and namespace apps (wrong placement = 0).
- Resource names must be exactly: dev, dev-role, dev-rolebinding (case-sensitive).
- Least privilege: do not grant delete, update, patch, or access to secrets.
- Do not create ClusterRoles or ClusterRoleBindings for this task.
- Do not leave temporary debug pods, Jobs, or test resources behind.`
}

func (l *AppsDeveloperAccessLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *AppsDeveloperAccessLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return fmt.Errorf("creating /opt/CKA: %w", err)
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/rbac-check.txt")

	ns := `apiVersion: v1
kind: Namespace
metadata:
  name: apps
`
	if err := kubectlApply(ctx, kubeconfigPath, ns); err != nil {
		return fmt.Errorf("creating apps namespace: %w", err)
	}

	// Ensure a clean slate if a prior attempt left objects (cleanup deletes ns, but be safe)
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "sa", "dev", "-n", "apps", "--ignore-not-found=true")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "role", "dev-role", "-n", "apps", "--ignore-not-found=true")
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "rolebinding", "dev-rolebinding", "-n", "apps", "--ignore-not-found=true")
	return nil
}

func (l *AppsDeveloperAccessLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(2 * time.Second)
	if authCanI(ctx, kubeconfigPath, "system:serviceaccount:apps:dev", "create", "deployments", "apps") {
		return fmt.Errorf("dev SA can already create deployments")
	}
	return nil
}

func (l *AppsDeveloperAccessLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if _, err := kubectl(ctx, kubeconfigPath, "get", "sa", "dev", "-n", "apps"); err != nil {
		return fmt.Errorf("ServiceAccount dev not found in apps: %w", err)
	}
	if _, err := kubectl(ctx, kubeconfigPath, "get", "role", "dev-role", "-n", "apps"); err != nil {
		return fmt.Errorf("Role dev-role not found in apps: %w", err)
	}
	if _, err := kubectl(ctx, kubeconfigPath, "get", "rolebinding", "dev-rolebinding", "-n", "apps"); err != nil {
		return fmt.Errorf("RoleBinding dev-rolebinding not found in apps: %w", err)
	}

	roleRef, err := kubectl(ctx, kubeconfigPath, "get", "rolebinding", "dev-rolebinding", "-n", "apps",
		"-o", "jsonpath={.roleRef.kind}/{.roleRef.name}")
	if err != nil || strings.TrimSpace(roleRef) != "Role/dev-role" {
		return fmt.Errorf("dev-rolebinding must reference Role/dev-role (got %q)", strings.TrimSpace(roleRef))
	}
	subject, err := kubectl(ctx, kubeconfigPath, "get", "rolebinding", "dev-rolebinding", "-n", "apps",
		"-o", "jsonpath={.subjects[0].kind}/{.subjects[0].name}")
	if err != nil || strings.TrimSpace(subject) != "ServiceAccount/dev" {
		return fmt.Errorf("dev-rolebinding subject must be ServiceAccount/dev (got %q)", strings.TrimSpace(subject))
	}

	const as = "system:serviceaccount:apps:dev"
	needYes := []struct{ verb, resource string }{
		{"create", "pods"},
		{"list", "pods"},
		{"watch", "pods"},
		{"create", "deployments"},
		{"list", "deployments"},
		{"watch", "deployments"},
	}
	for _, n := range needYes {
		if !authCanI(ctx, kubeconfigPath, as, n.verb, n.resource, "apps") {
			return fmt.Errorf("dev cannot %s %s in apps", n.verb, n.resource)
		}
	}
	needNo := []struct{ verb, resource string }{
		{"delete", "pods"},
		{"delete", "deployments"},
		{"list", "secrets"},
		{"get", "secrets"},
	}
	for _, n := range needNo {
		if authCanI(ctx, kubeconfigPath, as, n.verb, n.resource, "apps") {
			return fmt.Errorf("dev must not be able to %s %s (least privilege)", n.verb, n.resource)
		}
	}

	// Reject ClusterRoleBinding grants for this SA
	crb, _ := kubectl(ctx, kubeconfigPath, "get", "clusterrolebindings",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{'|'}{range .subjects[*]}{.namespace}{'/'}{.name}{' '}{end}{'\\n'}{end}")
	for _, line := range strings.Split(crb, "\n") {
		if strings.Contains(line, "apps/dev") {
			return fmt.Errorf("do not use ClusterRoleBindings for this task (found binding involving apps/dev)")
		}
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	content, err := dockerExec(ctx, node, "cat", "/opt/CKA/rbac-check.txt")
	if err != nil {
		return fmt.Errorf("/opt/CKA/rbac-check.txt missing: %w", err)
	}
	if strings.TrimSpace(content) != "yes" {
		return fmt.Errorf("/opt/CKA/rbac-check.txt must contain exactly %q (got %q)", "yes", strings.TrimSpace(content))
	}

	return assertNoDebugPods(ctx, kubeconfigPath, "apps")
}

func (l *AppsDeveloperAccessLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Create the ServiceAccount",
			Command:     "kubectl create serviceaccount dev -n apps",
		},
		{
			Description: "Create the Role (imperative)",
			Command: `kubectl create role dev-role -n apps \
  --verb=create,list,watch \
  --resource=pods,deployments`,
			Notes: "Docs search: kubernetes.io Using RBAC Authorization",
		},
		{
			Description: "Bind the Role to the ServiceAccount",
			Command: `kubectl create rolebinding dev-rolebinding -n apps \
  --role=dev-role \
  --serviceaccount=apps:dev`,
		},
		{
			Description: "Verify permissions",
			Command:     "kubectl auth can-i create deployments -n apps --as=system:serviceaccount:apps:dev",
		},
		{
			Description: "Write the artifact on the control plane",
			Command: `kubectl auth can-i create deployments -n apps --as=system:serviceaccount:apps:dev \
  | docker exec -i cka-lab-control-plane tee /opt/CKA/rbac-check.txt`,
			Notes: "Exam: ssh to the control-plane node, sudo -i, then write /opt/CKA/rbac-check.txt. kind equivalent uses docker exec.",
		},
	}
}
