package labs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&WebshopReleaseBrokenLab{})
}

// WebshopReleaseBrokenLab — troubleshooting: bad Helm upgrade; rollback required.
type WebshopReleaseBrokenLab struct {
	BaseLab
}

func (l *WebshopReleaseBrokenLab) ID() string { return "webshop_release_broken" }

func (l *WebshopReleaseBrokenLab) Title() string {
	return "Webshop Release No Longer Serves Traffic"
}

func (l *WebshopReleaseBrokenLab) Category() Category { return CategoryWorkloads }

func (l *WebshopReleaseBrokenLab) Difficulty() Difficulty { return DifficultyMedium }

func (l *WebshopReleaseBrokenLab) EstimatedTime() int { return 10 }

func (l *WebshopReleaseBrokenLab) Tags() []string {
	return []string{"troubleshooting", "helm", "rollback"}
}

func (l *WebshopReleaseBrokenLab) Hints() []string { return nil }

func (l *WebshopReleaseBrokenLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace storefront

[Weight: 7%] | Time limit: 7–10 minutes

Context:
The Helm release webshop in namespace storefront was upgraded a few minutes ago.
Since then the webshop pods never become Ready and customers cannot reach the site.

Task:
1. Restore the webshop release so its pods are Ready again and the Service answers on port 80.
2. Keep the Helm release name webshop (do not uninstall and reinstall under a new name).
3. Write the current Helm revision number for release webshop (digits only) to
   /opt/CKA/webshop-revision.txt.
4. Delete any temporary debug Pods you created in namespace storefront.

Constraints:
- Work only in context cka-lab and namespace storefront (wrong placement = 0).
- Do not uninstall the webshop release.
- Do not change the release name.
- Prefer restoring the last known-good Helm revision rather than manually editing
  Deployment fields outside Helm.
- Resource names must match exactly (case-sensitive).
- Do not leave temporary debug pods, Jobs, or test resources behind.`
}

func (l *WebshopReleaseBrokenLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return requireHelm(ctx)
}

func (l *WebshopReleaseBrokenLab) Break(ctx context.Context, kubeconfigPath string) error {
	chartDir, err := syncChartToExamPaths(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	node, err := ensureOptCKA(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/webshop-revision.txt")

	ns := `apiVersion: v1
kind: Namespace
metadata:
  name: storefront
`
	if err := kubectlApply(ctx, kubeconfigPath, ns); err != nil {
		return err
	}

	_, _ = helm(ctx, kubeconfigPath, "uninstall", "webshop", "-n", "storefront")

	if out, err := helm(ctx, kubeconfigPath, "install", "webshop", chartDir,
		"-n", "storefront",
		"--set", "replicaCount=2",
		"--set", "image.repository=nginx",
		"--set", "image.tag=alpine",
		"--set", "service.port=80",
		"--wait", "--timeout", "120s"); err != nil {
		return fmt.Errorf("helm install webshop: %s: %w", out, err)
	}

	// Bad upgrade — non-existent image tag (revision 2)
	if out, err := helm(ctx, kubeconfigPath, "upgrade", "webshop", chartDir,
		"-n", "storefront",
		"--set", "replicaCount=2",
		"--set", "image.repository=nginx",
		"--set", "image.tag=not-a-real-tag-cka-lab",
		"--set", "service.port=80"); err != nil {
		// upgrade may succeed as a release even if pods fail to pull
		_ = out
	}

	return nil
}

func (l *WebshopReleaseBrokenLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	return waitFor(ctx, 90*time.Second, func() error {
		ready, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "storefront",
			"-l", "app.kubernetes.io/instance=webshop",
			"-o", "jsonpath={.items[0].status.readyReplicas}")
		if err == nil && strings.TrimSpace(ready) == "2" {
			return fmt.Errorf("webshop still shows ready replicas — bad upgrade did not take effect")
		}
		rev, err := helmReleaseRevision(ctx, kubeconfigPath, "webshop", "storefront")
		if err != nil {
			return err
		}
		if rev < 2 {
			return fmt.Errorf("expected helm revision >= 2 after bad upgrade, got %d", rev)
		}
		return nil
	})
}

func (l *WebshopReleaseBrokenLab) Verify(ctx context.Context, kubeconfigPath string) error {
	// Release must still exist (no uninstall)
	if _, err := helm(ctx, kubeconfigPath, "status", "webshop", "-n", "storefront"); err != nil {
		return fmt.Errorf("release webshop missing — do not uninstall: %w", err)
	}

	rev, err := helmReleaseRevision(ctx, kubeconfigPath, "webshop", "storefront")
	if err != nil {
		return err
	}
	if rev < 3 {
		return fmt.Errorf("expected a new Helm revision after rollback (got %d; want >= 3)", rev)
	}

	image, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "storefront",
		"-l", "app.kubernetes.io/instance=webshop",
		"-o", "jsonpath={.items[0].spec.template.spec.containers[0].image}")
	if err != nil {
		return fmt.Errorf("webshop Deployment not found: %w", err)
	}
	if strings.TrimSpace(image) != "nginx:alpine" {
		return fmt.Errorf("webshop image must be restored to nginx:alpine (got %q)", strings.TrimSpace(image))
	}

	port, err := kubectl(ctx, kubeconfigPath, "get", "svc", "-n", "storefront",
		"-l", "app.kubernetes.io/instance=webshop",
		"-o", "jsonpath={.items[0].spec.ports[0].port}")
	if err != nil {
		return fmt.Errorf("webshop Service not found: %w", err)
	}
	if strings.TrimSpace(port) != "80" {
		return fmt.Errorf("webshop Service port must remain 80 (got %q)", strings.TrimSpace(port))
	}

	if err := waitFor(ctx, 120*time.Second, func() error {
		ready, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "storefront",
			"-l", "app.kubernetes.io/instance=webshop",
			"-o", "jsonpath={.status.readyReplicas}")
		if err != nil {
			ready, err = kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "storefront",
				"-l", "app.kubernetes.io/instance=webshop",
				"-o", "jsonpath={.items[0].status.readyReplicas}")
		}
		if err != nil {
			return err
		}
		n, _ := strconv.Atoi(strings.TrimSpace(ready))
		if n < 2 {
			return fmt.Errorf("webshop ready replicas %d, want 2", n)
		}
		return nil
	}); err != nil {
		return err
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	content, err := dockerExec(ctx, node, "cat", "/opt/CKA/webshop-revision.txt")
	if err != nil {
		return fmt.Errorf("/opt/CKA/webshop-revision.txt missing: %w", err)
	}
	gotRev, convErr := strconv.Atoi(strings.TrimSpace(content))
	if convErr != nil || gotRev != rev {
		return fmt.Errorf("/opt/CKA/webshop-revision.txt is %q, current revision is %d", strings.TrimSpace(content), rev)
	}

	return assertNoDebugPods(ctx, kubeconfigPath, "storefront")
}

func (l *WebshopReleaseBrokenLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Inspect the Helm release history and workload status",
			Command:     "helm history webshop -n storefront; kubectl get pods -n storefront -l app.kubernetes.io/instance=webshop",
			Notes:       "Docs search: helm.sh docs helm_history / helm_rollback",
		},
		{
			Description: "Roll back to the last good revision (usually 1)",
			Command:     "helm rollback webshop 1 -n storefront",
		},
		{
			Description: "Confirm pods recover and record the new revision",
			Command: `helm status webshop -n storefront
REV=$(helm status webshop -n storefront -o json | sed -n 's/.*"revision":\([0-9]*\).*/\1/p' | head -1)
# Exam: ssh control-plane && sudo -i && echo -n "$REV" > /opt/CKA/webshop-revision.txt
docker exec cka-lab-control-plane bash -c "echo -n $REV > /opt/CKA/webshop-revision.txt"`,
			Notes: "Exam uses ssh + sudo -i; kind equivalent is docker exec into the control-plane node",
		},
	}
}
