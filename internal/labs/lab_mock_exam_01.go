package labs

import (
	"context"
	"fmt"
)

func init() {
	Register(&MockExam01Lab{})
}

type MockExam01Lab struct {
	BaseLab
}

func (l *MockExam01Lab) ID() string             { return "mock_exam_01" }
func (l *MockExam01Lab) Title() string          { return "Weighted CKA Mock Exam 01" }
func (l *MockExam01Lab) Category() Category     { return CategoryExam }
func (l *MockExam01Lab) Difficulty() Difficulty { return DifficultyHard }
func (l *MockExam01Lab) EstimatedTime() int     { return 120 }
func (l *MockExam01Lab) Tags() []string {
	return []string{"mock-exam", "workloads", "storage", "autoscaling", "helm", "gateway"}
}
func (l *MockExam01Lab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return mockExamPreflight(ctx, kubeconfigPath)
}

func (l *MockExam01Lab) Description() string {
	return `Complete all 12 tasks. You may run verification at any time for a weighted score.
The timer stops only at 100/100.

Task 1 (8 points)
Create Pod mc-pod in namespace mc-namespace with three containers:
- mc-pod-1 uses nginx:1-alpine and has NODE_NAME populated from spec.nodeName.
- mc-pod-2 uses busybox:1 and appends the date every second to /var/log/shared/date.log.
- mc-pod-3 uses busybox:1 and tails /var/log/shared/date.log.
The last two containers share a volume named shared-volume at /var/log/shared. The
data must not survive Pod deletion and recreation.

Task 2 (7 points)
On the control-plane node, repair /etc/crictl.yaml so crictl connects to the
containerd CRI socket at unix:///run/containerd/containerd.sock. Do not restart
Kubernetes components.

Task 3 (6 points)
On the control-plane node, identify every CRD related to VerticalPodAutoscaler and
save the CRD names, one per line, to /root/vpa-crds.txt.

Task 4 (8 points)
Using an imperative command, expose Pod messaging in the default namespace as a
ClusterIP Service named messaging-service on port 6379.

Task 5 (10 points)
Create Deployment hr-web-app in the default namespace using image
kodekloud/webapp-color with 2 replicas.

Task 6 (12 points)
The Pod orange in the default namespace is not becoming Ready. Diagnose and fix it.

Task 7 (8 points)
Expose hr-web-app as a NodePort Service named hr-web-app-service. The application
listens on port 8080 and must be accessible on node port 30082.

Task 8 (8 points)
Create PersistentVolume pv-analytics with capacity 100Mi, access mode ReadWriteMany,
and hostPath /pv/data-analytics.

Task 9 (10 points)
Complete /root/webapp-hpa.yaml on the control-plane node and create HPA webapp-hpa
for Deployment kkapp-deploy in default. Keep minReplicas 2 and maxReplicas 10,
target average CPU utilization of 50%, and set the scale-down stabilization window
to 300 seconds.

Task 10 (9 points)
Create VerticalPodAutoscaler analytics-vpa in default for analytics-deployment.
Configure updateMode Recreate.

Task 11 (6 points)
Create Gateway web-gateway in namespace nginx-gateway using GatewayClass nginx.
Add one listener named http using HTTP on port 80.

Task 12 (8 points)
Release kk-mock1 is installed in namespace kk-ns from repository kk-mock1. Update
the repository cache, then upgrade the podinfo chart to version 6.11.2.`
}

func (l *MockExam01Lab) Hints() []string {
	return []string{
		"Use kubectl explain and imperative --dry-run=client -o yaml when you need a schema quickly",
		"Enter the node with: docker exec -it cka-lab-control-plane bash",
		"Run ./cka-lab-runner lab verify mock_exam_01 whenever you want an updated score",
		"Use helm list -A, helm repo list, and helm search repo -l to inspect Task 12",
	}
}

func (l *MockExam01Lab) Break(ctx context.Context, kubeconfigPath string) error {
	if err := setupMockExam01(ctx, kubeconfigPath); err != nil {
		return fmt.Errorf("setting up mock exam: %w", err)
	}
	return nil
}

func (l *MockExam01Lab) VerifyBroken(context.Context, string) error { return nil }

func (l *MockExam01Lab) Verify(ctx context.Context, kubeconfigPath string) error {
	report := l.Grade(ctx, kubeconfigPath)
	if report.Score() != report.MaxScore() {
		return fmt.Errorf("score is %d/%d", report.Score(), report.MaxScore())
	}
	return nil
}

func (l *MockExam01Lab) SolutionSteps() []SolutionStep {
	return mockExam01Solutions()
}
