package labs

func mockExam03Solutions() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Task 1 — create the probe Pod",
			Command:     `kubectl run probe -n practice --image=busybox:1.28 --command -- sleep 3600`,
			Notes:       "Docs search: kubectl run",
		},
		{
			Description: "Task 2 — create the store-api Deployment",
			Command: `kubectl create deployment store-api -n practice --image=nginx:alpine --replicas=2
kubectl label deployment store-api -n practice app=store-api --overwrite
# Ensure the pod template selector/labels include app=store-api (create already sets this).`,
			Notes: "Docs search: kubectl create deployment",
		},
		{
			Description: "Task 3 — expose store-api",
			Command: `kubectl expose deployment store-api -n practice --name=store-api \
  --port=80 --target-port=80 --type=ClusterIP`,
			Notes: "Docs search: kubectl expose",
		},
		{
			Description: "Task 4 — fix broken-web",
			Command: `kubectl describe pod broken-web -n practice
# The container exits immediately. Replace the failing command with a long-lived one, e.g.:
kubectl delete pod broken-web -n practice
kubectl run broken-web -n practice --image=busybox:1.28 --labels=app=broken-web --command -- sleep 3600
# Or edit in place with kubectl edit / replace if you prefer keeping the same object history.`,
			Notes: "Docs search: Troubleshoot Applications; CrashLoopBackOff",
		},
		{
			Description: "Task 5 — fix checkout Service selector",
			Command: `kubectl get deploy,svc -n shop -o wide
kubectl get endpointslices -n shop -l kubernetes.io/service-name=checkout
# Deployment pods are labeled app=checkout; Service selects app=checkouts.
kubectl patch svc checkout -n shop -p '{"spec":{"selector":{"app":"checkout"}}}'`,
			Notes: "Docs search: Connect a Service to your App; Service selectors",
		},
		{
			Description: "Task 6 — bind PVC and start data-pod",
			Command: `kubectl get pv practice-pv -o yaml
kubectl apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data
  namespace: practice
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: manual
  resources:
    requests:
      storage: 1Gi
EOF
kubectl get pvc app-data -n practice
kubectl get pod data-pod -n practice`,
			Notes: "Docs search: Persistent Volumes; Configure a Pod to Use a PersistentVolume for Storage",
		},
		{
			Description: "Task 7 — RBAC for read-only pods",
			Command: `kubectl create serviceaccount dev -n practice
kubectl create role dev-role -n practice \
  --verb=get,list,watch --resource=pods
kubectl create rolebinding dev-binding -n practice \
  --role=dev-role --serviceaccount=practice:dev
kubectl auth can-i list pods -n practice --as=system:serviceaccount:practice:dev`,
			Notes: "Docs search: Using RBAC Authorization",
		},
		{
			Description: "Task 8 — tolerate the exam taint",
			Command: `kubectl describe nodes | grep -A5 Taints
kubectl describe pod -n practice -l app=batch-worker | grep -A8 Events
kubectl patch deployment batch-worker -n practice --type merge -p '{"spec":{"template":{"spec":{"tolerations":[
  {"key":"node-role.kubernetes.io/control-plane","operator":"Exists","effect":"NoSchedule"},
  {"key":"node-role.kubernetes.io/master","operator":"Exists","effect":"NoSchedule"},
  {"key":"workload","operator":"Equal","value":"batch","effect":"NoSchedule"}
]}}}}'`,
			Notes: "Docs search: Taints and Tolerations. Do not remove the workload taint.",
		},
		{
			Description: "Task 9 — write the control-plane node name",
			Command: `kubectl get nodes
# On the control-plane node:
docker exec -it cka-lab-control-plane bash
mkdir -p /opt/CKA
kubectl get nodes -o jsonpath='{.items[?(@.metadata.labels.node-role\.kubernetes\.io/control-plane=="")].metadata.name}' > /opt/CKA/practice-node.txt
# Or simply: echo -n cka-lab-control-plane > /opt/CKA/practice-node.txt`,
			Notes: "Docs search: Viewing Pods and Nodes",
		},
		{
			Description: "Task 10 — create the report Job",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: batch/v1
kind: Job
metadata:
  name: report-job
  namespace: practice
spec:
  completions: 1
  backoffLimit: 2
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: report
        image: busybox:1.28
        command: ["sh", "-c", "echo ok"]
EOF
kubectl wait --for=condition=complete job/report-job -n practice --timeout=60s`,
			Notes: "Docs search: Jobs — Run to Completion",
		},
	}
}
