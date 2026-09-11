package labs

import (
	"context"
	"fmt"
)

func init() {
	Register(&MockExam03Lab{})
}

type MockExam03Lab struct {
	BaseLab
}

func (l *MockExam03Lab) ID() string             { return "mock_exam_03" }
func (l *MockExam03Lab) Title() string          { return "CKA Practice Exam (Easy/Medium)" }
func (l *MockExam03Lab) Category() Category     { return CategoryExam }
func (l *MockExam03Lab) Difficulty() Difficulty { return DifficultyMedium }
func (l *MockExam03Lab) EstimatedTime() int     { return 90 }
func (l *MockExam03Lab) Tags() []string {
	return []string{"mock-exam", "practice", "workloads", "storage", "rbac", "scheduling"}
}

func (l *MockExam03Lab) Prepare(ctx context.Context, kubeconfigPath string) error {
	if err := WaitForClusterReady(ctx, kubeconfigPath); err != nil {
		return err
	}
	return mockExam03Preflight(ctx, kubeconfigPath)
}

func (l *MockExam03Lab) Hints() []string {
	return []string{
		"Run ./cka-lab-runner lab verify mock_exam_03 whenever you want an updated score",
		"Local kind context is usually kind-cka-lab (exam text still says cka-lab)",
		"Node file tasks: docker exec -it cka-lab-control-plane bash",
	}
}

func (l *MockExam03Lab) Description() string {
	return `Set the context before doing any work:

  kubectl config use-context cka-lab

Complete all 10 tasks. You may run verification at any time for a weighted score.
The timer stops only at 100/100.

Unless a task says otherwise, create resources in namespace practice.
Do not leave temporary debug Pods behind.

────────────────────────────────────────
Task 1 (8 points) — Probe Pod
Create Pod probe in namespace practice using image busybox:1.28.
The container must run a long-lived command (for example sleep 3600) so the Pod becomes Running.

Task 2 (10 points) — Store API Deployment
Create Deployment store-api in namespace practice using image nginx:alpine,
2 replicas, and pod template label app=store-api.

Task 3 (10 points) — Store API Service
Create Service store-api in namespace practice of type ClusterIP.
It must listen on port 80, target port 80, and select pods with app=store-api.

Task 4 (12 points) — Broken Web Pod
Pod broken-web in namespace practice is failing. Diagnose and fix it so the Pod is Ready.
Do not rename the Pod.

Task 5 (12 points) — Checkout Service
Service checkout in namespace shop does not reach its backends. Fix it so the Service
has ready endpoints. Keep the Service name checkout and Service port 80 unchanged.

Task 6 (12 points) — Persistent data
PersistentVolume practice-pv already exists.
1. Create PersistentVolumeClaim app-data in namespace practice that binds to it
   (storageClassName manual, accessModes ReadWriteOnce, request 1Gi).
2. Pod data-pod in namespace practice is waiting on that claim — get it to Running.
Do not rename or delete practice-pv. Keep storageClassName manual on the claim.

Task 7 (12 points) — Developer read access
In namespace practice create:
- ServiceAccount named dev
- Role named dev-role allowing get, list, and watch on pods only
- RoleBinding named dev-binding binding Role dev-role to ServiceAccount dev
Least privilege: do not grant create/delete/update/patch or access to secrets.
Do not create ClusterRoles or ClusterRoleBindings for this task.

Task 8 (8 points) — Batch worker scheduling
Deployment batch-worker in namespace practice is Pending.
Get it to have Ready replicas. Do not remove node taints applied for this exam.

Task 9 (8 points) — Node name artifact
Write the control-plane node name (exact string, no trailing newline required)
to /opt/CKA/practice-node.txt on the control-plane node.

Task 10 (8 points) — Report Job
Create Job report-job in namespace practice using image busybox:1.28 with command
["sh","-c","echo ok"], completions 1, and backoffLimit 2. The Job must Complete.
`
}

func (l *MockExam03Lab) Break(ctx context.Context, kubeconfigPath string) error {
	if err := setupMockExam03(ctx, kubeconfigPath); err != nil {
		return fmt.Errorf("setting up mock exam: %w", err)
	}
	return nil
}

func (l *MockExam03Lab) VerifyBroken(context.Context, string) error { return nil }

func (l *MockExam03Lab) Verify(ctx context.Context, kubeconfigPath string) error {
	report := l.Grade(ctx, kubeconfigPath)
	if report.Score() != report.MaxScore() {
		return fmt.Errorf("score is %d/%d", report.Score(), report.MaxScore())
	}
	return nil
}

func (l *MockExam03Lab) SolutionSteps() []SolutionStep {
	return mockExam03Solutions()
}
