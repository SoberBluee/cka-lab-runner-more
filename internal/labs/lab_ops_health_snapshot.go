package labs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&OpsHealthSnapshotLab{})
}

// OpsHealthSnapshotLab is a mixed lab: fix a CrashLoop workload, record node
// kubelet version via host access, and create a ConfigMap from that value.
type OpsHealthSnapshotLab struct {
	BaseLab
}

func (l *OpsHealthSnapshotLab) ID() string { return "ops_health_snapshot" }

func (l *OpsHealthSnapshotLab) Title() string {
	return "Ops Health Snapshot Incomplete"
}

func (l *OpsHealthSnapshotLab) Category() Category { return CategoryWorkloads }

func (l *OpsHealthSnapshotLab) Difficulty() Difficulty { return DifficultyHard }

func (l *OpsHealthSnapshotLab) EstimatedTime() int { return 10 }

func (l *OpsHealthSnapshotLab) Tags() []string {
	return []string{"mixed", "workloads", "nodes", "configmap"}
}

func (l *OpsHealthSnapshotLab) Hints() []string { return nil }

func (l *OpsHealthSnapshotLab) Description() string {
	return `Set the context and namespace before doing any work:

  kubectl config use-context cka-lab
  # Workload objects for this task must be in namespace ops

[Weight: 8%] | Time limit: 7–10 minutes

Context:
The ops team cannot complete their nightly health snapshot. Deployment reporter in
namespace ops never stays up. They also still need a recorded kubelet version from
the control plane node.

Task:
1. Get Deployment reporter in namespace ops to 1/1 Ready (do not change its name,
   ServiceAccount, or replica count).
2. SSH to the control plane node named cka-lab-control-plane, escalate with sudo -i,
   and write the node's kubelet version string to /opt/CKA/kubelet-version.txt
   (exact value from the node; one line, no extra text). Example source:
   kubectl get node cka-lab-control-plane -o jsonpath='{.status.nodeInfo.kubeletVersion}'
3. Create ConfigMap named node-info in namespace ops with data key kubelet-version
   whose value exactly matches the contents of /opt/CKA/kubelet-version.txt.
4. Delete any temporary debug Pods you created in namespace ops.

Constraints:
- Work only in context cka-lab. Workload/ConfigMap objects must be in namespace ops (wrong placement = 0).
- Do not change Deployment name reporter or replicas (must remain 1).
- Do not remove or rename ServiceAccount reporter-sa if present.
- Resource names and ConfigMap keys must match exactly (case-sensitive).
- Do not leave temporary debug pods, Jobs, or test resources behind.`
}

func (l *OpsHealthSnapshotLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *OpsHealthSnapshotLab) Break(ctx context.Context, kubeconfigPath string) error {
	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}
	if _, err := dockerExec(ctx, node, "mkdir", "-p", "/opt/CKA"); err != nil {
		return fmt.Errorf("creating /opt/CKA: %w", err)
	}
	_, _ = dockerExec(ctx, node, "rm", "-f", "/opt/CKA/kubelet-version.txt")

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: ops
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: reporter-sa
  namespace: ops
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: reporter
  namespace: ops
spec:
  replicas: 1
  selector:
    matchLabels:
      app: reporter
  template:
    metadata:
      labels:
        app: reporter
    spec:
      serviceAccountName: reporter-sa
      containers:
      - name: reporter
        image: busybox:1.28
        command: ["sh", "-c", "echo reporter failed; exit 1"]
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying ops health scenario: %w", err)
	}
	return nil
}

func (l *OpsHealthSnapshotLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	return waitFor(ctx, 60*time.Second, func() error {
		restarts, err := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "ops",
			"-l", "app=reporter",
			"-o", "jsonpath={.items[0].status.containerStatuses[0].restartCount}")
		if err != nil {
			return fmt.Errorf("reporter pod not found yet: %w", err)
		}
		n, _ := strconv.Atoi(strings.TrimSpace(restarts))
		if n < 1 {
			phase, _ := kubectl(ctx, kubeconfigPath, "get", "pods", "-n", "ops",
				"-l", "app=reporter",
				"-o", "jsonpath={.items[0].status.containerStatuses[0].state.terminated.exitCode}")
			if strings.TrimSpace(phase) == "1" {
				return nil
			}
			return fmt.Errorf("reporter has not failed yet (restarts=%q)", strings.TrimSpace(restarts))
		}
		return nil
	})
}

func (l *OpsHealthSnapshotLab) Verify(ctx context.Context, kubeconfigPath string) error {
	replicas, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "reporter", "-n", "ops",
		"-o", "jsonpath={.spec.replicas}")
	if err != nil {
		return fmt.Errorf("Deployment reporter missing: %w", err)
	}
	if strings.TrimSpace(replicas) != "1" {
		return fmt.Errorf("reporter replicas must remain 1 (got %q)", strings.TrimSpace(replicas))
	}

	sa, err := kubectl(ctx, kubeconfigPath, "get", "deploy", "reporter", "-n", "ops",
		"-o", "jsonpath={.spec.template.spec.serviceAccountName}")
	if err != nil {
		return fmt.Errorf("reading reporter ServiceAccount: %w", err)
	}
	if strings.TrimSpace(sa) != "reporter-sa" {
		return fmt.Errorf("reporter must keep serviceAccountName reporter-sa (got %q)", strings.TrimSpace(sa))
	}

	if err := deploymentReady(ctx, kubeconfigPath, "ops", "reporter", 1, 90*time.Second); err != nil {
		return err
	}

	node, err := getControlPlaneNode(ctx, kubeconfigPath)
	if err != nil {
		return err
	}

	wantVersion, err := kubectl(ctx, kubeconfigPath, "get", "node", node,
		"-o", "jsonpath={.status.nodeInfo.kubeletVersion}")
	if err != nil {
		return fmt.Errorf("reading kubelet version: %w", err)
	}
	wantVersion = strings.TrimSpace(wantVersion)

	fileContent, err := dockerExec(ctx, node, "cat", "/opt/CKA/kubelet-version.txt")
	if err != nil {
		return fmt.Errorf("/opt/CKA/kubelet-version.txt missing: %w", err)
	}
	gotFile := strings.TrimSpace(fileContent)
	if gotFile != wantVersion {
		return fmt.Errorf("/opt/CKA/kubelet-version.txt is %q, node reports %q", gotFile, wantVersion)
	}

	cmVal, err := kubectl(ctx, kubeconfigPath, "get", "configmap", "node-info", "-n", "ops",
		"-o", "jsonpath={.data.kubelet-version}")
	if err != nil {
		return fmt.Errorf("ConfigMap node-info missing in ops: %w", err)
	}
	if strings.TrimSpace(cmVal) != wantVersion {
		return fmt.Errorf("ConfigMap node-info kubelet-version is %q, want %q", strings.TrimSpace(cmVal), wantVersion)
	}

	return assertNoDebugPods(ctx, kubeconfigPath, "ops")
}

func (l *OpsHealthSnapshotLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Inspect why reporter is failing",
			Command:     "kubectl get pods -n ops; kubectl logs -n ops deploy/reporter; kubectl get deploy reporter -n ops -o yaml | head -40",
		},
		{
			Description: "Fix the container command imperatively",
			Command: `kubectl patch deploy reporter -n ops --type json -p='[
  {"op":"replace","path":"/spec/template/spec/containers/0/command","value":["sh","-c","while true; do sleep 30; done"]}
]'`,
			Notes: "Docs search: kubernetes.io Deployments — update the pod template; keep name, replicas, and serviceAccountName",
		},
		{
			Description: "Record kubelet version on the control plane node",
			Command: `VER=$(kubectl get node cka-lab-control-plane -o jsonpath='{.status.nodeInfo.kubeletVersion}')
# Exam: ssh cka-lab-control-plane && sudo -i && echo -n "$VER" > /opt/CKA/kubelet-version.txt
# kind equivalent:
docker exec cka-lab-control-plane bash -c "echo -n '$VER' > /opt/CKA/kubelet-version.txt"`,
			Notes: "Exam uses ssh + sudo -i; this lab environment equivalent is docker exec -it <node> bash",
		},
		{
			Description: "Create the ConfigMap from the recorded value",
			Command: `VER=$(kubectl get node cka-lab-control-plane -o jsonpath='{.status.nodeInfo.kubeletVersion}')
kubectl create configmap node-info -n ops --from-literal=kubelet-version="$VER"`,
			Notes: "Docs search: kubernetes.io ConfigMap",
		},
		{
			Description: "Confirm Ready and clean up debug pods",
			Command:     "kubectl get deploy,pods,cm -n ops; kubectl delete pod tmp test debug -n ops --ignore-not-found",
		},
	}
}
