package labs

import (
	"context"
	"fmt"
	"time"
)

func init() {
	Register(&FleetReportForbiddenLab{})
}

type FleetReportForbiddenLab struct {
	BaseLab
}

func (l *FleetReportForbiddenLab) ID() string {
	return "fleet_report_forbidden"
}

func (l *FleetReportForbiddenLab) Title() string {
	return "Capacity Report Only Sees One Namespace"
}

func (l *FleetReportForbiddenLab) Category() Category {
	return CategoryRBAC
}

func (l *FleetReportForbiddenLab) Difficulty() Difficulty {
	return DifficultyHard
}

func (l *FleetReportForbiddenLab) Description() string {
	return `A nightly capacity report runs as the 'fleet-reader' ServiceAccount in namespace
'reporting'. It needs to list the nodes in the cluster and the pods in every namespace,
but it currently fails with two Forbidden errors:

  nodes is forbidden: ... cannot list resource "nodes" at the cluster scope
  pods is forbidden: ... cannot list resource "pods" in the namespace "kube-system"

Your task: grant 'fleet-reader' exactly the read access it needs — list nodes cluster-wide
and list pods in all namespaces — and nothing beyond it. The security review will reject
this if the identity can delete nodes or read Secrets anywhere in the cluster.`
}

func (l *FleetReportForbiddenLab) Hints() []string {
	return []string{
		"kubectl auth can-i list nodes --as=system:serviceaccount:reporting:fleet-reader shows where you stand",
		"A Role and RoleBinding can never grant anything outside their own namespace",
		"Cluster-scoped resources like nodes need a ClusterRole",
		"A ClusterRole bound with a RoleBinding only applies inside that one namespace; bind it with a ClusterRoleBinding for cluster-wide effect",
		"Grant only the verbs the report needs — binding cluster-admin will fail the review check",
	}
}

func (l *FleetReportForbiddenLab) EstimatedTime() int {
	return 25
}

func (l *FleetReportForbiddenLab) Tags() []string {
	return []string{"rbac", "identity", "least-privilege", "cluster-scope"}
}

func (l *FleetReportForbiddenLab) Prepare(ctx context.Context, kubeconfigPath string) error {
	return WaitForClusterReady(ctx, kubeconfigPath)
}

func (l *FleetReportForbiddenLab) Break(ctx context.Context, kubeconfigPath string) error {
	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: reporting
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: fleet-reader
  namespace: reporting
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: fleet-reader-role
  namespace: reporting
rules:
- apiGroups: [""]
  resources: ["pods", "nodes"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: fleet-reader-binding
  namespace: reporting
subjects:
- kind: ServiceAccount
  name: fleet-reader
  namespace: reporting
roleRef:
  kind: Role
  name: fleet-reader-role
  apiGroup: rbac.authorization.k8s.io
`
	if err := kubectlApply(ctx, kubeconfigPath, manifest); err != nil {
		return fmt.Errorf("applying fleet reader scenario: %w", err)
	}
	return nil
}

func (l *FleetReportForbiddenLab) VerifyBroken(ctx context.Context, kubeconfigPath string) error {
	time.Sleep(5 * time.Second)

	if authCanI(ctx, kubeconfigPath, "system:serviceaccount:reporting:fleet-reader", "list", "nodes", "") {
		return fmt.Errorf("fleet-reader can already list nodes")
	}
	return nil
}

func (l *FleetReportForbiddenLab) Verify(ctx context.Context, kubeconfigPath string) error {
	const subject = "system:serviceaccount:reporting:fleet-reader"

	if !authCanI(ctx, kubeconfigPath, subject, "list", "nodes", "") {
		return fmt.Errorf("fleet-reader still cannot list nodes at the cluster scope")
	}
	if !authCanI(ctx, kubeconfigPath, subject, "list", "pods", "kube-system") {
		return fmt.Errorf("fleet-reader still cannot list pods outside its own namespace")
	}
	if authCanI(ctx, kubeconfigPath, subject, "delete", "nodes", "") {
		return fmt.Errorf("fleet-reader can delete nodes — the grant is too broad for the security review")
	}
	if authCanI(ctx, kubeconfigPath, subject, "list", "secrets", "kube-system") {
		return fmt.Errorf("fleet-reader can read Secrets in kube-system — the grant is too broad for the security review")
	}

	return nil
}

func (l *FleetReportForbiddenLab) SolutionSteps() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Confirm what the identity can and cannot do today",
			Command: `kubectl auth can-i list nodes --as=system:serviceaccount:reporting:fleet-reader
kubectl auth can-i list pods -n kube-system --as=system:serviceaccount:reporting:fleet-reader`,
			Notes: "Both no, even though the namespaced Role mentions nodes — a Role cannot grant cluster-scoped access",
		},
		{
			Description: "Create a ClusterRole with only the reads the report needs",
			Command: `kubectl create clusterrole fleet-reader-cluster \
  --verb=get,list,watch --resource=nodes,pods`,
		},
		{
			Description: "Bind it cluster-wide to the ServiceAccount",
			Command: `kubectl create clusterrolebinding fleet-reader-cluster \
  --clusterrole=fleet-reader-cluster \
  --serviceaccount=reporting:fleet-reader`,
			Notes: "A RoleBinding to this ClusterRole would only work inside one namespace",
		},
		{
			Description: "Confirm the grant works",
			Command: `kubectl auth can-i list nodes --as=system:serviceaccount:reporting:fleet-reader
kubectl auth can-i list pods -n kube-system --as=system:serviceaccount:reporting:fleet-reader`,
		},
		{
			Description: "Confirm the grant is not too wide",
			Command: `kubectl auth can-i delete nodes --as=system:serviceaccount:reporting:fleet-reader
kubectl auth can-i list secrets -n kube-system --as=system:serviceaccount:reporting:fleet-reader`,
			Notes: "Both must answer no",
		},
	}
}
