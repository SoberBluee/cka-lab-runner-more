package labs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&BackupPolicyIncompleteLab{})
}

type BackupPolicyIncompleteLab struct {
	BaseLab
}

func (l *BackupPolicyIncompleteLab) ID() string {
	return "backup_policy_incomplete"
}

func (l *BackupPolicyIncompleteLab) Title() string {
	return "Stored Object No Longer Passes Validation"
}

func (l *BackupPolicyIncompleteLab) Category() Category {
	return CategoryWorkloads
}

func (l *BackupPolicyIncompleteLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *BackupPolicyIncompleteLab) Description() string {
	return `An extension in this cluster manages BackupPolicy objects. Its schema was tightened
last week and one object created before that change no longer satisfies it: re-applying
'nightly' in namespace 'ops' is rejected, and the controller logs complain about a missing
field.

Your task:
  1. Bring the existing 'nightly' BackupPolicy up to the current schema, keeping the last
     7 copies.
  2. Add a second policy named 'weekly' with schedule "0 3 * * 0" keeping the last 4 copies.

Everything you need to know about the accepted fields is discoverable from the cluster.`
}

func (l *BackupPolicyIncompleteLab) Hints() []string {
	return []string{
		"kubectl get crd and kubectl api-resources --api-group=ops.cka.local tell you what this kind is called and where it lives",
		"kubectl explain backuppolicy.spec --recursive lists the fields, their types and which are required",
		"kubectl get crd backuppolicies.ops.cka.local -o yaml shows the validation rules including required fields and value limits",
		"Objects created before a schema tightened are not revalidated, which is why the old one still exists but cannot be re-applied",
		"kubectl patch backuppolicy nightly -n ops --type merge -p '{\"spec\":{...}}' adds the missing field in place",
	}
}

func (l *BackupPolicyIncompleteLab) EstimatedTime() int {
	return 20
}

func (l *BackupPolicyIncompleteLab) Tags() []string {
	return []string{"api-extensions", "custom-resources", "validation"}
}

func (l *BackupPolicyIncompleteLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

// backupPolicyCRD renders the extension's definition. The lab installs the loose
// variant, creates an object against it, then installs the strict variant so the
// stored object is left behind by a schema that has moved on.
func backupPolicyCRD(strict bool) string {
	requiredFields := "            - schedule"
	keepLastSchema := "                type: integer"
	if strict {
		requiredFields = "            - schedule\n            - keepLast"
		keepLastSchema = "                type: integer\n                minimum: 1\n                maximum: 30"
	}

	return `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: backuppolicies.ops.cka.local
spec:
  group: ops.cka.local
  scope: Namespaced
  names:
    plural: backuppolicies
    singular: backuppolicy
    kind: BackupPolicy
    shortNames:
    - bp
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required:
` + requiredFields + `
            properties:
              schedule:
                type: string
              keepLast:
` + keepLastSchema + `
    additionalPrinterColumns:
    - name: Schedule
      type: string
      jsonPath: .spec.schedule
    - name: Keep
      type: integer
      jsonPath: .spec.keepLast
`
}

func (l *BackupPolicyIncompleteLab) Break(ctx context.Context, kubeconfigPath string) error {
	if err := kubectlApply(ctx, kubeconfigPath, backupPolicyCRD(false)); err != nil {
		return fmt.Errorf("installing backup CRD: %w", err)
	}

	legacyObject := `apiVersion: v1
kind: Namespace
metadata:
  name: ops
---
apiVersion: ops.cka.local/v1
kind: BackupPolicy
metadata:
  name: nightly
  namespace: ops
spec:
  schedule: "0 1 * * *"
`
	if err := waitFor(ctx, 60*time.Second, func() error {
		return kubectlApply(ctx, kubeconfigPath, legacyObject)
	}); err != nil {
		return fmt.Errorf("creating the legacy backup policy: %w", err)
	}

	// Tighten the schema after the object exists — Kubernetes does not
	// revalidate stored objects, which is what leaves 'nightly' non-compliant.
	if err := kubectlApply(ctx, kubeconfigPath, backupPolicyCRD(true)); err != nil {
		return fmt.Errorf("tightening backup CRD schema: %w", err)
	}

	_, _ = kubectl(ctx, kubeconfigPath, "delete", "backuppolicy", "weekly",
		"-n", "ops", "--ignore-not-found=true")
	return nil
}

func (l *BackupPolicyIncompleteLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	keep, err := kubectl(ctx, kubeconfigPath, "get", "backuppolicies.ops.cka.local", "nightly",
		"-n", "ops", "-o", "jsonpath={.spec.keepLast}")
	if err != nil {
		return fmt.Errorf("nightly backup policy missing: %w", err)
	}
	if strings.TrimSpace(keep) != "" {
		return fmt.Errorf("nightly already has keepLast set")
	}
	return nil
}

func (l *BackupPolicyIncompleteLab) Verify(ctx context.Context, kubeconfigPath string) error {
	checks := []struct {
		name         string
		wantSchedule string
		wantKeep     int
	}{
		{"nightly", "0 1 * * *", 7},
		{"weekly", "0 3 * * 0", 4},
	}

	for _, check := range checks {
		schedule, err := kubectl(ctx, kubeconfigPath, "get", "backuppolicies.ops.cka.local", check.name,
			"-n", "ops", "-o", "jsonpath={.spec.schedule}")
		if err != nil {
			return fmt.Errorf("no BackupPolicy named %s in namespace ops: %w", check.name, err)
		}
		if strings.TrimSpace(schedule) != check.wantSchedule {
			return fmt.Errorf("%s has schedule %q, expected %q", check.name, strings.TrimSpace(schedule), check.wantSchedule)
		}

		keepOutput, err := kubectl(ctx, kubeconfigPath, "get", "backuppolicies.ops.cka.local", check.name,
			"-n", "ops", "-o", "jsonpath={.spec.keepLast}")
		if err != nil {
			return fmt.Errorf("reading keepLast on %s: %w", check.name, err)
		}
		keep, convErr := strconv.Atoi(strings.TrimSpace(keepOutput))
		if convErr != nil {
			return fmt.Errorf("%s has no valid keepLast value (%q)", check.name, strings.TrimSpace(keepOutput))
		}
		if keep != check.wantKeep {
			return fmt.Errorf("%s keeps %d copies, expected %d", check.name, keep, check.wantKeep)
		}
	}

	return nil
}

func (l *BackupPolicyIncompleteLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Identify the extension and where its objects live",
			Command:     "kubectl get crd | grep ops.cka.local; kubectl api-resources --api-group=ops.cka.local",
			Notes:       "BackupPolicy, short name bp, namespaced, version v1",
		},
		{
			Description: "Look at the object that no longer validates",
			Command:     "kubectl get backuppolicies -n ops -o yaml",
			Notes:       "nightly has a schedule but no keepLast",
		},
		{
			Description: "Read the current schema",
			Command:     "kubectl explain backuppolicy.spec --recursive; kubectl get crd backuppolicies.ops.cka.local -o jsonpath='{.spec.versions[0].schema.openAPIV3Schema.properties.spec}'",
			Notes:       "keepLast is now required and must be between 1 and 30",
		},
		{
			Description: "Bring the existing object up to the schema",
			Command:     `kubectl patch backuppolicy nightly -n ops --type merge -p '{"spec":{"keepLast":7}}'`,
		},
		{
			Description: "Create the second policy",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: ops.cka.local/v1
kind: BackupPolicy
metadata:
  name: weekly
  namespace: ops
spec:
  schedule: "0 3 * * 0"
  keepLast: 4
EOF`,
		},
		{
			Description: "Confirm both policies",
			Command:     "kubectl get bp -n ops",
			Notes:       "The printer columns show Schedule and Keep for each object",
		},
	}
}
