package labs

const mockExam04CRDs = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gatewayclasses.gateway.networking.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: gateway.networking.k8s.io
  scope: Cluster
  names:
    plural: gatewayclasses
    singular: gatewayclass
    kind: GatewayClass
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              controllerName:
                type: string
            required: [controllerName]
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gateways.gateway.networking.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: gateway.networking.k8s.io
  scope: Namespaced
  names:
    plural: gateways
    singular: gateway
    kind: Gateway
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            required: [gatewayClassName, listeners]
            properties:
              gatewayClassName:
                type: string
              listeners:
                type: array
                x-kubernetes-list-type: atomic
                items:
                  type: object
                  required: [name, protocol, port]
                  x-kubernetes-preserve-unknown-fields: true
                  properties:
                    name:
                      type: string
                    protocol:
                      type: string
                    port:
                      type: integer
                      format: int32
                    hostname:
                      type: string
                    tls:
                      type: object
                      x-kubernetes-preserve-unknown-fields: true
                    allowedRoutes:
                      type: object
                      x-kubernetes-preserve-unknown-fields: true
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: httproutes.gateway.networking.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: gateway.networking.k8s.io
  scope: Namespaced
  names:
    plural: httproutes
    singular: httproute
    kind: HTTPRoute
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            x-kubernetes-preserve-unknown-fields: true
`

const mockExam04HPASkeleton = `apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: worker-hpa
  namespace: api
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: worker-deploy
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
`

const mockExam04BaseResources = `apiVersion: v1
kind: Namespace
metadata:
  name: publish
---
apiVersion: v1
kind: Namespace
metadata:
  name: edge
---
apiVersion: v1
kind: Namespace
metadata:
  name: team-dev
---
apiVersion: v1
kind: Namespace
metadata:
  name: api
---
apiVersion: v1
kind: Namespace
metadata:
  name: edge-gw
---
apiVersion: v1
kind: Namespace
metadata:
  name: charts-safe
---
apiVersion: v1
kind: Namespace
metadata:
  name: charts-legacy
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    purpose: frontend
---
apiVersion: v1
kind: Namespace
metadata:
  name: backend
  labels:
    purpose: backend
---
apiVersion: v1
kind: Namespace
metadata:
  name: databases
  labels:
    purpose: databases
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: portal-deploy
  namespace: edge
spec:
  replicas: 1
  selector:
    matchLabels:
      app: portal
  template:
    metadata:
      labels:
        app: portal
    spec:
      containers:
      - name: web
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: portal-svc
  namespace: edge
spec:
  selector:
    app: portal
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker-deploy
  namespace: api
spec:
  replicas: 1
  selector:
    matchLabels:
      app: worker
  template:
    metadata:
      labels:
        app: worker
    spec:
      containers:
      - name: worker
        image: nginx:alpine
        resources:
          requests:
            memory: 64Mi
            cpu: 50m
---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: cka-gateway
spec:
  controllerName: example.com/cka-gateway
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: edge-gateway
  namespace: edge-gw
spec:
  gatewayClassName: cka-gateway
  listeners:
  - name: https
    protocol: HTTP
    port: 80
    allowedRoutes:
      namespaces:
        from: Same
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend-app
  namespace: frontend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: frontend
  template:
    metadata:
      labels:
        app: frontend
    spec:
      containers:
      - name: app
        image: nginx:alpine
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: backend-app
  namespace: backend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: backend
  template:
    metadata:
      labels:
        app: backend
    spec:
      containers:
      - name: app
        image: nginx:alpine
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db-app
  namespace: databases
spec:
  replicas: 1
  selector:
    matchLabels:
      app: database
  template:
    metadata:
      labels:
        app: database
    spec:
      containers:
      - name: app
        image: nginx:alpine
`

const mockExam04Netpol1 = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: net-policy-1
  namespace: backend
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector: {}
`

const mockExam04Netpol2 = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: net-policy-2
  namespace: backend
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          purpose: frontend
    - namespaceSelector:
        matchLabels:
          purpose: databases
`

const mockExam04Netpol3 = `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: net-policy-3
  namespace: backend
spec:
  podSelector:
    matchLabels:
      app: backend
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          purpose: frontend
`
