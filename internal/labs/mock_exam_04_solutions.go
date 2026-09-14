package labs

func mockExam04Solutions() []SolutionStep {
	return []SolutionStep{
		{
			Description: "Task 1 — create default StorageClass disk-local",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: disk-local
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
EOF
kubectl get sc`,
			Notes: "Docs search: kubernetes.io Storage Classes; Change the default StorageClass",
		},
		{
			Description: "Task 2 — shared HTML publisher (builder writes, nginx serves)",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: content-publisher
  namespace: publish
spec:
  replicas: 1
  selector:
    matchLabels:
      app: content-publisher
  template:
    metadata:
      labels:
        app: content-publisher
    spec:
      volumes:
      - name: site-data
        emptyDir: {}
      containers:
      - name: builder
        image: busybox:1.28
        command: ["sh","-c","while true; do echo '<html><body>ok</body></html>' > /content/index.html; sleep 5; done"]
        volumeMounts:
        - name: site-data
          mountPath: /content
      - name: web
        image: nginx:alpine
        volumeMounts:
        - name: site-data
          mountPath: /usr/share/nginx/html
EOF`,
			Notes: "Docs search: kubernetes.io Configure a Pod to Use a Volume for Storage; Share Process Namespace / multi-container Pods",
		},
		{
			Description: "Task 3 — create portal Ingress",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: portal-ingress
  namespace: edge
spec:
  rules:
  - host: shop.exam.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: portal-svc
            port:
              number: 80
EOF`,
			Notes: "Docs search: kubernetes.io Ingress; Ingress controllers",
		},
		{
			Description: "Task 4 — create then rolling-upgrade via apply",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-rollout
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-rollout
  template:
    metadata:
      labels:
        app: web-rollout
    spec:
      containers:
      - name: nginx
        image: nginx:1.19
EOF
kubectl apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-rollout
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-rollout
  template:
    metadata:
      labels:
        app: web-rollout
    spec:
      containers:
      - name: nginx
        image: nginx:1.21
EOF
kubectl rollout status deploy/web-rollout`,
			Notes: "Docs search: Performing a Rolling Update",
		},
		{
			Description: "Task 5 — CSR + Role + RoleBinding for maria",
			Command: `# On the control-plane node:
docker exec -it cka-lab-control-plane bash
cd /opt/CKA
REQUEST=$(cat maria.csr | base64 | tr -d '\n')
kubectl apply -f - <<EOF
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: maria-developer
spec:
  signerName: kubernetes.io/kube-apiserver-client
  request: $REQUEST
  usages:
  - digital signature
  - key encipherment
  - client auth
EOF
kubectl certificate approve maria-developer
kubectl create role developer -n team-dev --resource=pods --verb=create,list,get,update,delete
kubectl create rolebinding developer-binding -n team-dev --role=developer --user=maria
kubectl auth can-i update pods -n team-dev --as=maria`,
			Notes: "Docs search: Manage TLS Certificates in a Cluster; Using RBAC Authorization",
		},
		{
			Description: "Task 6 — Service/Pod DNS artifacts",
			Command: `kubectl run dns-probe --image=nginx
kubectl expose pod dns-probe --name=dns-probe-svc --port=80 --target-port=80 --type=ClusterIP
kubectl run tmp-svc --image=busybox:1.28 --rm -i --restart=Never -- nslookup dns-probe-svc.default.svc.cluster.local \
  | docker exec -i cka-lab-control-plane tee /opt/CKA/dns.svc
POD_IP=$(kubectl get pod dns-probe -o jsonpath='{.status.podIP}' | tr '.' '-')
kubectl run tmp-pod --image=busybox:1.28 --rm -i --restart=Never -- nslookup ${POD_IP}.default.pod.cluster.local \
  | docker exec -i cka-lab-control-plane tee /opt/CKA/dns.pod`,
			Notes: "Docs search: DNS for Services and Pods. Exam: write files under /opt/CKA on the node.",
		},
		{
			Description: "Task 7 — static Pod on the control-plane",
			Command: `kubectl run priority-web --image=nginx --dry-run=client -o yaml | docker exec -i cka-lab-control-plane \
  tee /etc/kubernetes/manifests/priority-web.yaml
# Strip status/managed fields if present; keep apiVersion/kind/metadata/spec only.
kubectl get pod -A | grep priority-web`,
			Notes: "Docs search: Create static Pods. kind maps ssh/node work to docker exec on the node container.",
		},
		{
			Description: "Task 8 — fix and apply worker-hpa.yaml",
			Command: `# On the control-plane node, edit /opt/CKA/worker-hpa.yaml so that:
#   minReplicas: 2
#   maxReplicas: 12
#   metrics[0].resource.name: memory
#   averageUtilization: 70
kubectl apply -f /opt/CKA/worker-hpa.yaml
kubectl get hpa worker-hpa -n api -o yaml`,
			Notes: "Docs search: Horizontal Pod Autoscaler; autoscaling/v2 memory Utilization",
		},
		{
			Description: "Task 9 — fix Gateway HTTPS listener",
			Command: `kubectl apply -f - <<'EOF'
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: edge-gateway
  namespace: edge-gw
spec:
  gatewayClassName: cka-gateway
  listeners:
  - name: https
    protocol: HTTPS
    port: 443
    hostname: shop.exam.local
    tls:
      certificateRefs:
      - name: shop-tls
    allowedRoutes:
      namespaces:
        from: Same
EOF`,
			Notes: "Docs search: Gateway API Gateway; TLS configuration",
		},
		{
			Description: "Task 10 — find and uninstall vulnerable Helm release",
			Command: `helm list -A
kubectl get deploy -A -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{" "}{.spec.template.spec.containers[*].image}{"\n"}{end}' | grep 1.14-alpine
helm uninstall legacy-color -n charts-legacy
helm list -A`,
			Notes: "Docs search: helm.sh helm_list / helm_uninstall",
		},
		{
			Description: "Task 11 — apply the restrictive NetworkPolicy only",
			Command: `docker exec -it cka-lab-control-plane bash
cat /opt/CKA/netpol-1.yaml /opt/CKA/netpol-2.yaml /opt/CKA/netpol-3.yaml
# netpol-3 only allows frontend → backend
kubectl apply -f /opt/CKA/netpol-3.yaml
kubectl get netpol -n backend`,
			Notes: "Docs search: NetworkPolicies. Do not apply netpol-1 or netpol-2.",
		},
	}
}
