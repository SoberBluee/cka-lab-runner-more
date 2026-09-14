package labs

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(&SecretMountReadyLab{})
}

type SecretMountReadyLab struct {
	BaseLab
}

func (l *SecretMountReadyLab) ID() string             { return "secret_mount_ready" }
func (l *SecretMountReadyLab) Title() string          { return "Secret Mount Not Ready" }
func (l *SecretMountReadyLab) Category() Category     { return CategoryWorkloads }
func (l *SecretMountReadyLab) Difficulty() Difficulty { return DifficultyMedium }
func (l *SecretMountReadyLab) EstimatedTime() int     { return 10 }
func (l *SecretMountReadyLab) Hints() []string        { return nil }
func (l *SecretMountReadyLab) Tags() []string {
	return []string{"creation", "secrets", "volumes", "pods"}
}

func (l *SecretMountReadyLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace admin-secrets

[Weight: 20%] | Time limit: 6–10 minutes

Context:
Namespace admin-secrets already contains a Secret named dotfile-secret.

Task:
1. Create a Pod called secret-practice in namespace admin-secrets using image busybox.
2. The container within the Pod must be named secret-admin and must sleep for 4800 seconds.
3. Mount a read-only secret volume named secret-volume at path /etc/secret-volume.
4. The volume must source Secret dotfile-secret.

Constraints:
- Exact names matter (Pod, container, volume, Secret, mount path).
- The volume mount must be read-only.
- Do not leave temporary debug Pods behind.
`
}

func (l *SecretMountReadyLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *SecretMountReadyLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: admin-secrets
---
apiVersion: v1
kind: Secret
metadata:
  name: dotfile-secret
  namespace: admin-secrets
type: Opaque
stringData:
  .secret-file: "practice-secret-value"
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating admin-secrets baseline: %w", err)
	}
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "pod", "secret-practice", "-n", "admin-secrets",
		"--ignore-not-found=true", "--wait=false")
	return nil
}

func (l *SecretMountReadyLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	if _, err := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets"); err == nil {
		return fmt.Errorf("secret-practice already exists")
	}
	return nil
}

func (l *SecretMountReadyLab) Verify(ctx context.Context, kubeconfigPath string) error {
	cname, err := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets",
		"-o", "jsonpath={.spec.containers[0].name}")
	if err != nil {
		return fmt.Errorf("Pod secret-practice not found: %w", err)
	}
	if strings.TrimSpace(cname) != "secret-admin" {
		return fmt.Errorf("container name is %q, want secret-admin", strings.TrimSpace(cname))
	}
	image, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets",
		"-o", "jsonpath={.spec.containers[0].image}")
	if !strings.Contains(strings.TrimSpace(image), "busybox") {
		return fmt.Errorf("image must be busybox (got %q)", strings.TrimSpace(image))
	}

	cmd, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets",
		"-o", "jsonpath={.spec.containers[0].command}{.spec.containers[0].args}")
	if !strings.Contains(cmd, "4800") {
		return fmt.Errorf("container must sleep for 4800 seconds")
	}

	secretName, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets",
		"-o", `jsonpath={.spec.volumes[?(@.name=="secret-volume")].secret.secretName}`)
	if strings.TrimSpace(secretName) != "dotfile-secret" {
		return fmt.Errorf("secret-volume must reference dotfile-secret (got %q)", strings.TrimSpace(secretName))
	}

	mount, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets",
		"-o", `jsonpath={.spec.containers[0].volumeMounts[?(@.name=="secret-volume")].mountPath}`)
	if strings.TrimSpace(mount) != "/etc/secret-volume" {
		return fmt.Errorf("mountPath must be /etc/secret-volume (got %q)", strings.TrimSpace(mount))
	}
	ro, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets",
		"-o", `jsonpath={.spec.containers[0].volumeMounts[?(@.name=="secret-volume")].readOnly}`)
	if strings.TrimSpace(ro) != "true" {
		return fmt.Errorf("secret-volume mount must be readOnly")
	}

	phase, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "secret-practice", "-n", "admin-secrets",
		"-o", "jsonpath={.status.phase}")
	if strings.TrimSpace(phase) != "Running" {
		return fmt.Errorf("Pod phase is %q, want Running", strings.TrimSpace(phase))
	}
	return nil
}

func (l *SecretMountReadyLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Generate a Pod manifest and add the secret volume",
			Command: `kubectl -n admin-secrets run secret-practice --image=busybox --dry-run=client -o yaml \
  --command -- sleep 4800 > /tmp/secret-practice.yaml
# Edit: container name secret-admin; add volumes/volumeMounts for secret-volume`,
			Notes: "Docs search: Secrets; Distribute Credentials Securely Using Secrets",
		},
		{
			Description: "Apply the completed Pod",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: secret-practice
  namespace: admin-secrets
spec:
  volumes:
  - name: secret-volume
    secret:
      secretName: dotfile-secret
  containers:
  - name: secret-admin
    image: busybox
    command: ["sleep", "4800"]
    volumeMounts:
    - name: secret-volume
      mountPath: /etc/secret-volume
      readOnly: true
EOF
kubectl get pod secret-practice -n admin-secrets`,
		},
	}
}
