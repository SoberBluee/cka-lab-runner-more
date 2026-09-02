package labs

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(&RetentionObjectRejectedLab{})
}

type RetentionObjectRejectedLab struct {
	BaseLab
}

func (l *RetentionObjectRejectedLab) ID() string {
	return "retention_object_rejected"
}

func (l *RetentionObjectRejectedLab) Title() string {
	return "Team Manifest Rejected As Unknown Kind"
}

func (l *RetentionObjectRejectedLab) Category() Category {
	return CategoryWorkloads
}

func (l *RetentionObjectRejectedLab) Difficulty() Difficulty {
	return DifficultyMedium
}

func (l *RetentionObjectRejectedLab) Description() string {
	return `The data team cannot apply their manifest. Their exact command fails with:

  error: unable to recognize "policy.yaml": no matches for kind "RetentionPolicy"
  in version "ops.cka.local/v1"

They insist the extension is installed, and they are right — the cluster does know about
this kind. A copy of their manifest is stored in the ConfigMap 'retention-manifest' in
namespace 'ops'.

Your task: get a RetentionPolicy named 'default-retention' created in namespace 'ops' with
retentionDays 30 and target "archive". The API server must accept it — do not fake it with
a ConfigMap.`
}

func (l *RetentionObjectRejectedLab) Hints() []string {
	return []string{
		"kubectl get crd — the extension is installed, so the mismatch is in what the manifest asks for",
		"kubectl api-resources | grep -i retention shows which version the API currently offers",
		"kubectl get crd retentionpolicies.ops.cka.local -o yaml — each entry under spec.versions has served and storage flags",
		"A version that is not served cannot be used, even though it is defined",
		"Two valid fixes: use the version the API serves, or flip served: true on the version the manifest wants",
		"kubectl explain retentionpolicy.spec --api-version=<served-version> shows the fields the object accepts",
	}
}

func (l *RetentionObjectRejectedLab) EstimatedTime() int {
	return 20
}

func (l *RetentionObjectRejectedLab) Tags() []string {
	return []string{"api-extensions", "custom-resources", "versions", "troubleshooting"}
}

func (l *RetentionObjectRejectedLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *RetentionObjectRejectedLab) Break(ctx context.Context, kubeconfigPath string) error {
	crd := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: retentionpolicies.ops.cka.local
spec:
  group: ops.cka.local
  scope: Namespaced
  names:
    plural: retentionpolicies
    singular: retentionpolicy
    kind: RetentionPolicy
    shortNames:
    - rp
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required:
            - retentionDays
            - target
            properties:
              retentionDays:
                type: integer
                minimum: 1
              target:
                type: string
    additionalPrinterColumns:
    - name: Days
      type: integer
      jsonPath: .spec.retentionDays
    - name: Target
      type: string
      jsonPath: .spec.target
  - name: v1
    served: false
    storage: false
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required:
            - retentionDays
            - target
            properties:
              retentionDays:
                type: integer
                minimum: 1
              target:
                type: string
`
	if err := kubectlApply(ctx, kubeconfigPath, crd); err != nil {
		return fmt.Errorf("installing retention CRD: %w", err)
	}

	teamManifest := `apiVersion: v1
kind: Namespace
metadata:
  name: ops
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: retention-manifest
  namespace: ops
data:
  policy.yaml: |
    apiVersion: ops.cka.local/v1
    kind: RetentionPolicy
    metadata:
      name: default-retention
      namespace: ops
    spec:
      retentionDays: 30
      target: archive
`
	if err := kubectlApply(ctx, kubeconfigPath, teamManifest); err != nil {
		return fmt.Errorf("storing team manifest: %w", err)
	}

	_, _ = kubectl(ctx, kubeconfigPath, "delete", "retentionpolicy", "default-retention",
		"-n", "ops", "--ignore-not-found=true")
	return nil
}

func (l *RetentionObjectRejectedLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	if _, err := kubectl(ctx, kubeconfigPath, "get", "retentionpolicies.ops.cka.local",
		"default-retention", "-n", "ops"); err == nil {
		return fmt.Errorf("the RetentionPolicy already exists")
	}
	return nil
}

func (l *RetentionObjectRejectedLab) Verify(ctx context.Context, kubeconfigPath string) error {
	days, err := kubectl(ctx, kubeconfigPath, "get", "retentionpolicies.ops.cka.local",
		"default-retention", "-n", "ops", "-o", "jsonpath={.spec.retentionDays}")
	if err != nil {
		return fmt.Errorf("no RetentionPolicy named default-retention in namespace ops: %w", err)
	}
	if strings.TrimSpace(days) != "30" {
		return fmt.Errorf("default-retention has retentionDays %q, expected 30", strings.TrimSpace(days))
	}

	target, err := kubectl(ctx, kubeconfigPath, "get", "retentionpolicies.ops.cka.local",
		"default-retention", "-n", "ops", "-o", "jsonpath={.spec.target}")
	if err != nil {
		return fmt.Errorf("reading default-retention target: %w", err)
	}
	if strings.TrimSpace(target) != "archive" {
		return fmt.Errorf("default-retention has target %q, expected archive", strings.TrimSpace(target))
	}

	return nil
}

func (l *RetentionObjectRejectedLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Read the manifest the team tried to apply",
			Command:     "kubectl get cm retention-manifest -n ops -o jsonpath='{.data.policy\\.yaml}'",
		},
		{
			Description: "Confirm the extension is installed and see what the API offers",
			Command:     "kubectl get crd | grep retention; kubectl api-resources --api-group=ops.cka.local",
			Notes:       "The CRD exists, but api-resources reports v1alpha1 — not the v1 the manifest asks for",
		},
		{
			Description: "Inspect the defined versions",
			Command:     "kubectl get crd retentionpolicies.ops.cka.local -o jsonpath='{range .spec.versions[*]}{.name}{\" served=\"}{.served}{\" storage=\"}{.storage}{\"\\n\"}{end}'",
			Notes:       "v1 is defined but served: false, so nothing can be created with it",
		},
		{
			Description: "Check the fields the object needs",
			Command:     "kubectl explain retentionpolicy.spec --api-version=ops.cka.local/v1alpha1",
		},
		{
			Description: "Fix option A: create it with the version the API serves",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: ops.cka.local/v1alpha1
kind: RetentionPolicy
metadata:
  name: default-retention
  namespace: ops
spec:
  retentionDays: 30
  target: archive
EOF`,
		},
		{
			Description: "Fix option B: serve the version the manifest uses",
			Command:     `kubectl patch crd retentionpolicies.ops.cka.local --type json -p='[{"op":"replace","path":"/spec/versions/1/served","value":true}]'`,
			Notes:       "Then the team's original manifest applies unchanged",
		},
		{
			Description: "Confirm the object exists",
			Command:     "kubectl get retentionpolicies -n ops",
			Notes:       "The printer columns show Days and Target",
		},
	}
}
