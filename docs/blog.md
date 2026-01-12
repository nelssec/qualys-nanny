# Deploying Qualys Security Agents on OpenShift with the Qualys Nanny Operator

Securing containerized workloads requires visibility at multiple layers: the host operating system, container images, runtime behavior, and the orchestration platform itself. This post walks through deploying the complete Qualys security stack on OpenShift clusters using the Qualys Nanny Operator.

## Architecture Overview

The Qualys Nanny Operator manages two primary custom resources that deploy five distinct security components:

```mermaid
flowchart TB
    subgraph cluster[OpenShift Cluster]
        OPERATOR[Qualys Nanny Operator]

        subgraph control[Control Plane]
            CLUSTER[Cluster Sensor]
            ADMISSION[Admission Controller]
        end

        subgraph node1[Worker Node 1]
            CA1[Cloud Agent]
            CS1[Container Sensor]
            RT1[Runtime Sensor]
        end

        subgraph node2[Worker Node 2]
            CA2[Cloud Agent]
            CS2[Container Sensor]
            RT2[Runtime Sensor]
        end
    end

    QP[Qualys Platform]

    OPERATOR --> CA1
    OPERATOR --> CA2
    OPERATOR --> CS1
    OPERATOR --> CS2
    OPERATOR --> RT1
    OPERATOR --> RT2
    OPERATOR --> CLUSTER
    OPERATOR --> ADMISSION

    CA1 --> QP
    CA2 --> QP
    CS1 --> QP
    CS2 --> QP
    RT1 --> QP
    RT2 --> QP
    CLUSTER --> QP
    ADMISSION --> QP
```

## Custom Resource Hierarchy

The operator uses three CRDs with a hierarchical relationship:

```mermaid
flowchart LR
    subgraph CRDs
        PC[QualysPlatformConfig]
        CA[QualysCloudAgent]
        CS[QualysContainerSecurity]
    end

    subgraph Credentials
        SEC[Secret]
        ESO[ExternalSecret]
    end

    subgraph "Host Resources"
        DS1[Cloud Agent DaemonSet]
        SA1[ServiceAccount]
        CM1[ConfigMap]
        SCC1[SCC]
    end

    subgraph "Container Resources"
        DS2[Container Sensor DaemonSet]
        DS3[Runtime Sensor DaemonSet]
        DEP1[Cluster Sensor Deployment]
        DEP2[Admission Controller Deployment]
        WH[ValidatingWebhook]
    end

    PC --> SEC
    PC --> ESO
    CA --> PC
    CS --> PC
    CA --> DS1
    CA --> SA1
    CA --> CM1
    CA --> SCC1
    CS --> DS2
    CS --> DS3
    CS --> DEP1
    CS --> DEP2
    DEP2 --> WH
```

## Security Components

### Component Overview

| Component | CR | Type | Purpose |
|-----------|-----|------|---------|
| Cloud Agent | QualysCloudAgent | DaemonSet | Host-level vulnerability and compliance scanning |
| Container Sensor | QualysContainerSecurity | DaemonSet | Container image and runtime vulnerability scanning |
| Cluster Sensor | QualysContainerSecurity | Deployment | K8s API monitoring, workload inventory, network activity |
| Admission Controller | QualysContainerSecurity | Deployment + Webhook | Security policy enforcement on resource creation |
| Runtime Sensor | QualysContainerSecurity | DaemonSet | eBPF-based file and process event tracking |

### Default Configuration

| Component | Default State |
|-----------|---------------|
| Cloud Agent | Enabled (separate CR) |
| Container Sensor | Enabled |
| Cluster Sensor | Enabled |
| Admission Controller | Disabled |
| Runtime Sensor | Disabled |

## Qualys Cloud Agent on CoreOS and RHEL

The Cloud Agent performs host-level vulnerability management and compliance scanning. On immutable operating systems like CoreOS, traditional agent installation isn't possible. The operator solves this by running the agent as a privileged container with host access.

### Deployment Modes

```mermaid
flowchart TD
    START[Deploy QualysCloudAgent]
    MODE{deploymentMode?}
    AUTO[Auto-detect OS]
    BOOT[Bootstrapper Mode]
    CORE[CoreOS Mode]
    DETECT{Node OS?}

    START --> MODE
    MODE -->|auto| AUTO
    MODE -->|bootstrapper| BOOT
    MODE -->|coreos| CORE
    AUTO --> DETECT
    DETECT -->|CoreOS/RHCOS/Flatcar| CORE
    DETECT -->|RHEL/CentOS/Debian/Ubuntu| BOOT
```

| Mode | OS Types | Image | Description |
|------|----------|-------|-------------|
| `bootstrapper` | RHEL, CentOS, Debian, Ubuntu | `nelssec/qualys-agent-bootstrapper` | Installs agent on host via nsenter |
| `coreos` | CoreOS, RHCOS, Flatcar | `qualys/qagent-rhcos` | Runs agent in container (immutable OS) |

### Host Mounts Required

```mermaid
flowchart LR
    AGENT[Cloud Agent Pod]

    AGENT --> ETC[/etc]
    AGENT --> VAR[/var]
    AGENT --> PROC[/proc]
    AGENT --> SYS[/sys]
    AGENT --> OPT[/opt/qualys]
```

### CoreOS Considerations

CoreOS uses an immutable root filesystem with atomic updates. The agent handles this by:

- Storing persistent data in `/opt/qualys` (mounted from the host)
- Using the host's `/etc/machine-id` for consistent host identification
- Running as a privileged container to access host namespaces

## Container Security Components

The QualysContainerSecurity CR manages four components that can be independently enabled or disabled.

### Container Sensor (DaemonSet)

Scans container images and running containers for vulnerabilities.

```mermaid
flowchart TB
    subgraph node[OpenShift Node]
        SENSOR[Container Sensor]
        SOCKET[CRI-O Socket]
        IMAGES[Image Store]

        subgraph workloads[Workload Pods]
            APP1[App 1]
            APP2[App 2]
            APP3[App 3]
        end
    end

    SENSOR --> SOCKET
    SENSOR --> IMAGES
    SOCKET --> workloads
```

**Features:**
- Image vulnerability scanning
- Running container scanning
- Malware detection (optional)
- Secret detection (optional)

### Cluster Sensor (Deployment)

Monitors the Kubernetes API server for cluster-wide visibility.

```mermaid
flowchart LR
    subgraph cluster[Cluster]
        API[K8s API Server]
        CS[Cluster Sensor]
    end

    QP[Qualys Platform]

    CS --> API
    CS --> QP

    API -->|Events| CS
    API -->|Workloads| CS
    API -->|Network Policies| CS
```

**Features:**
- Cluster event monitoring
- Workload inventory
- Network policy analysis
- RBAC visibility

### Admission Controller (Deployment + Webhook)

Enforces security policies on resource creation and updates.

```mermaid
sequenceDiagram
    participant User
    participant API as K8s API
    participant WH as Admission Webhook
    participant AC as Admission Controller
    participant QP as Qualys Platform

    User->>API: Create Pod
    API->>WH: Validate Request
    WH->>AC: Check Policy
    AC->>QP: Lookup Vulnerabilities
    QP-->>AC: Policy Decision
    AC-->>WH: Allow/Deny
    WH-->>API: Response
    API-->>User: Result
```

**Features:**
- Block deployment of vulnerable images
- Enforce security baselines
- Configurable failure policy (Fail/Ignore)
- Namespace-scoped enforcement

### Runtime Sensor (DaemonSet)

Uses eBPF for kernel-level visibility into container behavior.

```mermaid
flowchart TB
    subgraph node[Node Kernel]
        EBPF[eBPF Programs]
        subgraph events[Kernel Events]
            FILE[File Access]
            PROC[Process Exec]
            NET[Network]
        end
    end

    RT[Runtime Sensor]
    QP[Qualys Platform]

    EBPF --> events
    RT --> EBPF
    RT --> QP
```

**Features:**
- Real-time file integrity monitoring
- Process execution tracking
- Network connection visibility
- Behavioral anomaly detection

## Installation

### 1. Create Namespace and Credentials

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: qualys
---
apiVersion: v1
kind: Secret
metadata:
  name: qualys-credentials
  namespace: qualys
type: Opaque
stringData:
  ACTIVATION_ID: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  CUSTOMER_ID: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

### 2. Create Platform Configuration

```yaml
apiVersion: qualys.qualys.io/v1alpha1
kind: QualysPlatformConfig
metadata:
  name: qualys-platform
spec:
  platform:
    serverUri: "https://qagpublic.qg2.apps.qualys.com/CloudAgent/"
  credentials:
    sourceType: secret
    secretRef:
      name: qualys-credentials
      namespace: qualys
```

### 3. Deploy Cloud Agent (Host Scanning)

```yaml
apiVersion: qualys.qualys.io/v1alpha1
kind: QualysCloudAgent
metadata:
  name: qualys-cloud-agent
  namespace: qualys
spec:
  platformConfigRef:
    name: qualys-platform
  deploymentMode: auto
  config:
    logLevel: 3
  scheduling:
    tolerations:
      - operator: Exists
```

### 4. Deploy Container Security

```yaml
apiVersion: qualys.qualys.io/v1alpha1
kind: QualysContainerSecurity
metadata:
  name: qualys-container-security
  namespace: qualys
spec:
  platformConfigRef:
    name: qualys-platform
  containerSensor:
    enabled: true
    mode: general
    k8sMode: true
    scanning:
      enableImageScan: true
      enableContainerScan: true
  clusterSensor:
    enabled: true
    replicas: 1
  admissionController:
    enabled: false
    failurePolicy: Ignore
  runtimeSensor:
    enabled: false
  containerRuntime:
    type: auto
```

## Reconciliation Flow

The operator continuously reconciles the desired state with the actual state:

```mermaid
flowchart LR
    A[CR Created] --> B[Validate Config]
    B --> C{Credentials Ready?}
    C -->|No| D[Wait]
    D --> C
    C -->|Yes| E[Reconcile Components]
    E --> F[Container Sensor]
    E --> G[Cluster Sensor]
    E --> H[Admission Controller]
    E --> I[Runtime Sensor]
    F --> J{All Ready?}
    G --> J
    H --> J
    I --> J
    J -->|No| K[Degraded]
    J -->|Yes| L[Available]
    K --> E
    L --> M[Watch for Changes]
    M --> B
```

## Security Context Constraints

On OpenShift, the operator automatically creates SCCs to grant the required privileges:

```mermaid
flowchart LR
    OP[Operator]

    subgraph sccs[Created SCCs]
        SCC1[Cloud Agent SCC]
        SCC2[Container Sensor SCC]
        SCC3[Runtime Sensor SCC]
    end

    subgraph sas[Service Accounts]
        SA1[cloud-agent-sa]
        SA2[container-sensor-sa]
        SA3[runtime-sensor-sa]
    end

    OP --> SCC1
    OP --> SCC2
    OP --> SCC3
    SCC1 --> SA1
    SCC2 --> SA2
    SCC3 --> SA3
```

**Cloud Agent SCC grants:**
- `privileged: true`
- `hostPID: true`
- `SYS_ADMIN` capability

**Container Sensor SCC grants:**
- `hostNetwork: true`
- Container runtime socket access

**Runtime Sensor SCC grants:**
- `privileged: true`
- eBPF capabilities

## Monitoring Deployment Status

```bash
kubectl get qualysplatformconfig qualys-platform -o yaml
kubectl get qualyscloudagent -n qualys -o wide
kubectl get qualyscontainersecurity -n qualys -o wide
kubectl get pods -n qualys -o wide
```

Example output:

```
NAME                       CONTAINER   CLUSTER   ADMISSION   RUNTIME   AGE
qualys-container-security  true        true      false       false     2h
```

## Data Flow to Qualys Platform

```mermaid
sequenceDiagram
    participant CA as Cloud Agent
    participant CS as Container Sensor
    participant CL as Cluster Sensor
    participant RT as Runtime Sensor
    participant QP as Qualys Platform

    loop Host Scanning
        CA->>CA: Scan packages
        CA->>QP: Upload VM findings
    end

    loop Container Monitoring
        CS->>CS: Scan images
        CS->>QP: Upload vulnerabilities
    end

    loop Cluster Monitoring
        CL->>CL: Watch K8s events
        CL->>QP: Upload inventory
    end

    loop Runtime Monitoring
        RT->>RT: Capture eBPF events
        RT->>QP: Upload behaviors
    end

    QP->>QP: Correlate data
```

## Troubleshooting

### Component Not Starting

```mermaid
flowchart TD
    START[Component not starting]
    A{SCC exists?}
    B[Check operator logs]
    C{Secret exists?}
    D[Create secret]
    E{Config ready?}
    F[Fix PlatformConfig]
    G[Check component status]

    START --> A
    A -->|No| B
    A -->|Yes| C
    C -->|No| D
    C -->|Yes| E
    E -->|No| F
    E -->|Yes| G
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Pods pending | Missing SCC | Check operator logs for SCC creation errors |
| CrashLoopBackOff | Invalid credentials | Verify ACTIVATION_ID and CUSTOMER_ID |
| No data in Qualys | Network blocked | Check firewall rules for Qualys platform URLs |
| ImagePullBackOff | Registry access | Configure imagePullSecrets |
| Admission webhook errors | Service unavailable | Check admission controller pod logs |

## Conclusion

The Qualys Nanny Operator simplifies deploying the complete Qualys security stack across OpenShift clusters by:

- Managing five security components through two custom resources
- Supporting component-level enable/disable toggles
- Handling the complexity of privileged container configuration
- Automatically managing OpenShift SCCs
- Supporting both immutable (CoreOS) and traditional (RHEL) node operating systems
- Providing Kubernetes-native status reporting through CR conditions

For more information, see the [Qualys Nanny GitHub repository](https://github.com/nelssec/qualys-nanny).
