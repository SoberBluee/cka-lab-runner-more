package labs

import (
	"context"
	"fmt"
)

func init() {
	Register(&MockExam04Lab{})
}

type MockExam04Lab struct {
	BaseLab
}

func (l *MockExam04Lab) ID() string             { return "mock_exam_04" }
func (l *MockExam04Lab) Title() string          { return "CKA Practice Exam (Mixed Topics)" }
func (l *MockExam04Lab) Category() Category     { return CategoryExam }
func (l *MockExam04Lab) Difficulty() Difficulty { return DifficultyHard }
func (l *MockExam04Lab) EstimatedTime() int     { return 120 }
func (l *MockExam04Lab) Tags() []string {
	return []string{"mock-exam", "storage", "workloads", "networking", "rbac", "helm", "gateway"}
}

func (l *MockExam04Lab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return mockExam04Preflight(ctx, kubeconfigPath)
}

func (l *MockExam04Lab) Hints() []string {
	return []string{
		"Run ./cka-lab-runner lab verify mock_exam_04 whenever you want an updated score",
		"Local kind context is usually kind-cka-lab (exam text still says cka-lab)",
		"Node file tasks: docker exec -it cka-lab-control-plane bash",
	}
}

func (l *MockExam04Lab) Description() string {
	return `Set the context before doing any work:

  kubectl config use-context cka-lab

Complete all 11 tasks. You may run verification at any time for a weighted score.
The timer stops only at 100/100.

Do not leave temporary debug Pods behind unless a task requires an artifact from one
(and then remove it afterward).

────────────────────────────────────────
Task 1 (8 points) — Default StorageClass
Create StorageClass disk-local with:
- provisioner kubernetes.io/no-provisioner
- volumeBindingMode WaitForFirstConsumer
- allowVolumeExpansion true
Make disk-local the default StorageClass.

Task 2 (10 points) — Shared content publisher
In namespace publish create Deployment content-publisher with 1 replica and two containers
that share an emptyDir volume named site-data:
- builder (image busybox:1.28) continuously writes HTML to /content/index.html
  (for example a loop that echoes a simple page and sleeps)
- web (image nginx:alpine) must serve that content by mounting the same volume at
  /usr/share/nginx/html
Both containers must stay Running. The shared volume must not survive Pod deletion
(emptyDir, not a PVC).

Task 3 (10 points) — Ingress
Deployment portal-deploy and Service portal-svc already exist in namespace edge.
Create Ingress portal-ingress in edge that:
- uses pathType Prefix
- routes host shop.exam.local path / to Service portal-svc port 80
Do not rename the existing Deployment or Service.

Task 4 (8 points) — Rolling update
Using kubectl apply only (create and upgrade):
1. Create Deployment web-rollout in the default namespace with image nginx:1.19 and 1 replica.
2. Upgrade it to image nginx:1.21 with a rolling update (still via kubectl apply).

Task 5 (10 points) — User certificate and RBAC
Create user maria access using CSR named maria-developer.
The private key is at /opt/CKA/maria.key and the CSR PEM is at /opt/CKA/maria.csr
on the control-plane node.
- Approve the CertificateSigningRequest after creating it (signerName required).
- Create Role developer in namespace team-dev granting create, list, get, update, and
  delete on pods only.
- Create RoleBinding developer-binding in team-dev binding Role developer to user maria.
Least privilege: do not grant secrets or cluster-wide bindings for this task.

Task 6 (10 points) — DNS lookup artifacts
Create Pod dns-probe using image nginx and expose it with ClusterIP Service
dns-probe-svc (port 80 → 80) in the default namespace.
From inside the cluster, using image busybox:1.28:
- save nslookup output for the Service DNS name to /opt/CKA/dns.svc
- save nslookup output for the Pod DNS name (pod-ip with dots as hyphens under
  default.pod) to /opt/CKA/dns.pod
Remove any temporary lookup Pods afterward.

Task 7 (8 points) — Static Pod
On the control-plane node create a static Pod named priority-web using image nginx.
Place the manifest under /etc/kubernetes/manifests so kubelet recreates it on failure.
The API-visible Pod name will be priority-web-<nodename>.

Task 8 (10 points) — HorizontalPodAutoscaler
On the control-plane node, complete /opt/CKA/worker-hpa.yaml and create HPA worker-hpa
in namespace api for Deployment worker-deploy.
Requirements:
- average memory utilization 70%
- minReplicas 2, maxReplicas 12

Task 9 (10 points) — Gateway TLS listener
Gateway edge-gateway in namespace edge-gw is misconfigured.
Update it so the https listener uses HTTPS on port 443 for hostname shop.exam.local
with TLS certificate from Secret shop-tls (already present in edge-gw).
Keep the Gateway name and GatewayClass.

Task 10 (10 points) — Remove vulnerable Helm release
Multiple Helm releases are installed across namespaces. One Deployment uses the
vulnerable image nginx:1.14-alpine. Find that release and uninstall it with Helm.
Do not delete unrelated releases.

Task 11 (6 points) — NetworkPolicy selection
Namespaces frontend, backend, and databases already exist with sample workloads.
Three candidate NetworkPolicy manifests are on the control-plane node:
  /opt/CKA/netpol-1.yaml
  /opt/CKA/netpol-2.yaml
  /opt/CKA/netpol-3.yaml
Apply the most restrictive policy that allows traffic from frontend apps to backend
apps while denying databases. Do not delete existing policies. Do not apply the
incorrect candidates.
`
}

func (l *MockExam04Lab) Break(ctx context.Context, kubeconfigPath string) error {
	if err := setupMockExam04(ctx, kubeconfigPath); err != nil {
		return fmt.Errorf("setting up mock exam: %w", err)
	}
	return nil
}

func (l *MockExam04Lab) VerifyBroken(context.Context, string) error { return nil }

func (l *MockExam04Lab) Verify(ctx context.Context, kubeconfigPath string) error {
	report := l.Grade(ctx, kubeconfigPath)
	if report.Score() != report.MaxScore() {
		return fmt.Errorf("score is %d/%d", report.Score(), report.MaxScore())
	}
	return nil
}

func (l *MockExam04Lab) SolutionSteps() []SolutionStep {
	return mockExam04Solutions()
}
