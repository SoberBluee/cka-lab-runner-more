package labs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&BillingValuesDriftLab{})
}

// BillingValuesDriftLab — mix: diagnose bad Helm values, upgrade with a values
// file, and leave an artifact documenting the corrected config.
type BillingValuesDriftLab struct {
	BaseLab
}

func (l *BillingValuesDriftLab) ID() string { return "billing_values_drift" }

func (l *BillingValuesDriftLab) Title() string {
	return "Billing Frontend Misconfigured After Upgrade"
}

func (l *BillingValuesDriftLab) Category() Category { return CategoryWorkloads }

func (l *BillingValuesDriftLab) Difficulty() Difficulty { return DifficultyHard }

func (l *BillingValuesDriftLab) EstimatedTime() int { return 12 }

func (l *BillingValuesDriftLab) Tags() []string {
	return []string{"mixed", "helm", "upgrade", "values"}
}

func (l *BillingValuesDriftLab) Hints() []string { return nil }

func (l *BillingValuesDriftLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace billing

[Weight: 8%] | Time limit: 8–12 minutes

Context:
Helm release billing-web in namespace billing was scaled up, but clients calling the
Service on port 80 now fail. Pods may look healthy while the Service does not match
what the containers expose.

Task:
1. Inspect the release (history, values, and live Service/Deployment) and correct the
   configuration with Helm so that:
   - replicaCount is 3
   - service.port is 80
   - image remains nginx:alpine
   - service.type remains ClusterIP
2. Keep the release name billing-web (do not uninstall).
3. Persist the corrected values you applied as a YAML file at
   /opt/CKA/billing-values.yaml containing at least:
     replicaCount: 3
     service:
       port: 80
4. Write the new Helm revision number (digits only) to /opt/CKA/billing-revision.txt.
5. Delete any temporary debug Pods you created in namespace billing.

Constraints:
- Work only in context cka-lab and namespace billing (wrong placement = 0).
- Do not uninstall billing-web.
- Do not change the release name.
- Prefer helm upgrade (with --set and/or -f) over kubectl edit of the live objects.
- Resource names and field values must match exactly (case-sensitive).
- Do not leave temporary debug pods, Jobs, or test resources behind.`
}

func (l *BillingValuesDriftLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return requireHelm(ctx)
}

func (l *BillingValuesDriftLab) Break(ctx context.Context, kubeconfigPath string) error {
	chartDir, err := syncChartToExamPaths(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	node, err := ensureOptCKA(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/billing-values.yaml", "/opt/CKA/billing-revision.txt")

	ns := `apiVersion: v1
kind: Namespace
metadata:
  name: billing
`
	if err := kubectlApply(ctx, kubeconfigPath, ns); err != nil {
		return err
	}
	_, _ = helm(ctx, kubeconfigPath, "uninstall", "billing-web", "-n", "billing")

	if out, err := helm(ctx, kubeconfigPath, "install", "billing-web", chartDir,
		"-n", "billing",
		"--set", "replicaCount=1",
		"--set", "image.repository=nginx",
		"--set", "image.tag=alpine",
		"--set", "service.type=ClusterIP",
		"--set", "service.port=80",
		"--wait", "--timeout", "120s"); err != nil {
		return fmt.Errorf("helm install billing-web: %s: %w", out, err)
	}

	// Drifted upgrade: scale up but point Service at the wrong port
	if out, err := helm(ctx, kubeconfigPath, "upgrade", "billing-web", chartDir,
		"-n", "billing",
		"--set", "replicaCount=3",
		"--set", "image.repository=nginx",
		"--set", "image.tag=alpine",
		"--set", "service.type=ClusterIP",
		"--set", "service.port=9999",
		"--wait", "--timeout", "120s"); err != nil {
		return fmt.Errorf("helm upgrade billing-web (drift): %s: %w", out, err)
	}
	return nil
}

func (l *BillingValuesDriftLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	port, err := kubectl(ctx, kubeconfigPath, "get", "svc", "-n", "billing",
		"-l", "app.kubernetes.io/instance=billing-web",
		"-o", "jsonpath={.items[0].spec.ports[0].port}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(port) != "9999" {
		return fmt.Errorf("expected drifted service port 9999, got %q", strings.TrimSpace(port))
	}
	return nil
}

func (l *BillingValuesDriftLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if _, err := helm(ctx, kubeconfigPath, "status", "billing-web", "-n", "billing"); err != nil {
		return fmt.Errorf("release billing-web missing — do not uninstall: %w", err)
	}

	rev, err := helmReleaseRevision(ctx, kubeconfigPath, "billing-web", "billing")
	if err != nil {
		return err
	}
	if rev < 3 {
		return fmt.Errorf("expected a corrective helm upgrade (revision >= 3, got %d)", rev)
	}

	replicas, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "billing",
		"-l", "app.kubernetes.io/instance=billing-web",
		"-o", "jsonpath={.items[0].spec.replicas}")
	if err != nil {
		return fmt.Errorf("billing-web Deployment not found: %w", err)
	}
	if strings.TrimSpace(replicas) != "3" {
		return fmt.Errorf("replicaCount must be 3 (got %q)", strings.TrimSpace(replicas))
	}

	image, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "billing",
		"-l", "app.kubernetes.io/instance=billing-web",
		"-o", "jsonpath={.items[0].spec.template.spec.containers[0].image}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(image) != "nginx:alpine" {
		return fmt.Errorf("image must remain nginx:alpine (got %q)", strings.TrimSpace(image))
	}

	port, err := kubectl(ctx, kubeconfigPath, "get", "svc", "-n", "billing",
		"-l", "app.kubernetes.io/instance=billing-web",
		"-o", "jsonpath={.items[0].spec.ports[0].port}")
	if err != nil {
		return fmt.Errorf("billing-web Service not found: %w", err)
	}
	if strings.TrimSpace(port) != "80" {
		return fmt.Errorf("service.port must be 80 (got %q)", strings.TrimSpace(port))
	}

	svcType, err := kubectl(ctx, kubeconfigPath, "get", "svc", "-n", "billing",
		"-l", "app.kubernetes.io/instance=billing-web",
		"-o", "jsonpath={.items[0].spec.type}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(svcType) != "ClusterIP" {
		return fmt.Errorf("service.type must remain ClusterIP (got %q)", strings.TrimSpace(svcType))
	}

	// Reject pure kubectl-edit fixes that left Helm values wrong
	vals, err := helm(ctx, kubeconfigPath, "get", "values", "billing-web", "-n", "billing", "--all")
	if err != nil {
		return fmt.Errorf("helm get values failed: %w", err)
	}
	if !strings.Contains(vals, "port: 80") && !strings.Contains(vals, "port:80") {
		return fmt.Errorf("helm release values still do not show service.port 80 — fix via helm upgrade")
	}

	if err := waitFor(ctx, 90*time.Second, func() error {
		ready, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "billing",
			"-l", "app.kubernetes.io/instance=billing-web",
			"-o", "jsonpath={.items[0].status.readyReplicas}")
		if err != nil {
			return err
		}
		n, _ := strconv.Atoi(strings.TrimSpace(ready))
		if n < 3 {
			return fmt.Errorf("ready replicas %d, want 3", n)
		}
		return nil
	}); err != nil {
		return err
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	valuesFile, err := dockerExec(ctx, node, "cat", "/opt/CKA/billing-values.yaml")
	if err != nil {
		return fmt.Errorf("/opt/CKA/billing-values.yaml missing: %w", err)
	}
	if !strings.Contains(valuesFile, "replicaCount: 3") && !strings.Contains(valuesFile, "replicaCount:3") {
		return fmt.Errorf("/opt/CKA/billing-values.yaml must set replicaCount: 3")
	}
	if !strings.Contains(valuesFile, "port: 80") && !strings.Contains(valuesFile, "port:80") {
		return fmt.Errorf("/opt/CKA/billing-values.yaml must set service.port: 80")
	}

	revFile, err := dockerExec(ctx, node, "cat", "/opt/CKA/billing-revision.txt")
	if err != nil {
		return fmt.Errorf("/opt/CKA/billing-revision.txt missing: %w", err)
	}
	got, convErr := strconv.Atoi(strings.TrimSpace(revFile))
	if convErr != nil || got != rev {
		return fmt.Errorf("/opt/CKA/billing-revision.txt is %q, release revision is %d", strings.TrimSpace(revFile), rev)
	}

	return assertNoDebugPods(ctx, kubeconfigPath, "billing")
}

func (l *BillingValuesDriftLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Compare Helm values to the live Service",
			Command:     "helm get values billing-web -n billing --all; kubectl get svc -n billing -l app.kubernetes.io/instance=billing-web -o yaml",
			Notes:       "Docs search: helm.sh docs helm_get_values / helm_upgrade",
		},
		{
			Description: "Write a values file and upgrade the release",
			Command: `cat > /tmp/billing-values.yaml <<'EOF'
replicaCount: 3
image:
  repository: nginx
  tag: alpine
service:
  type: ClusterIP
  port: 80
EOF
helm upgrade billing-web /tmp/opt/CKA/charts/cka-webapp -n billing -f /tmp/billing-values.yaml --wait`,
		},
		{
			Description: "Copy artifacts to the control plane",
			Command: `REV=$(helm status billing-web -n billing -o json | sed -n 's/.*"revision":\([0-9]*\).*/\1/p' | head -1)
docker cp /tmp/billing-values.yaml cka-lab-control-plane:/opt/CKA/billing-values.yaml
docker exec cka-lab-control-plane bash -c "echo -n $REV > /opt/CKA/billing-revision.txt"`,
			Notes: "Exam: create /opt/CKA/billing-values.yaml on the node with ssh/sudo; kind uses docker cp/exec",
		},
	}
}
