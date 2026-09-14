package labs

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func init() {
	Register(&OpsInventoryExportLab{})
}

type OpsInventoryExportLab struct {
	BaseLab
}

func (l *OpsInventoryExportLab) ID() string             { return "ops_inventory_export" }
func (l *OpsInventoryExportLab) Title() string          { return "Ops Inventory Export Incomplete" }
func (l *OpsInventoryExportLab) Category() Category     { return CategoryWorkloads }
func (l *OpsInventoryExportLab) Difficulty() Difficulty { return DifficultyMedium }
func (l *OpsInventoryExportLab) EstimatedTime() int     { return 10 }
func (l *OpsInventoryExportLab) Hints() []string        { return nil }
func (l *OpsInventoryExportLab) Tags() []string {
	return []string{"creation", "kubectl", "custom-columns", "jsonpath"}
}

func (l *OpsInventoryExportLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # All work for this task must be done in namespace ops-admin

[Weight: 15%] | Time limit: 8–10 minutes

Context:
Platform ops needs an inventory of Deployments in namespace ops-admin.

Task:
1. Print the names of all Deployments in namespace ops-admin in the following format:

DEPLOYMENT   CONTAINER_IMAGE   READY_REPLICAS   NAMESPACE

<deployment name>   <container image used>   <ready replica count>   <Namespace>

2. Sort by increasing order of the Deployment name.
3. Write the result to /opt/CKA/ops_inventory on the control-plane node.

Example:
DEPLOYMENT   CONTAINER_IMAGE   READY_REPLICAS   NAMESPACE
deploy0   nginx:alpine   1   ops-admin

Constraints:
- Work only in context cka-lab and namespace ops-admin.
- Column headers must match exactly.
- Do not leave temporary debug Pods behind.
`
}

func (l *OpsInventoryExportLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *OpsInventoryExportLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return fmt.Errorf("creating /opt/CKA: %w", err)
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/ops_inventory")

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: ops-admin
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy-cache
  namespace: ops-admin
spec:
  replicas: 2
  selector:
    matchLabels:
      app: deploy-cache
  template:
    metadata:
      labels:
        app: deploy-cache
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy-api
  namespace: ops-admin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: deploy-api
  template:
    metadata:
      labels:
        app: deploy-api
    spec:
      containers:
      - name: api
        image: nginx:1.25-alpine
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy-web
  namespace: ops-admin
spec:
  replicas: 3
  selector:
    matchLabels:
      app: deploy-web
  template:
    metadata:
      labels:
        app: deploy-web
    spec:
      containers:
      - name: web
        image: nginx:alpine
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("creating ops-admin workloads: %w", err)
	}
	return nil
}

func (l *OpsInventoryExportLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "test", "-f", "/opt/CKA/ops_inventory"); err == nil {
		return fmt.Errorf("ops_inventory already exists")
	}
	return nil
}

func (l *OpsInventoryExportLab) Verify(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	content, err := dockerExec(ctx, node, "cat", "/opt/CKA/ops_inventory")
	if err != nil {
		return fmt.Errorf("/opt/CKA/ops_inventory missing: %w", err)
	}

	want, err := expectedOpsInventory(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	got := normalizeInventory(content)
	if got != want {
		return fmt.Errorf("ops_inventory content mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
	return nil
}

func expectedOpsInventory(ctx context.Context, kubeconfigPath string) (string, error) {
	raw, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "-n", "ops-admin",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{'\t'}{.spec.template.spec.containers[0].image}{'\t'}{.status.readyReplicas}{'\t'}{.metadata.namespace}{'\n'}{end}")
	if err != nil {
		return "", err
	}
	type row struct {
		name, image, ready, ns string
	}
	var rows []row
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		ready := parts[2]
		if ready == "" {
			ready = "<none>"
		}
		rows = append(rows, row{parts[0], parts[1], ready, parts[3]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	var b strings.Builder
	b.WriteString("DEPLOYMENT   CONTAINER_IMAGE   READY_REPLICAS   NAMESPACE")
	for _, r := range rows {
		fmt.Fprintf(&b, "\n%s   %s   %s   %s", r.name, r.image, r.ready, r.ns)
	}
	return normalizeInventory(b.String()), nil
}

func normalizeInventory(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		out = append(out, strings.Join(fields, "   "))
	}
	return strings.Join(out, "\n")
}

func (l *OpsInventoryExportLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Generate the sorted custom-columns inventory",
			Command: `kubectl -n ops-admin get deployment \
  -o custom-columns=DEPLOYMENT:.metadata.name,CONTAINER_IMAGE:.spec.template.spec.containers[].image,READY_REPLICAS:.status.readyReplicas,NAMESPACE:.metadata.namespace \
  --sort-by=.metadata.name`,
			Notes: "Docs search: kubectl cheat sheet; custom-columns",
		},
		{
			Description: "Write the file on the control-plane node",
			Command: `kubectl -n ops-admin get deployment \
  -o custom-columns=DEPLOYMENT:.metadata.name,CONTAINER_IMAGE:.spec.template.spec.containers[].image,READY_REPLICAS:.status.readyReplicas,NAMESPACE:.metadata.namespace \
  --sort-by=.metadata.name \
  | docker exec -i cka-lab-control-plane tee /opt/CKA/ops_inventory`,
			Notes: "Exam: ssh to control-plane and redirect to /opt/CKA/ops_inventory",
		},
	}
}
