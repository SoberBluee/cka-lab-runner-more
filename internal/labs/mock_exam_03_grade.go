package labs

import (
	"context"
	"strings"
)

func (l *MockExam03Lab) Grade(ctx context.Context, kubeconfigPath string) ExamReport {
	node, _ := getControlPlaneNode(ctx, kubeconfigPath)
	return ExamReport{Tasks: []ExamTaskResult{
		gradeMock03Task1(ctx, kubeconfigPath),
		gradeMock03Task2(ctx, kubeconfigPath),
		gradeMock03Task3(ctx, kubeconfigPath),
		gradeMock03Task4(ctx, kubeconfigPath),
		gradeMock03Task5(ctx, kubeconfigPath),
		gradeMock03Task6(ctx, kubeconfigPath),
		gradeMock03Task7(ctx, kubeconfigPath),
		gradeMock03Task8(ctx, kubeconfigPath),
		gradeMock03Task9(ctx, node),
		gradeMock03Task10(ctx, kubeconfigPath),
	}}
}

func gradeMock03Task1(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	image := kubeEquals(ctx, kubeconfigPath, "busybox:1.28", "get", "pod", "probe", "-n", "practice",
		"-o", "jsonpath={.spec.containers[0].image}")
	phase := kubeEquals(ctx, kubeconfigPath, "Running", "get", "pod", "probe", "-n", "practice",
		"-o", "jsonpath={.status.phase}")
	cmd, _ := kubectl(ctx, kubeconfigPath, "get", "pod", "probe", "-n", "practice",
		"-o", "jsonpath={.spec.containers[0].command}{.spec.containers[0].args}")
	hasSleep := strings.Contains(strings.ToLower(cmd), "sleep")
	return examTask(1, "Probe Pod", 8,
		examCheck("Pod uses busybox:1.28", 3, image),
		examCheck("container runs a sleep command", 2, hasSleep),
		examCheck("Pod is Running", 3, phase))
}

func gradeMock03Task2(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	image := kubeEquals(ctx, kubeconfigPath, "nginx:alpine", "get", "deployment", "store-api", "-n", "practice",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	replicas := kubeEquals(ctx, kubeconfigPath, "2", "get", "deployment", "store-api", "-n", "practice",
		"-o", "jsonpath={.spec.replicas}")
	label := kubeEquals(ctx, kubeconfigPath, "store-api", "get", "deployment", "store-api", "-n", "practice",
		"-o", "jsonpath={.spec.template.metadata.labels.app}")
	return examTask(2, "Store API Deployment", 10,
		examCheck("Deployment uses nginx:alpine", 3, image),
		examCheck("Deployment requests 2 replicas", 4, replicas),
		examCheck("pod template has label app=store-api", 3, label))
}

func gradeMock03Task3(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	port := kubeEquals(ctx, kubeconfigPath, "80", "get", "svc", "store-api", "-n", "practice",
		"-o", "jsonpath={.spec.ports[0].port}")
	target := kubeEquals(ctx, kubeconfigPath, "80", "get", "svc", "store-api", "-n", "practice",
		"-o", "jsonpath={.spec.ports[0].targetPort}")
	selector := kubeEquals(ctx, kubeconfigPath, "store-api", "get", "svc", "store-api", "-n", "practice",
		"-o", "jsonpath={.spec.selector.app}")
	svcType := kubeEquals(ctx, kubeconfigPath, "ClusterIP", "get", "svc", "store-api", "-n", "practice",
		"-o", "jsonpath={.spec.type}")
	return examTask(3, "Store API Service", 10,
		examCheck("Service is ClusterIP on port 80 → 80", 5, port && target && svcType),
		examCheck("selector is app=store-api", 5, selector))
}

func gradeMock03Task4(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	ready := kubeEquals(ctx, kubeconfigPath, "True", "get", "pod", "broken-web", "-n", "practice",
		"-o", `jsonpath={.status.conditions[?(@.type=="Ready")].status}`)
	return examTask(4, "Broken Web Pod", 12,
		examCheck("broken-web Pod is Ready", 12, ready))
}

func gradeMock03Task5(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	nameOK := kubeEquals(ctx, kubeconfigPath, "checkout", "get", "svc", "checkout", "-n", "shop",
		"-o", "jsonpath={.metadata.name}")
	port := kubeEquals(ctx, kubeconfigPath, "80", "get", "svc", "checkout", "-n", "shop",
		"-o", "jsonpath={.spec.ports[0].port}")
	endpoints, _ := endpointAddressCount(ctx, kubeconfigPath, "shop", "checkout")
	return examTask(5, "Checkout Service", 12,
		examCheck("Service checkout still exists on port 80", 4, nameOK && port),
		examCheck("Service has ready backend endpoints", 8, endpoints > 0))
}

func gradeMock03Task6(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	pvExists := kubeEquals(ctx, kubeconfigPath, "practice-pv", "get", "pv", "practice-pv",
		"-o", "jsonpath={.metadata.name}")
	sc := kubeEquals(ctx, kubeconfigPath, "manual", "get", "pvc", "app-data", "-n", "practice",
		"-o", "jsonpath={.spec.storageClassName}")
	mode := kubeEquals(ctx, kubeconfigPath, "ReadWriteOnce", "get", "pvc", "app-data", "-n", "practice",
		"-o", "jsonpath={.spec.accessModes[0]}")
	bound := kubeEquals(ctx, kubeconfigPath, "Bound", "get", "pvc", "app-data", "-n", "practice",
		"-o", "jsonpath={.status.phase}")
	phase := kubeEquals(ctx, kubeconfigPath, "Running", "get", "pod", "data-pod", "-n", "practice",
		"-o", "jsonpath={.status.phase}")
	return examTask(6, "Persistent data", 12,
		examCheck("practice-pv still exists", 2, pvExists),
		examCheck("PVC app-data uses manual + RWO and is Bound", 5, sc && mode && bound),
		examCheck("data-pod is Running", 5, phase))
}

func gradeMock03Task7(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	saOK := kubeEquals(ctx, kubeconfigPath, "dev", "get", "sa", "dev", "-n", "practice",
		"-o", "jsonpath={.metadata.name}")
	roleRef := kubeEquals(ctx, kubeconfigPath, "Role/dev-role", "get", "rolebinding", "dev-binding", "-n", "practice",
		"-o", "jsonpath={.roleRef.kind}/{.roleRef.name}")
	subject := kubeEquals(ctx, kubeconfigPath, "ServiceAccount/dev", "get", "rolebinding", "dev-binding", "-n", "practice",
		"-o", "jsonpath={.subjects[0].kind}/{.subjects[0].name}")

	const as = "system:serviceaccount:practice:dev"
	canRead := authCanI(ctx, kubeconfigPath, as, "get", "pods", "practice") &&
		authCanI(ctx, kubeconfigPath, as, "list", "pods", "practice") &&
		authCanI(ctx, kubeconfigPath, as, "watch", "pods", "practice")
	leastPriv := !authCanI(ctx, kubeconfigPath, as, "create", "pods", "practice") &&
		!authCanI(ctx, kubeconfigPath, as, "delete", "pods", "practice") &&
		!authCanI(ctx, kubeconfigPath, as, "update", "pods", "practice") &&
		!authCanI(ctx, kubeconfigPath, as, "list", "secrets", "practice")

	crbLeak := false
	crb, _ := kubectl(ctx, kubeconfigPath, "get", "clusterrolebindings",
		"-o", "jsonpath={range .items[*]}{.metadata.name}{'|'}{range .subjects[*]}{.namespace}{'/'}{.name}{' '}{end}{'\\n'}{end}")
	for _, line := range strings.Split(crb, "\n") {
		if strings.Contains(line, "practice/dev") {
			crbLeak = true
			break
		}
	}

	return examTask(7, "Developer read access", 12,
		examCheck("ServiceAccount dev exists", 2, saOK),
		examCheck("RoleBinding wires Role/dev-role to SA/dev", 3, roleRef && subject),
		examCheck("dev can get/list/watch pods", 4, canRead),
		examCheck("least privilege (no create/delete/update pods or list secrets; no CRB)", 3, leastPriv && !crbLeak))
}

func gradeMock03Task8(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	ready := kubeEquals(ctx, kubeconfigPath, "1", "get", "deployment", "batch-worker", "-n", "practice",
		"-o", "jsonpath={.status.readyReplicas}")
	taintOK := true
	nodes, err := nodeNames(ctx, kubeconfigPath)
	if err != nil || len(nodes) == 0 {
		taintOK = false
	}
	for _, n := range nodes {
		if !nodeHasTaintKey(ctx, kubeconfigPath, n, mockExam03TaintKey) {
			taintOK = false
			break
		}
	}
	return examTask(8, "Batch worker scheduling", 8,
		examCheck("batch-worker has a Ready replica", 5, ready),
		examCheck("exam node taint still present", 3, taintOK))
}

func gradeMock03Task9(ctx context.Context, node string) ExamTaskResult {
	content, err := dockerExec(ctx, node, "cat", "/opt/CKA/practice-node.txt")
	ok := err == nil && strings.TrimSpace(content) == node
	return examTask(9, "Node name artifact", 8,
		examCheck("/opt/CKA/practice-node.txt has the control-plane node name", 8, ok))
}

func gradeMock03Task10(ctx context.Context, kubeconfigPath string) ExamTaskResult {
	image := kubeEquals(ctx, kubeconfigPath, "busybox:1.28", "get", "job", "report-job", "-n", "practice",
		"-o", "jsonpath={.spec.template.spec.containers[0].image}")
	completions := kubeEquals(ctx, kubeconfigPath, "1", "get", "job", "report-job", "-n", "practice",
		"-o", "jsonpath={.spec.completions}")
	backoff := kubeEquals(ctx, kubeconfigPath, "2", "get", "job", "report-job", "-n", "practice",
		"-o", "jsonpath={.spec.backoffLimit}")
	complete := kubeEquals(ctx, kubeconfigPath, "True", "get", "job", "report-job", "-n", "practice",
		"-o", `jsonpath={.status.conditions[?(@.type=="Complete")].status}`)
	return examTask(10, "Report Job", 8,
		examCheck("Job uses busybox:1.28 with completions 1 and backoffLimit 2", 3, image && completions && backoff),
		examCheck("Job has completed", 5, complete))
}
