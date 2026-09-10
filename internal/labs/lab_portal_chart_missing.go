package labs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&PortalChartMissingLab{})
}

// PortalChartMissingLab — creation: install a provided chart with exact values.
type PortalChartMissingLab struct {
	BaseLab
}

func (l *PortalChartMissingLab) ID() string { return "portal_chart_missing" }

func (l *PortalChartMissingLab) Title() string {
	return "Customer Portal Chart Not Deployed"
}

func (l *PortalChartMissingLab) Category() Category { return CategoryWorkloads }

func (l *PortalChartMissingLab) Difficulty() Difficulty { return DifficultyMedium }

func (l *PortalChartMissingLab) EstimatedTime() int { return 10 }

func (l *PortalChartMissingLab) Tags() []string {
	return []string{"creation", "helm", "install"}
}

func (l *PortalChartMissingLab) Hints() []string { return nil }

func (l *PortalChartMissingLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace portal

[Weight: 6%] | Time limit: 7–10 minutes

Context:
The platform team left a Helm chart for the customer portal on the exam node, but
nothing has been installed yet. Namespace portal already exists.

Task:
1. Using the chart at /opt/CKA/charts/cka-webapp, install a Helm release named
   portal-web in namespace portal with all of the following:
   - replicaCount=3
   - image.repository=nginx
   - image.tag=alpine
   - service.type=ClusterIP
   - service.port=8080
2. Wait until the release is deployed and its pods are Ready.
3. Write the release revision number (digits only) to /opt/CKA/portal-revision.txt.
4. Delete any temporary debug Pods you created in namespace portal.

Constraints:
- Work only in context cka-lab and namespace portal (wrong placement = 0).
- Release name must be exactly portal-web.
- Chart must be cka-webapp from /opt/CKA/charts/cka-webapp (local practice mirror:
  /tmp/opt/CKA/charts/cka-webapp if /opt/CKA is not writable on your workstation).
- Do not leave temporary debug pods, Jobs, or test resources behind.
- Resource names and values must match exactly (case-sensitive).`
}

func (l *PortalChartMissingLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return requireHelm(ctx)
}

func (l *PortalChartMissingLab) Break(ctx context.Context, kubeconfigPath string) error {
	if _, err := syncChartToExamPaths(ctx, kubeconfigPath); err != nil {
		return err
	}
	node, err := ensureOptCKA(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/portal-revision.txt")

	ns := `apiVersion: v1
kind: Namespace
metadata:
  name: portal
`
	if err := kubectlApply(ctx, kubeconfigPath, ns); err != nil {
		return err
	}
	_, _ = helm(ctx, kubeconfigPath, "uninstall", "portal-web", "-n", "portal")
	return nil
}

func (l *PortalChartMissingLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(2 * time.Second)
	if _, err := helm(ctx, kubeconfigPath, "status", "portal-web", "-n", "portal"); err == nil {
		return fmt.Errorf("portal-web release already exists")
	}
	return nil
}

func (l *PortalChartMissingLab) Verify(ctx context.Context, kubeconfigPath string) error {
	if _, err := helm(ctx, kubeconfigPath, "status", "portal-web", "-n", "portal"); err != nil {
		return fmt.Errorf("release portal-web not found in portal: %w", err)
	}

	chart, _ := helm(ctx, kubeconfigPath, "list", "-n", "portal", "-o", "json")
	if !strings.Contains(chart, "portal-web") {
		return fmt.Errorf("portal-web not listed in namespace portal")
	}
	if !strings.Contains(chart, "cka-webapp") {
		return fmt.Errorf("portal-web must be installed from chart cka-webapp")
	}

	replicas, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "portal",
		"-l", "app.kubernetes.io/instance=portal-web",
		"-o", "jsonpath={.items[0].spec.replicas}")
	if err != nil {
		return fmt.Errorf("portal-web Deployment not found: %w", err)
	}
	if strings.TrimSpace(replicas) != "3" {
		return fmt.Errorf("replicaCount must be 3 (got %q)", strings.TrimSpace(replicas))
	}

	image, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "portal",
		"-l", "app.kubernetes.io/instance=portal-web",
		"-o", "jsonpath={.items[0].spec.template.spec.containers[0].image}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(image) != "nginx:alpine" {
		return fmt.Errorf("image must be nginx:alpine (got %q)", strings.TrimSpace(image))
	}

	port, err := kubectl(ctx, kubeconfigPath, "get", "svc", "-n", "portal",
		"-l", "app.kubernetes.io/instance=portal-web",
		"-o", "jsonpath={.items[0].spec.ports[0].port}")
	if err != nil {
		return fmt.Errorf("portal-web Service not found: %w", err)
	}
	if strings.TrimSpace(port) != "8080" {
		return fmt.Errorf("service.port must be 8080 (got %q)", strings.TrimSpace(port))
	}

	svcType, err := kubectl(ctx, kubeconfigPath, "get", "svc", "-n", "portal",
		"-l", "app.kubernetes.io/instance=portal-web",
		"-o", "jsonpath={.items[0].spec.type}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(svcType) != "ClusterIP" {
		return fmt.Errorf("service.type must be ClusterIP (got %q)", strings.TrimSpace(svcType))
	}

	if err := waitFor(ctx, 120*time.Second, func() error {
		ready, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "portal",
			"-l", "app.kubernetes.io/instance=portal-web",
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

	rev, err := helmReleaseRevision(ctx, kubeconfigPath, "portal-web", "portal")
	if err != nil {
		return err
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	content, err := dockerExec(ctx, node, "cat", "/opt/CKA/portal-revision.txt")
	if err != nil {
		return fmt.Errorf("/opt/CKA/portal-revision.txt missing: %w", err)
	}
	got, convErr := strconv.Atoi(strings.TrimSpace(content))
	if convErr != nil || got != rev {
		return fmt.Errorf("/opt/CKA/portal-revision.txt is %q, release revision is %d", strings.TrimSpace(content), rev)
	}

	return assertNoDebugPods(ctx, kubeconfigPath, "portal")
}

func (l *PortalChartMissingLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm the chart is present",
			Command:     "ls /tmp/opt/CKA/charts/cka-webapp || docker exec cka-lab-control-plane ls /opt/CKA/charts/cka-webapp",
			Notes:       "Docs search: helm.sh docs helm_install. Exam path is /opt/CKA/charts/cka-webapp; local mirror is /tmp/opt/CKA/charts/cka-webapp",
		},
		{
			Description: "Install the release with the required values",
			Command: `helm install portal-web /tmp/opt/CKA/charts/cka-webapp -n portal \
  --set replicaCount=3 \
  --set image.repository=nginx \
  --set image.tag=alpine \
  --set service.type=ClusterIP \
  --set service.port=8080 \
  --wait`,
		},
		{
			Description: "Record the revision on the control plane",
			Command: `REV=$(helm status portal-web -n portal -o json | sed -n 's/.*"revision":\([0-9]*\).*/\1/p' | head -1)
docker exec cka-lab-control-plane bash -c "echo -n $REV > /opt/CKA/portal-revision.txt"`,
			Notes: "Exam: ssh to the control-plane node, sudo -i, write /opt/CKA/portal-revision.txt",
		},
	}
}
