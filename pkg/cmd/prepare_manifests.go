package cmd

const latestContainerdManifests = "https://raw.githubusercontent.com/warm-metal/cliapp/master/config/samples/containerd.yaml"
const latestMinikubeManifests = "https://raw.githubusercontent.com/warm-metal/cliapp/master/config/samples/minikube.yaml"

const containerdManifests = `apiVersion: v1
kind: Namespace
metadata:
  labels:
    control-plane: controller-manager
  name: cliapp-system
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.4.1
  creationTimestamp: null
  name: cliapps.core.cliapp.warm-metal.tech
spec:
  group: core.cliapp.warm-metal.tech
  names:
    kind: CliApp
    listKind: CliAppList
    plural: cliapps
    singular: cliapp
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - jsonPath: .spec.targetPhase
      name: TargetPhase
      type: string
    - jsonPath: .status.phase
      name: Phase
      type: string
    - jsonPath: .status.podName
      name: Pod
      type: string
    - jsonPath: .status.error
      name: Error
      type: string
    - jsonPath: .spec.distro
      name: Distro
      type: string
    - jsonPath: .spec.shell
      name: Shell
      type: string
    name: v1
    schema:
      openAPIV3Schema:
        description: CliApp is the Schema for the cliapps API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: CliAppSpec defines the desired state of CliApp
            properties:
              command:
                description: Set the command to be executed when client runs the app. It is usually an executable binary. It should be found in the PATH, or an absolute path to the binary. If no set, session-gate will run commands in the app context rootfs instead of the rootfs of Spec.Image.
                items:
                  type: string
                type: array
              distro:
                description: 'Distro the app dependents. The default is alpine. Valid values are: - "alpine" (default): The app works on Alpine; - "ubuntu: The app works on Ubuntu.'
                enum:
                - alpine
                - ubuntu
                type: string
              dockerfile:
                description: Specify a Dockerfile to build a image used to run the app. Http(s) URI is also supported. Only one of Image or Dockerfile can be set.
                type: string
              env:
                description: Environment variables in the form of "key=value".
                items:
                  type: string
                type: array
              fork:
                description: Specify that the app will fork a workload in the same namespace.
                properties:
                  container:
                    description: Set the target container name if the ForObject has more than one containers.
                    type: string
                  object:
                    description: Specify the kind and name of the object to be forked. The object could be either of Deployment, StatefulSet, DaemonSet, ReplicaSet, (Cron)Job, or Pod. The valid format would be Kind/Name.
                    type: string
                  withEnvs:
                    description: Set if expected to inherit envs from the original workload
                    type: boolean
                type: object
              hostpath:
                description: Host paths would be mounted to the app. Each HostPath can be an absolute host path, or in the form of "hostpath:mount-point".
                items:
                  type: string
                type: array
              image:
                description: Specify the image the app uses. Only one of Image or Dockerfile can be set.
                type: string
              shell:
                description: 'The shell interpreter you preferred. Can be either bash or zsh. Valid values are: - "bash" (default): The app will run in Bash; - "zsh: The app will run in Zsh.'
                enum:
                - bash
                - zsh
                type: string
              targetPhase:
                description: 'The target phase the app should achieve. Valid values are: - "Rest" (default): The app is installed but not started; - "Live": The app is running.'
                enum:
                - Rest
                - Recovering
                - Building
                - Live
                - WaitingForSessions
                - ShuttingDown
                type: string
              uninstall:
                description: Set if uninstalls the App when it transits out of phase Live
                type: boolean
            type: object
          status:
            description: CliAppStatus defines the observed state of CliApp
            properties:
              error:
                description: Specify Errors on reconcile.
                type: string
              lastPhaseTransition:
                description: Timestamp of the last phase transition
                format: date-time
                type: string
              phase:
                description: 'Show the app state. Valid values are: - "Rest" (default): The app is installed but not started; - "Recovering": The app is starting; - "Building": The app is waiting for image building; - "Live": The app is running; - "WaitingForSessions": The app is waiting for new sessions and will be shutdown later; - "ShuttingDown": The app is shutting down.'
                enum:
                - Rest
                - Recovering
                - Building
                - Live
                - WaitingForSessions
                - ShuttingDown
                type: string
              podName:
                description: Specify the Pod name if app is in phase Live.
                type: string
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cliapp-session-gate
  namespace: cliapp-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: csi-image-warm-metal
  namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: csi-configmap-warm-metal
  namespace: system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cliapp-leader-election-role
  namespace: cliapp-system
rules:
- apiGroups:
  - ""
  - coordination.k8s.io
  resources:
  - configmaps
  - leases
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: cliapp-manager-role
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - create
  - delete
  - deletecollection
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - apps
  resources:
  - daemonsets
  - deployments
  - replicasets
  - statefulsets
  verbs:
  - get
- apiGroups:
  - batch
  resources:
  - cronjobs
  - jobs
  verbs:
  - get
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps/finalizers
  verbs:
  - update
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - extensions
  resources:
  - daemonsets
  - deployments
  - replicasets
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-metrics-reader
rules:
- nonResourceURLs:
  - /metrics
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-proxy-role
rules:
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
- apiGroups:
  - authorization.k8s.io
  resources:
  - subjectaccessreviews
  verbs:
  - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-session-gate
rules:
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - create
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - create
  - list
  - watch
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - pods/exec
  verbs:
  - create
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps
  verbs:
  - get
  - list
  - watch
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-shell-user
rules:
- apiGroups:
  - ""
  resourceNames:
  - cliapp-shell-context
  resources:
  - configmaps
  verbs:
  - list
  - get
  - watch
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: csi-configmap-warm-metal
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
  - update
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: csi-image-warm-metal
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  - pods
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cliapp-leader-election-rolebinding
  namespace: cliapp-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cliapp-leader-election-role
subjects:
- kind: ServiceAccount
  name: default
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-manager-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-manager-role
subjects:
- kind: ServiceAccount
  name: default
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-proxy-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-proxy-role
subjects:
- kind: ServiceAccount
  name: default
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-session-gate
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-session-gate
subjects:
- kind: ServiceAccount
  name: cliapp-session-gate
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-shell-context-all-allowed
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-shell-user
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:serviceaccounts
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: csi-configmap-warm-metal
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: csi-configmap-warm-metal
subjects:
- kind: ServiceAccount
  name: csi-configmap-warm-metal
  namespace: system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: csi-image-warm-metal
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: csi-image-warm-metal
subjects:
- kind: ServiceAccount
  name: csi-image-warm-metal
  namespace: kube-system
---
apiVersion: v1
data:
  controller_manager_config.yaml: |-
    apiVersion: config.cliapp.warm-metal.tech/v1
    kind: CliAppDefault
    health:
      healthProbeBindAddress: :8081
    metrics:
      bindAddress: 127.0.0.1:8080
    webhook:
      port: 9443
    leaderElection:
      leaderElect: true
      resourceName: 337df6b6.cliapp.warm-metal.tech
    defaultShell: bash
    defaultDistro: alpine
    maxDurationIdleLivesLast: 10m
    builder: tcp://buildkitd:2375
kind: ConfigMap
metadata:
  name: cliapp-manager-config
  namespace: cliapp-system
---
apiVersion: v1
data:
  .bash_history: ""
  .zsh_history: ""
kind: ConfigMap
metadata:
  name: cliapp-shell-context
  namespace: cliapp-system
---
apiVersion: v1
data:
  buildkitd.toml: |-
    debug = true
    # root is where all buildkit state is stored.
    root = "/var/lib/buildkit"
    local-mount-source-root = "/var/lib/buildkit/local-mnt-src"
    # insecure-entitlements allows insecure entitlements, disabled by default.
    insecure-entitlements = [ "network.host", "security.insecure" ]

    [grpc]
      address = [ "unix:///run/buildkit/buildkitd.sock", "tcp://0.0.0.0:2375" ]
      uid = 0
      gid = 0

    [worker.oci]
      enabled = false

    [worker.containerd]
      address = "/run/containerd/containerd.sock"
      enabled = true
      platforms = [ "linux/amd64", "linux/arm64", "linux/arm/v7", "linux/arm/v6", "linux/riscv64", "linux/ppc64le", "linux/s390x", "linux/386", "linux/mips64le", "linux/mips64" ]
      namespace = "k8s.io"
      gc = true
      [[worker.containerd.gcpolicy]]
        keepBytes = 10240000000
        keepDuration = 3600
kind: ConfigMap
metadata:
  name: buildkitd.toml-2fc6k85c68
---
apiVersion: v1
kind: Service
metadata:
  labels:
    control-plane: controller-manager
  name: cliapp-controller-manager-metrics-service
  namespace: cliapp-system
spec:
  ports:
  - name: https
    port: 8443
    targetPort: https
  selector:
    control-plane: controller-manager
---
apiVersion: v1
kind: Service
metadata:
  name: cliapp-session-gate
  namespace: cliapp-system
spec:
  ports:
  - name: session-gate
    port: 8001
    protocol: TCP
    targetPort: 8001
  selector:
    app: session-gate
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  name: buildkitd
  namespace: system
spec:
  ports:
  - name: buildkitd
    port: 2375
    protocol: TCP
    targetPort: 2375
  selector:
    app: builder
    owner: warm-metal.tech
  type: LoadBalancer
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: cliapp-controller-manager
  namespace: cliapp-system
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      containers:
      - args:
        - --config=controller_manager_config.yaml
        command:
        - /manager
        image: docker.io/warmmetal/cliapp-controller:v0.4.0
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        name: manager
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 100m
            memory: 30Mi
          requests:
            cpu: 100m
            memory: 20Mi
        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - mountPath: /controller_manager_config.yaml
          name: manager-config
          subPath: controller_manager_config.yaml
      - args:
        - --secure-listen-address=0.0.0.0:8443
        - --upstream=http://127.0.0.1:8080/
        - --logtostderr=true
        - --v=10
        image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
        name: kube-rbac-proxy
        ports:
        - containerPort: 8443
          name: https
      securityContext:
        runAsUser: 65532
      terminationGracePeriodSeconds: 10
      volumes:
      - configMap:
          name: cliapp-manager-config
        name: manager-config
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: session-gate
  name: cliapp-session-gate
  namespace: cliapp-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: session-gate
  template:
    metadata:
      labels:
        app: session-gate
    spec:
      containers:
      - image: docker.io/warmmetal/session-gate:v0.3.0
        name: session-gate
        ports:
        - containerPort: 8001
      serviceAccountName: cliapp-session-gate
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: builder
    owner: warm-metal.tech
  name: buildkitd
  namespace: system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: builder
      owner: warm-metal.tech
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: builder
        owner: warm-metal.tech
    spec:
      containers:
      - env:
        - name: BUILDKIT_STEP_LOG_MAX_SIZE
          value: "-1"
        image: docker.io/moby/buildkit:latest
        livenessProbe:
          exec:
            command:
            - buildctl
            - debug
            - workers
          failureThreshold: 3
          initialDelaySeconds: 5
          periodSeconds: 30
          successThreshold: 1
          timeoutSeconds: 1
        name: buildkitd
        ports:
        - containerPort: 2375
          name: service
          protocol: TCP
        readinessProbe:
          exec:
            command:
            - buildctl
            - debug
            - workers
          failureThreshold: 3
          initialDelaySeconds: 5
          periodSeconds: 30
          successThreshold: 1
          timeoutSeconds: 1
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /var/lib/containerd
          name: containerd-root
        - mountPath: /var/lib/buildkit
          mountPropagation: Bidirectional
          name: buildkit-root
        - mountPath: /etc/buildkit/buildkitd.toml
          name: buildkitd-conf
          subPath: buildkitd.toml
        - mountPath: /run/containerd
          mountPropagation: Bidirectional
          name: containerd-runtime
      volumes:
      - hostPath:
          path: /var/lib/containerd
          type: Directory
        name: containerd-root
      - hostPath:
          path: /var/lib/buildkit
          type: DirectoryOrCreate
        name: buildkit-root
      - configMap:
          defaultMode: 420
          items:
          - key: buildkitd.toml
            path: buildkitd.toml
          name: buildkitd.toml
        name: buildkitd-conf
      - hostPath:
          path: /run/containerd
          type: Directory
        name: containerd-runtime
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: csi-image-warm-metal
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: csi-image-warm-metal
  template:
    metadata:
      labels:
        app: csi-image-warm-metal
    spec:
      containers:
      - args:
        - --csi-address=/csi/csi.sock
        - --kubelet-registration-path=/var/lib/kubelet/plugins/csi-image.warm-metal.tech/csi.sock
        env:
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: quay.io/k8scsi/csi-node-driver-registrar:v1.1.0
        imagePullPolicy: IfNotPresent
        lifecycle:
          preStop:
            exec:
              command:
              - /bin/sh
              - -c
              - rm -rf /registration/csi-image.warm-metal.tech /registration/csi-image.warm-metal.tech-reg.sock
        name: node-driver-registrar
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /registration
          name: registration-dir
      - args:
        - --endpoint=$(CSI_ENDPOINT)
        - --node=$(KUBE_NODE_NAME)
        - --containerd-addr=$(CRI_ADDR)
        env:
        - name: CSI_ENDPOINT
          value: unix:///csi/csi.sock
        - name: CRI_ADDR
          value: unix:///run/containerd/containerd.sock
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: docker.io/warmmetal/csi-image:v0.5.1
        imagePullPolicy: IfNotPresent
        name: plugin
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /var/lib/kubelet/pods
          mountPropagation: Bidirectional
          name: mountpoint-dir
        - mountPath: /run/containerd/containerd.sock
          name: runtime-socket
        - mountPath: /mnt/vda1/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs
          mountPropagation: Bidirectional
          name: snapshot-root-0
      hostNetwork: true
      serviceAccountName: csi-image-warm-metal
      volumes:
      - hostPath:
          path: /var/lib/kubelet/plugins/csi-image.warm-metal.tech
          type: DirectoryOrCreate
        name: socket-dir
      - hostPath:
          path: /var/lib/kubelet/pods
          type: DirectoryOrCreate
        name: mountpoint-dir
      - hostPath:
          path: /var/lib/kubelet/plugins_registry
          type: Directory
        name: registration-dir
      - hostPath:
          path: /run/containerd/containerd.sock
          type: Socket
        name: runtime-socket
      - hostPath:
          path: /mnt/vda1/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs
          type: Directory
        name: snapshot-root-0
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: csi-configmap-warm-metal
  namespace: system
spec:
  selector:
    matchLabels:
      app: csi-configmap-warm-metal
  template:
    metadata:
      labels:
        app: csi-configmap-warm-metal
    spec:
      containers:
      - args:
        - --csi-address=/csi/csi.sock
        - --kubelet-registration-path=/var/lib/kubelet/plugins/csi-configmap.warm-metal.tech/csi.sock
        env:
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: quay.io/k8scsi/csi-node-driver-registrar:v1.1.0
        imagePullPolicy: IfNotPresent
        lifecycle:
          preStop:
            exec:
              command:
              - /bin/sh
              - -c
              - rm -rf /registration/csi-configmap.warm-metal.tech /registration/csi-configmap.warm-metal.tech-reg.sock
        name: node-driver-registrar
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /registration
          name: registration-dir
      - args:
        - -endpoint=$(CSI_ENDPOINT)
        - -node=$(KUBE_NODE_NAME)
        env:
        - name: CSI_ENDPOINT
          value: unix:///csi/csi.sock
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: docker.io/warmmetal/csi-configmap:v0.2.0
        imagePullPolicy: IfNotPresent
        name: plugin
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /var/lib/kubelet/pods
          mountPropagation: Bidirectional
          name: mountpoint-dir
        - mountPath: /var/lib/warm-metal/cm-volume
          name: cm-source-root
      hostNetwork: true
      serviceAccountName: csi-configmap-warm-metal
      volumes:
      - hostPath:
          path: /var/lib/kubelet/plugins/csi-configmap.warm-metal.tech
          type: DirectoryOrCreate
        name: socket-dir
      - hostPath:
          path: /var/lib/kubelet/pods
          type: DirectoryOrCreate
        name: mountpoint-dir
      - hostPath:
          path: /var/lib/kubelet/plugins_registry
          type: Directory
        name: registration-dir
      - hostPath:
          path: /var/lib/warm-metal/cm-volume
          type: DirectoryOrCreate
        name: cm-source-root
---
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: csi-cm.warm-metal.tech
spec:
  attachRequired: false
  podInfoOnMount: true
  volumeLifecycleModes:
  - Ephemeral
---
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: csi-image.warm-metal.tech
spec:
  attachRequired: false
  podInfoOnMount: true
  volumeLifecycleModes:
  - Persistent
  - Ephemeral
`
const minikubeManifests = `apiVersion: v1
kind: Namespace
metadata:
  labels:
    control-plane: controller-manager
  name: cliapp-system
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.4.1
  creationTimestamp: null
  name: cliapps.core.cliapp.warm-metal.tech
spec:
  group: core.cliapp.warm-metal.tech
  names:
    kind: CliApp
    listKind: CliAppList
    plural: cliapps
    singular: cliapp
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - jsonPath: .spec.targetPhase
      name: TargetPhase
      type: string
    - jsonPath: .status.phase
      name: Phase
      type: string
    - jsonPath: .status.podName
      name: Pod
      type: string
    - jsonPath: .status.error
      name: Error
      type: string
    - jsonPath: .spec.distro
      name: Distro
      type: string
    - jsonPath: .spec.shell
      name: Shell
      type: string
    name: v1
    schema:
      openAPIV3Schema:
        description: CliApp is the Schema for the cliapps API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: CliAppSpec defines the desired state of CliApp
            properties:
              command:
                description: Set the command to be executed when client runs the app. It is usually an executable binary. It should be found in the PATH, or an absolute path to the binary. If no set, session-gate will run commands in the app context rootfs instead of the rootfs of Spec.Image.
                items:
                  type: string
                type: array
              distro:
                description: 'Distro the app dependents. The default is alpine. Valid values are: - "alpine" (default): The app works on Alpine; - "ubuntu: The app works on Ubuntu.'
                enum:
                - alpine
                - ubuntu
                type: string
              dockerfile:
                description: Specify a Dockerfile to build a image used to run the app. Http(s) URI is also supported. Only one of Image or Dockerfile can be set.
                type: string
              env:
                description: Environment variables in the form of "key=value".
                items:
                  type: string
                type: array
              fork:
                description: Specify that the app will fork a workload in the same namespace.
                properties:
                  container:
                    description: Set the target container name if the ForObject has more than one containers.
                    type: string
                  object:
                    description: Specify the kind and name of the object to be forked. The object could be either of Deployment, StatefulSet, DaemonSet, ReplicaSet, (Cron)Job, or Pod. The valid format would be Kind/Name.
                    type: string
                  withEnvs:
                    description: Set if expected to inherit envs from the original workload
                    type: boolean
                type: object
              hostpath:
                description: Host paths would be mounted to the app. Each HostPath can be an absolute host path, or in the form of "hostpath:mount-point".
                items:
                  type: string
                type: array
              image:
                description: Specify the image the app uses. Only one of Image or Dockerfile can be set.
                type: string
              shell:
                description: 'The shell interpreter you preferred. Can be either bash or zsh. Valid values are: - "bash" (default): The app will run in Bash; - "zsh: The app will run in Zsh.'
                enum:
                - bash
                - zsh
                type: string
              targetPhase:
                description: 'The target phase the app should achieve. Valid values are: - "Rest" (default): The app is installed but not started; - "Live": The app is running.'
                enum:
                - Rest
                - Recovering
                - Building
                - Live
                - WaitingForSessions
                - ShuttingDown
                type: string
              uninstall:
                description: Set if uninstalls the App when it transits out of phase Live
                type: boolean
            type: object
          status:
            description: CliAppStatus defines the observed state of CliApp
            properties:
              error:
                description: Specify Errors on reconcile.
                type: string
              lastPhaseTransition:
                description: Timestamp of the last phase transition
                format: date-time
                type: string
              phase:
                description: 'Show the app state. Valid values are: - "Rest" (default): The app is installed but not started; - "Recovering": The app is starting; - "Building": The app is waiting for image building; - "Live": The app is running; - "WaitingForSessions": The app is waiting for new sessions and will be shutdown later; - "ShuttingDown": The app is shutting down.'
                enum:
                - Rest
                - Recovering
                - Building
                - Live
                - WaitingForSessions
                - ShuttingDown
                type: string
              podName:
                description: Specify the Pod name if app is in phase Live.
                type: string
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cliapp-session-gate
  namespace: cliapp-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: csi-configmap-warm-metal
  namespace: cliapp-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: csi-image-warm-metal
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cliapp-leader-election-role
  namespace: cliapp-system
rules:
- apiGroups:
  - ""
  - coordination.k8s.io
  resources:
  - configmaps
  - leases
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - events
  verbs:
  - create
  - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: cliapp-manager-role
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - create
  - delete
  - deletecollection
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - apps
  resources:
  - daemonsets
  - deployments
  - replicasets
  - statefulsets
  verbs:
  - get
- apiGroups:
  - batch
  resources:
  - cronjobs
  - jobs
  verbs:
  - get
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps/finalizers
  verbs:
  - update
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps/status
  verbs:
  - get
  - patch
  - update
- apiGroups:
  - extensions
  resources:
  - daemonsets
  - deployments
  - replicasets
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-metrics-reader
rules:
- nonResourceURLs:
  - /metrics
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-proxy-role
rules:
- apiGroups:
  - authentication.k8s.io
  resources:
  - tokenreviews
  verbs:
  - create
- apiGroups:
  - authorization.k8s.io
  resources:
  - subjectaccessreviews
  verbs:
  - create
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-session-gate
rules:
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - get
  - create
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
  - create
  - list
  - watch
  - update
  - patch
  - delete
- apiGroups:
  - ""
  resources:
  - pods/exec
  verbs:
  - create
- apiGroups:
  - core.cliapp.warm-metal.tech
  resources:
  - cliapps
  verbs:
  - get
  - list
  - watch
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cliapp-shell-user
rules:
- apiGroups:
  - ""
  resourceNames:
  - cliapp-shell-context
  resources:
  - configmaps
  verbs:
  - list
  - get
  - watch
  - update
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: csi-configmap-warm-metal
rules:
- apiGroups:
  - ""
  resources:
  - configmaps
  verbs:
  - get
  - list
  - watch
  - update
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: csi-image-warm-metal
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  - pods
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cliapp-leader-election-rolebinding
  namespace: cliapp-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cliapp-leader-election-role
subjects:
- kind: ServiceAccount
  name: default
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-manager-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-manager-role
subjects:
- kind: ServiceAccount
  name: default
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-proxy-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-proxy-role
subjects:
- kind: ServiceAccount
  name: default
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-session-gate
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-session-gate
subjects:
- kind: ServiceAccount
  name: cliapp-session-gate
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cliapp-shell-context-all-allowed
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cliapp-shell-user
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:serviceaccounts
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: csi-configmap-warm-metal
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: csi-configmap-warm-metal
subjects:
- kind: ServiceAccount
  name: csi-configmap-warm-metal
  namespace: cliapp-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: csi-image-warm-metal
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: csi-image-warm-metal
subjects:
- kind: ServiceAccount
  name: csi-image-warm-metal
  namespace: cliapp-system
---
apiVersion: v1
data:
  buildkitd.toml: |-
    debug = true
    # root is where all buildkit state is stored.
    root = "/var/lib/buildkit"
    local-mount-source-root = "/var/lib/buildkit/local-mnt-src"
    # insecure-entitlements allows insecure entitlements, disabled by default.
    insecure-entitlements = [ "network.host", "security.insecure" ]

    [grpc]
      address = [ "unix:///run/buildkit/buildkitd.sock", "tcp://0.0.0.0:2375" ]
      uid = 0
      gid = 0

    [worker.oci]
      enabled = false

    [worker.containerd]
      address = "/run/containerd/containerd.sock"
      enabled = true
      platforms = [ "linux/amd64", "linux/arm64", "linux/arm/v7", "linux/arm/v6", "linux/riscv64", "linux/ppc64le", "linux/s390x", "linux/386", "linux/mips64le", "linux/mips64" ]
      namespace = "k8s.io"
      gc = true
      [[worker.containerd.gcpolicy]]
        keepBytes = 10240000000
        keepDuration = 3600
kind: ConfigMap
metadata:
  name: buildkitd.toml-2fc6k85c68
  namespace: cliapp-system
---
apiVersion: v1
data:
  controller_manager_config.yaml: |-
    apiVersion: config.cliapp.warm-metal.tech/v1
    kind: CliAppDefault
    health:
      healthProbeBindAddress: :8081
    metrics:
      bindAddress: 127.0.0.1:8080
    webhook:
      port: 9443
    leaderElection:
      leaderElect: true
      resourceName: 337df6b6.cliapp.warm-metal.tech
    defaultShell: bash
    defaultDistro: alpine
    maxDurationIdleLivesLast: 10m
    builder: tcp://buildkitd:2375
kind: ConfigMap
metadata:
  name: cliapp-manager-config
  namespace: cliapp-system
---
apiVersion: v1
data:
  .bash_history: ""
  .zsh_history: ""
kind: ConfigMap
metadata:
  name: cliapp-shell-context
  namespace: cliapp-system
---
apiVersion: v1
kind: Service
metadata:
  name: buildkitd
  namespace: cliapp-system
spec:
  ports:
  - name: buildkitd
    port: 2375
    protocol: TCP
    targetPort: 2375
  selector:
    app: builder
    owner: warm-metal.tech
  type: LoadBalancer
---
apiVersion: v1
kind: Service
metadata:
  labels:
    control-plane: controller-manager
  name: cliapp-controller-manager-metrics-service
  namespace: cliapp-system
spec:
  ports:
  - name: https
    port: 8443
    targetPort: https
  selector:
    control-plane: controller-manager
---
apiVersion: v1
kind: Service
metadata:
  name: cliapp-session-gate
  namespace: cliapp-system
spec:
  ports:
  - name: session-gate
    port: 8001
    protocol: TCP
    targetPort: 8001
  selector:
    app: session-gate
  type: LoadBalancer
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: builder
    owner: warm-metal.tech
  name: buildkitd
  namespace: cliapp-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: builder
      owner: warm-metal.tech
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: builder
        owner: warm-metal.tech
    spec:
      containers:
      - env:
        - name: BUILDKIT_STEP_LOG_MAX_SIZE
          value: "-1"
        image: docker.io/moby/buildkit:latest
        livenessProbe:
          exec:
            command:
            - buildctl
            - debug
            - workers
          failureThreshold: 3
          initialDelaySeconds: 5
          periodSeconds: 30
          successThreshold: 1
          timeoutSeconds: 1
        name: buildkitd
        ports:
        - containerPort: 2375
          name: service
          protocol: TCP
        readinessProbe:
          exec:
            command:
            - buildctl
            - debug
            - workers
          failureThreshold: 3
          initialDelaySeconds: 5
          periodSeconds: 30
          successThreshold: 1
          timeoutSeconds: 1
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /mnt/vda1/var/lib/containerd
          name: containerd-root
        - mountPath: /var/lib/buildkit
          mountPropagation: Bidirectional
          name: buildkit-root
        - mountPath: /etc/buildkit/buildkitd.toml
          name: buildkitd-conf
          subPath: buildkitd.toml
        - mountPath: /run/containerd
          mountPropagation: Bidirectional
          name: containerd-runtime
      volumes:
      - hostPath:
          path: /mnt/vda1/var/lib/containerd
          type: Directory
        name: containerd-root
      - hostPath:
          path: /var/lib/buildkit
          type: DirectoryOrCreate
        name: buildkit-root
      - configMap:
          defaultMode: 420
          items:
          - key: buildkitd.toml
            path: buildkitd.toml
          name: buildkitd.toml-2fc6k85c68
        name: buildkitd-conf
      - hostPath:
          path: /run/containerd
          type: Directory
        name: containerd-runtime
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    control-plane: controller-manager
  name: cliapp-controller-manager
  namespace: cliapp-system
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      containers:
      - args:
        - --config=controller_manager_config.yaml
        command:
        - /manager
        image: docker.io/warmmetal/cliapp-controller:v0.4.0
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 20
        name: manager
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          limits:
            cpu: 100m
            memory: 30Mi
          requests:
            cpu: 100m
            memory: 20Mi
        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - mountPath: /controller_manager_config.yaml
          name: manager-config
          subPath: controller_manager_config.yaml
      - args:
        - --secure-listen-address=0.0.0.0:8443
        - --upstream=http://127.0.0.1:8080/
        - --logtostderr=true
        - --v=10
        image: gcr.io/kubebuilder/kube-rbac-proxy:v0.5.0
        name: kube-rbac-proxy
        ports:
        - containerPort: 8443
          name: https
      securityContext:
        runAsUser: 65532
      terminationGracePeriodSeconds: 10
      volumes:
      - configMap:
          name: cliapp-manager-config
        name: manager-config
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: session-gate
  name: cliapp-session-gate
  namespace: cliapp-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: session-gate
  template:
    metadata:
      labels:
        app: session-gate
    spec:
      containers:
      - image: docker.io/warmmetal/session-gate:v0.3.0
        name: session-gate
        ports:
        - containerPort: 8001
      serviceAccountName: cliapp-session-gate
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: csi-configmap-warm-metal
  namespace: cliapp-system
spec:
  selector:
    matchLabels:
      app: csi-configmap-warm-metal
  template:
    metadata:
      labels:
        app: csi-configmap-warm-metal
    spec:
      containers:
      - args:
        - --csi-address=/csi/csi.sock
        - --kubelet-registration-path=/var/lib/kubelet/plugins/csi-configmap.warm-metal.tech/csi.sock
        env:
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: quay.io/k8scsi/csi-node-driver-registrar:v1.1.0
        imagePullPolicy: IfNotPresent
        lifecycle:
          preStop:
            exec:
              command:
              - /bin/sh
              - -c
              - rm -rf /registration/csi-configmap.warm-metal.tech /registration/csi-configmap.warm-metal.tech-reg.sock
        name: node-driver-registrar
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /registration
          name: registration-dir
      - args:
        - -endpoint=$(CSI_ENDPOINT)
        - -node=$(KUBE_NODE_NAME)
        env:
        - name: CSI_ENDPOINT
          value: unix:///csi/csi.sock
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: docker.io/warmmetal/csi-configmap:v0.2.0
        imagePullPolicy: IfNotPresent
        name: plugin
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /var/lib/kubelet/pods
          mountPropagation: Bidirectional
          name: mountpoint-dir
        - mountPath: /var/lib/warm-metal/cm-volume
          name: cm-source-root
      hostNetwork: true
      serviceAccountName: csi-configmap-warm-metal
      volumes:
      - hostPath:
          path: /var/lib/kubelet/plugins/csi-configmap.warm-metal.tech
          type: DirectoryOrCreate
        name: socket-dir
      - hostPath:
          path: /var/lib/kubelet/pods
          type: DirectoryOrCreate
        name: mountpoint-dir
      - hostPath:
          path: /var/lib/kubelet/plugins_registry
          type: Directory
        name: registration-dir
      - hostPath:
          path: /var/lib/warm-metal/cm-volume
          type: DirectoryOrCreate
        name: cm-source-root
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: csi-image-warm-metal
  namespace: cliapp-system
spec:
  selector:
    matchLabels:
      app: csi-image-warm-metal
  template:
    metadata:
      labels:
        app: csi-image-warm-metal
    spec:
      containers:
      - args:
        - --csi-address=/csi/csi.sock
        - --kubelet-registration-path=/var/lib/kubelet/plugins/csi-image.warm-metal.tech/csi.sock
        env:
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: quay.io/k8scsi/csi-node-driver-registrar:v1.1.0
        imagePullPolicy: IfNotPresent
        lifecycle:
          preStop:
            exec:
              command:
              - /bin/sh
              - -c
              - rm -rf /registration/csi-image.warm-metal.tech /registration/csi-image.warm-metal.tech-reg.sock
        name: node-driver-registrar
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /registration
          name: registration-dir
      - args:
        - --endpoint=$(CSI_ENDPOINT)
        - --node=$(KUBE_NODE_NAME)
        - --containerd-addr=$(CRI_ADDR)
        env:
        - name: CSI_ENDPOINT
          value: unix:///csi/csi.sock
        - name: CRI_ADDR
          value: unix:///run/containerd/containerd.sock
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        image: docker.io/warmmetal/csi-image:v0.5.1
        imagePullPolicy: IfNotPresent
        name: plugin
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi
          name: socket-dir
        - mountPath: /var/lib/kubelet/pods
          mountPropagation: Bidirectional
          name: mountpoint-dir
        - mountPath: /run/containerd/containerd.sock
          name: runtime-socket
        - mountPath: /mnt/vda1/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs
          mountPropagation: Bidirectional
          name: snapshot-root-0
      hostNetwork: true
      serviceAccountName: csi-image-warm-metal
      volumes:
      - hostPath:
          path: /var/lib/kubelet/plugins/csi-image.warm-metal.tech
          type: DirectoryOrCreate
        name: socket-dir
      - hostPath:
          path: /var/lib/kubelet/pods
          type: DirectoryOrCreate
        name: mountpoint-dir
      - hostPath:
          path: /var/lib/kubelet/plugins_registry
          type: Directory
        name: registration-dir
      - hostPath:
          path: /run/containerd/containerd.sock
          type: Socket
        name: runtime-socket
      - hostPath:
          path: /mnt/vda1/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs
          type: Directory
        name: snapshot-root-0
---
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: csi-cm.warm-metal.tech
  namespace: cliapp-system
spec:
  attachRequired: false
  podInfoOnMount: true
  volumeLifecycleModes:
  - Ephemeral
---
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: csi-image.warm-metal.tech
  namespace: cliapp-system
spec:
  attachRequired: false
  podInfoOnMount: true
  volumeLifecycleModes:
  - Persistent
  - Ephemeral
`
