# Deploying Qualys Security Agents on OpenShift with the Qualys Nanny Operator

Securing containerized workloads requires visibility at multiple layers: the host operating system, the container runtime, and the orchestration platform. This post walks through deploying Qualys Cloud Agent for host-level scanning on CoreOS and RHEL nodes, and the Qualys Container Security Sensor for container and image scanning on OpenShift clusters.

## Architecture Overview

The Qualys Nanny Operator manages two distinct security agents through Kubernetes-native custom resources:

```mermaid
flowchart TB
    subgraph OpenShift Cluster
        subgraph Control Plane
            API[API Server]
            OPERATOR[Qualys Nanny Operator]
        end

        subgraph Worker Nodes
            subgraph CoreOS/RHEL Node 1
                CA1[Cloud Agent Pod]
                CS1[Container Sensor Pod]
                CRI1[CRI-O Runtime]
                HOST1[Host OS]
            end

            subgraph CoreOS/RHEL Node 2
                CA2[Cloud Agent Pod]
                CS2[Container Sensor Pod]
                CRI2[CRI-O Runtime]
                HOST2[Host OS]
            end
        end
    end

    subgraph Qualys Platform
        QP[Qualys Cloud Platform]
    end

    OPERATOR --> CA1
    OPERATOR --> CA2
    OPERATOR --> CS1
    OPERATOR --> CS2

    CA1 -.->|VM/Compliance Data| QP
    CA2 -.->|VM/Compliance Data| QP
    CS1 -.->|Image/Container Data| QP
    CS2 -.->|Image/Container Data| QP

    CA1 -->|Scans| HOST1
    CA2 -->|Scans| HOST2
    CS1 -->|Monitors| CRI1
    CS2 -->|Monitors| CRI2
```

## Custom Resource Hierarchy

The operator uses three CRDs with a hierarchical relationship:

```mermaid
erDiagram
    QualysPlatformConfig ||--o{ QualysCloudAgent : "referenced by"
    QualysPlatformConfig ||--o{ QualysContainerSecurity : "referenced by"
    QualysPlatformConfig ||--|| Secret : "reads credentials"
    QualysPlatformConfig ||--o| ExternalSecret : "or reads from"

    QualysCloudAgent ||--|| DaemonSet : creates
    QualysCloudAgent ||--|| ServiceAccount : creates
    QualysCloudAgent ||--|| ConfigMap : creates
    QualysCloudAgent ||--|| SCC : "creates on OpenShift"

    QualysContainerSecurity ||--|| DaemonSet : creates
    QualysContainerSecurity ||--|| ServiceAccount : creates
    QualysContainerSecurity ||--|| ClusterRole : creates
    QualysContainerSecurity ||--|| SCC : "creates on OpenShift"
```

## Qualys Cloud Agent on CoreOS and RHEL

The Cloud Agent performs host-level vulnerability management and compliance scanning. On immutable operating systems like CoreOS, traditional agent installation isn't possible. The operator solves this by running the agent as a privileged container with host access.

### How It Works

```mermaid
sequenceDiagram
    participant User
    participant API as K8s API
    participant Op as Operator
    participant DS as DaemonSet
    participant Pod as Agent Pod
    participant Host as CoreOS/RHEL Host
    participant QP as Qualys Platform

    User->>API: Create QualysCloudAgent CR
    API->>Op: Reconcile event
    Op->>Op: Validate PlatformConfig
    Op->>API: Create ServiceAccount
    Op->>API: Create SCC (OpenShift)
    Op->>API: Create ConfigMap
    Op->>API: Create DaemonSet
    DS->>Pod: Schedule on each node
    Pod->>Host: Mount /etc, /var, /proc, /sys
    Pod->>Host: Run with SYS_ADMIN capability
    loop Every scan interval
        Pod->>Host: Scan packages, configs
        Pod->>QP: Report findings
    end
```

### Host Mounts Required

The Cloud Agent requires extensive host access to perform vulnerability and compliance scanning:

```mermaid
flowchart LR
    subgraph Agent Pod
        AGENT[qualys-cloud-agent]
    end

    subgraph Host Filesystem
        ETC[/etc - OS configs]
        VAR[/var - Package DBs]
        PROC[/proc - Process info]
        SYS[/sys - Kernel params]
        OPT[/opt/qualys - Agent data]
        HOSTID[/etc/machine-id]
    end

    AGENT --> ETC
    AGENT --> VAR
    AGENT --> PROC
    AGENT --> SYS
    AGENT --> OPT
    AGENT --> HOSTID
```

### Installation Steps

1. **Create the namespace and credentials:**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: qualys-system
---
apiVersion: v1
kind: Secret
metadata:
  name: qualys-credentials
  namespace: qualys-system
type: Opaque
stringData:
  ACTIVATION_ID: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  CUSTOMER_ID: "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

2. **Create the platform configuration:**

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
      namespace: qualys-system
```

3. **Deploy the Cloud Agent:**

```yaml
apiVersion: qualys.qualys.io/v1alpha1
kind: QualysCloudAgent
metadata:
  name: qualys-cloud-agent
  namespace: qualys-system
spec:
  platformConfigRef:
    name: qualys-platform
  image:
    repository: nelssec/qualys-agent-bootstrapper
    tag: v2.1.0
  config:
    logLevel: 3
  scheduling:
    tolerations:
      - operator: Exists
```

### CoreOS Considerations

CoreOS uses an immutable root filesystem with atomic updates. The agent handles this by:

- Storing persistent data in `/opt/qualys` (mounted from the host)
- Using the host's `/etc/machine-id` for consistent host identification
- Running as a privileged container to access host namespaces

```mermaid
flowchart TB
    subgraph CoreOS Node
        subgraph Read-Only Root
            OS[/usr - Immutable OS]
        end

        subgraph Writable Paths
            ETC[/etc - Configs]
            VAR[/var - Data]
            OPT[/opt - Extensions]
        end

        subgraph Agent Pod
            QA[Qualys Agent]
        end
    end

    QA -->|Reads| OS
    QA -->|Reads| ETC
    QA -->|Reads| VAR
    QA -->|Writes| OPT
```

## Container Security Sensor on OpenShift

The Container Security Sensor monitors container images and running containers for vulnerabilities, malware, and secrets.

### Runtime Detection

The operator automatically detects the container runtime on OpenShift:

```mermaid
flowchart TD
    START[Start Reconciliation]
    CHECK_SPEC{Runtime specified in CR?}
    USE_SPEC[Use specified runtime]
    CHECK_OCP{Is OpenShift?}
    USE_CRIO[Use CRI-O]
    CHECK_NODE[Query node info]
    PARSE[Parse containerRuntimeVersion]

    START --> CHECK_SPEC
    CHECK_SPEC -->|Yes| USE_SPEC
    CHECK_SPEC -->|No| CHECK_OCP
    CHECK_OCP -->|Yes| USE_CRIO
    CHECK_OCP -->|No| CHECK_NODE
    CHECK_NODE --> PARSE

    PARSE -->|containerd://| CONTAINERD[Use containerd socket]
    PARSE -->|cri-o://| CRIO2[Use CRI-O socket]
    PARSE -->|docker://| DOCKER[Use Docker socket]
```

### Sensor Architecture

```mermaid
flowchart TB
    subgraph OpenShift Node
        subgraph Container Sensor Pod
            SENSOR[QCS Sensor]
            SCANNER[Image Scanner]
            MONITOR[Runtime Monitor]
        end

        subgraph CRI-O Runtime
            SOCKET[/var/run/crio/crio.sock]
            IMAGES[(Image Store)]
            CONTAINERS[(Running Containers)]
        end

        subgraph Workload Pods
            APP1[App Container 1]
            APP2[App Container 2]
            APP3[App Container 3]
        end
    end

    SENSOR --> SOCKET
    SCANNER --> IMAGES
    MONITOR --> CONTAINERS

    CONTAINERS -.-> APP1
    CONTAINERS -.-> APP2
    CONTAINERS -.-> APP3
```

### Installation Steps

1. **Ensure platform config exists** (from previous section)

2. **Deploy the Container Security Sensor:**

```yaml
apiVersion: qualys.qualys.io/v1alpha1
kind: QualysContainerSecurity
metadata:
  name: qualys-container-sensor
  namespace: qualys-system
spec:
  platformConfigRef:
    name: qualys-platform
  image:
    repository: qualys/qcs-sensor
    tag: 1.22.0
  containerRuntime:
    type: auto
  sensorConfig:
    mode: general
    k8sMode: true
    scanning:
      enableImageScan: true
      enableContainerScan: true
```

### Security Context Constraints

On OpenShift, the operator automatically creates SCCs to grant the required privileges:

```mermaid
flowchart LR
    subgraph Operator
        RECONCILE[Reconcile Loop]
    end

    subgraph OpenShift API
        SCC_API[SCC API]
    end

    subgraph Created SCCs
        CA_SCC[Cloud Agent SCC<br/>- privileged: true<br/>- hostPID: true<br/>- SYS_ADMIN cap]
        CS_SCC[Container Sensor SCC<br/>- hostNetwork: true<br/>- socket access]
    end

    subgraph Service Accounts
        CA_SA[cloud-agent-sa]
        CS_SA[container-sensor-sa]
    end

    RECONCILE --> SCC_API
    SCC_API --> CA_SCC
    SCC_API --> CS_SCC
    CA_SCC --> CA_SA
    CS_SCC --> CS_SA
```

## Reconciliation Flow

The operator continuously reconciles the desired state with the actual state:

```mermaid
stateDiagram-v2
    [*] --> Pending: CR Created
    Pending --> Validating: Reconcile triggered
    Validating --> WaitingCredentials: PlatformConfig found
    Validating --> Error: PlatformConfig missing
    WaitingCredentials --> Progressing: Credentials ready
    WaitingCredentials --> WaitingCredentials: Credentials not ready
    Progressing --> Creating: Create resources
    Creating --> Available: All pods ready
    Creating --> Degraded: Some pods failing
    Available --> Progressing: Spec changed
    Degraded --> Progressing: Issue resolved
    Error --> Validating: Retry after interval
```

## Monitoring Deployment Status

Check the status of your deployments:

```bash
# Platform config status
kubectl get qualysplatformconfig qualys-platform -o yaml

# Cloud Agent status
kubectl get qualyscloudagent -n qualys-system -o wide

# Container Sensor status
kubectl get qualyscontainersecurity -n qualys-system -o wide

# View pods across all nodes
kubectl get pods -n qualys-system -o wide
```

Example output:

```
NAME                  DESIRED   READY   AVAILABLE   STATUS   AGE
qualys-cloud-agent    5         5       5           True     2h

NAME                      RUNTIME   DESIRED   READY   AVAILABLE   STATUS   AGE
qualys-container-sensor   cri-o     5         5       5           True     2h
```

## Data Flow to Qualys Platform

```mermaid
sequenceDiagram
    participant CA as Cloud Agent
    participant CS as Container Sensor
    participant QP as Qualys Platform
    participant UI as Qualys Console

    loop Host Scanning
        CA->>CA: Scan packages
        CA->>CA: Scan configurations
        CA->>QP: Upload VM findings
        CA->>QP: Upload compliance data
    end

    loop Container Monitoring
        CS->>CS: Detect new images
        CS->>CS: Scan image layers
        CS->>QP: Upload vulnerabilities
        CS->>CS: Monitor containers
        CS->>QP: Upload runtime events
    end

    QP->>QP: Correlate data
    QP->>UI: Update dashboards
```

## Troubleshooting

### Agent Not Starting

```mermaid
flowchart TD
    START[Pod not starting]
    CHECK_SCC{SCC created?}
    CREATE_SCC[Check operator logs<br/>for SCC errors]
    CHECK_SECRET{Secret exists?}
    CREATE_SECRET[Create credentials secret]
    CHECK_PLATFORM{PlatformConfig ready?}
    FIX_PLATFORM[Fix PlatformConfig]
    CHECK_RESOURCES{Node resources?}
    ADJUST[Adjust resource limits]
    RUNNING[Pod running]

    START --> CHECK_SCC
    CHECK_SCC -->|No| CREATE_SCC
    CHECK_SCC -->|Yes| CHECK_SECRET
    CHECK_SECRET -->|No| CREATE_SECRET
    CHECK_SECRET -->|Yes| CHECK_PLATFORM
    CHECK_PLATFORM -->|No| FIX_PLATFORM
    CHECK_PLATFORM -->|Yes| CHECK_RESOURCES
    CHECK_RESOURCES -->|Insufficient| ADJUST
    CHECK_RESOURCES -->|OK| RUNNING
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Pods pending | Missing SCC | Check operator logs for SCC creation errors |
| CrashLoopBackOff | Invalid credentials | Verify ACTIVATION_ID and CUSTOMER_ID |
| No data in Qualys | Network blocked | Check firewall rules for Qualys platform URLs |
| ImagePullBackOff | Registry access | Configure imagePullSecrets |

## Conclusion

The Qualys Nanny Operator simplifies deploying security agents across OpenShift clusters by:

- Handling the complexity of privileged container configuration
- Automatically managing OpenShift SCCs
- Supporting both immutable (CoreOS) and traditional (RHEL) node operating systems
- Providing Kubernetes-native status reporting through CR conditions

For more information, see the [Qualys Nanny GitHub repository](https://github.com/nelssec/qualys-nanny).
