package labs

const mockExam02CRDs = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: verticalpodautoscalers.autoscaling.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: autoscaling.k8s.io
  scope: Namespaced
  names:
    plural: verticalpodautoscalers
    singular: verticalpodautoscaler
    kind: VerticalPodAutoscaler
    shortNames: [vpa]
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
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: verticalpodautoscalercheckpoints.autoscaling.k8s.io
  annotations:
    api-approved.kubernetes.io: "unapproved, experimental-only"
spec:
  group: autoscaling.k8s.io
  scope: Namespaced
  names:
    plural: verticalpodautoscalercheckpoints
    singular: verticalpodautoscalercheckpoint
    kind: VerticalPodAutoscalerCheckpoint
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
---
apiVersion: apiextensions.k8s.io/v1
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
                  properties:
                    name:
                      type: string
                    protocol:
                      type: string
                    port:
                      type: integer
                      format: int32
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
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.ops.exam.local
spec:
  group: ops.exam.local
  scope: Namespaced
  names:
    plural: widgets
    singular: widget
    kind: Widget
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
            required: [color, replicas]
            properties:
              color:
                type: string
              replicas:
                type: integer
`

const mockExam02BaseResources = `apiVersion: v1
kind: Namespace
metadata:
  name: ops
---
apiVersion: v1
kind: Namespace
metadata:
  name: edge
---
apiVersion: v1
kind: Namespace
metadata:
  name: finance
---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: nginx
spec:
  controllerName: exam.cka.local/nginx
---
apiVersion: v1
kind: Pod
metadata:
  name: checkout
  namespace: default
  labels:
    app: checkout
spec:
  containers:
  - name: checkout
    image: nginx:alpine
    command: ["/bin/sh"]
    args: ["-c", "cat /config/app.conf && nginx -g 'daemon off;'"]
    volumeMounts:
    - name: config
      mountPath: /config
  volumes:
  - name: config
    configMap:
      name: checkout-config-missing
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout-app
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: checkout-app
  template:
    metadata:
      labels:
        app: checkout-app
    spec:
      containers:
      - name: web
        image: nginx:alpine
        resources:
          requests:
            cpu: 20m
            memory: 16Mi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalog
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: catalog
  template:
    metadata:
      labels:
        app: catalog
    spec:
      containers:
      - name: catalog
        image: nginx:alpine
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: shop
  namespace: edge
spec:
  replicas: 1
  selector:
    matchLabels:
      app: shop
  template:
    metadata:
      labels:
        app: shop
    spec:
      containers:
      - name: shop
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: shop
  namespace: edge
spec:
  selector:
    app: shop
  ports:
  - port: 80
    targetPort: 80
`
