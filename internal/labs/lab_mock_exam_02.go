package labs

import (
	"context"
	"fmt"
)

func init() {
	Register(&MockExam02Lab{})
}

type MockExam02Lab struct {
	BaseLab
}

func (l *MockExam02Lab) ID() string             { return "mock_exam_02" }
func (l *MockExam02Lab) Title() string          { return "Weighted CKA Mock Exam 02" }
func (l *MockExam02Lab) Category() Category     { return CategoryExam }
func (l *MockExam02Lab) Difficulty() Difficulty { return DifficultyHard }
func (l *MockExam02Lab) EstimatedTime() int     { return 120 }
func (l *MockExam02Lab) Tags() []string {
	return []string{"mock-exam", "workloads", "crds", "autoscaling", "gateway"}
}

func (l *MockExam02Lab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return mockExam02Preflight(ctx, kubeconfigPath)
}

func (l *MockExam02Lab) Description() string {
	return `Complete all 12 tasks. You may run verification at any time for a weighted score.
The timer stops only at 100/100.

Task 1 (8 points)
Create Pod bootstrap in namespace ops with:
- init container prep using busybox:1 that writes a file at /work/ready
- main container app using nginx:alpine
Both must mount an emptyDir named work at /work.

Task 2 (7 points)
Create Deployment billing-api in the default namespace using image nginx:alpine,
3 replicas, and label tier=backend.

Task 3 (12 points)
Pod checkout in the default namespace is CrashLooping. Diagnose and fix it
so the Pod is Ready.

Task 4 (8 points)
Create CronJob nightly-report in namespace ops. It must run busybox:1 with
command echo done on schedule */5 * * * * and must not be suspended.

Task 5 (6 points)
On the control-plane node, identify every CRD in the gateway.networking.k8s.io
API group and save the CRD names, one per line, to /root/gateway-crds.txt.

Task 6 (8 points)
A Widget custom resource is available in this cluster. Create Widget storefront
in namespace ops with spec.color blue and spec.replicas 2.

Task 7 (10 points)
Create HorizontalPodAutoscaler checkout-hpa for Deployment checkout-app in default.
Use minReplicas 1, maxReplicas 5, average memory utilization 70%, and a scale-up
stabilization window of 60 seconds.

Task 8 (9 points)
Create VerticalPodAutoscaler billing-vpa in default for Deployment billing-api.
Set updateMode to Auto.

Task 9 (8 points)
Create Gateway shop-gateway in namespace edge using GatewayClass nginx.
Add one listener named https using HTTPS on port 443.

Task 10 (6 points)
Create HTTPRoute shop-route in namespace edge that attaches to Gateway shop-gateway
and matches path / to Service shop in the same namespace.

Task 11 (10 points)
Create Job import-job in namespace finance using busybox:1 with command
["sh","-c","echo imported"], completions 1, and backoffLimit 2. It must Complete.

Task 12 (8 points)
Scale Deployment catalog in default to 4 replicas and add label env=prod
on the Deployment object.`
}

func (l *MockExam02Lab) Hints() []string {
	return []string{
		"kubectl explain and --dry-run=client -o yaml remain the fastest way to recall schemas",
		"Enter the node with: docker exec -it cka-lab-control-plane bash",
		"Run ./cka-lab-runner lab verify mock_exam_02 whenever you want an updated score",
		"kubectl get crd and kubectl explain widget.spec describe the custom API",
	}
}

func (l *MockExam02Lab) Break(ctx context.Context, kubeconfigPath string) error {
	if err := setupMockExam02(ctx, kubeconfigPath); err != nil {
		return fmt.Errorf("setting up mock exam: %w", err)
	}
	return nil
}

func (l *MockExam02Lab) VerifyBroken(context.Context, string) error { return nil }

func (l *MockExam02Lab) Verify(ctx context.Context, kubeconfigPath string) error {
	report := l.Grade(ctx, kubeconfigPath)
	if report.Score() != report.MaxScore() {
		return fmt.Errorf("score is %d/%d", report.Score(), report.MaxScore())
	}
	return nil
}

func (l *MockExam02Lab) SolutionSteps() []SolutionStep {
	return mockExam02Solutions()
}
