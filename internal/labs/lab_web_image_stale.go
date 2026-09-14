package labs

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(&WebImageStaleLab{})
}

type WebImageStaleLab struct {
	BaseLab
}

func (l *WebImageStaleLab) ID() string             { return "web_image_stale" }
func (l *WebImageStaleLab) Title() string          { return "Web Image Stale" }
func (l *WebImageStaleLab) Category() Category     { return CategoryWorkloads }
func (l *WebImageStaleLab) Difficulty() Difficulty { return DifficultyMedium }
func (l *WebImageStaleLab) EstimatedTime() int     { return 12 }
func (l *WebImageStaleLab) Hints() []string        { return nil }
func (l *WebImageStaleLab) Tags() []string {
	return []string{"creation", "deployment", "rolling-update", "annotation"}
}

func (l *WebImageStaleLab) Description() string {
	return `Set the context before doing any work:

  kubectl config use-context cka-lab
  # Create resources in the default namespace unless stated otherwise

[Weight: 12%] | Time limit: 8–12 minutes

Context:
The web team needs a controlled nginx rollout recorded on the Deployment object.

Task:
1. Create a new Deployment called nginx-deploy with image nginx:1.16 and 1 replica.
2. Upgrade the Deployment to image nginx:1.17 using a rolling update.
3. Add the annotation message=Updated nginx image to 1.17 on the Deployment.

Constraints:
- Prefer kubectl apply for create and update.
- Resource name must be exactly nginx-deploy.
- Do not leave temporary debug Pods behind.
`
}

func (l *WebImageStaleLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *WebImageStaleLab) Break(ctx context.Context, kubeconfigPath string) error {
	_, _ = kubectl(ctx, kubeconfigPath, "delete", "deploy", "nginx-deploy",
		"--ignore-not-found=true", "--wait=false")
	return nil
}

func (l *WebImageStaleLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	if _, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "nginx-deploy"); err == nil {
		return fmt.Errorf("nginx-deploy already exists")
	}
	return nil
}

func (l *WebImageStaleLab) Verify(ctx context.Context, kubeconfigPath string) error {
	image, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "nginx-deploy",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	if err != nil {
		return fmt.Errorf("nginx-deploy not found: %w", err)
	}
	if strings.TrimSpace(image) != "nginx:1.17" {
		return fmt.Errorf("image is %q, want nginx:1.17", strings.TrimSpace(image))
	}

	genStr, _ := kubectl(ctx, kubeconfigPath, "get", "deploy", "nginx-deploy",
		"-o", "jsonpath={.metadata.generation}")
	gen := 0
	fmt.Sscanf(strings.TrimSpace(genStr), "%d", &gen)
	if gen < 2 {
		return fmt.Errorf("expected a rolling update after create (generation >= 2, got %s)", genStr)
	}

	msg, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "nginx-deploy",
		"-o", "jsonpath={.metadata.annotations.message}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(msg) != "Updated nginx image to 1.17" {
		return fmt.Errorf("annotation message is %q, want %q",
			strings.TrimSpace(msg), "Updated nginx image to 1.17")
	}
	return nil
}

func (l *WebImageStaleLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Create nginx-deploy at 1.16 via apply",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-deploy
  template:
    metadata:
      labels:
        app: nginx-deploy
    spec:
      containers:
      - name: nginx
        image: nginx:1.16
EOF`,
			Notes: "Docs search: Performing a Rolling Update",
		},
		{
			Description: "Apply the 1.17 image upgrade",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx-deploy
  template:
    metadata:
      labels:
        app: nginx-deploy
    spec:
      containers:
      - name: nginx
        image: nginx:1.17
EOF
kubectl rollout status deploy/nginx-deploy`,
		},
		{
			Description: "Add the required annotation",
			Command:     `kubectl annotate deploy nginx-deploy message='Updated nginx image to 1.17' --overwrite`,
		},
	}
}
