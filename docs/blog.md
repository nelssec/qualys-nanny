# Deploying Qualys Security Agents on OpenShift with the Qualys Nanny Operator

Securing containerized workloads requires visibility at multiple layers: the host operating system, the container runtime, and the orchestration platform. This post walks through deploying Qualys Cloud Agent for host-level scanning on CoreOS and RHEL nodes, and the Qualys Container Security Sensor for container and image scanning on OpenShift clusters.

## Architecture Overview

The Qualys Nanny Operator manages two distinct security agents through Kubernetes-native custom resources:

```mermaid
flowchart TB
    subgraph cluster[OpenShift Cluster]
        OPERATOR[Qualys Nanny Operator]

        subgraph node1[Worker Node 1]
            CA1[Cloud Agent]
            CS1[Container Sensor]
        end

        subgraph node2[Worker Node 2]
            CA2[Cloud Agent]
            CS2[Container Sensor]
        end
    end

    QP[Qualys Platform]

    OPERATOR --> CA1
    OPERATOR --> CA2
    OPERATOR --> CS1
    OPERATOR --> CS2

    CA1 --> QP
    CA2 --> QP
    CS1 --> QP
    CS2 --> QP
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

    subgraph Resources
        DS1[DaemonSet]
        DS2[DaemonSet]
        SA[ServiceAccount]
        CM[ConfigMap]
        SCC[SCC]
    end

    PC --> SEC
    PC --> ESO
    CA --> PC
    CS --> PC
    CA --> DS1
    CA --> SA
    CA --> CM
    CA --> SCC
    CS --> DS2
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
    participant Host as Host OS
    participant QP as Qualys Platform

    User->>API: Create QualysCloudAgent CR
    API->>Op: Reconcile event
    Op->>API: Create ServiceAccount
    Op->>API: Create SCC
    Op->>API: Create ConfigMap
    Op->>API: Create DaemonSet
    DS->>Pod: Schedule on each node
    Pod->>Host: Mount host paths
    loop Scan Interval
        Pod->>Host: Scan packages
        Pod->>QP: Report findings
    end
```

### Host Mounts Required

The Cloud Agent requires extensive host access to perform vulnerability and compliance scanning:

```mermaid
flowchart LR
    AGENT[Cloud Agent Pod]

    AGENT --> ETC[/etc]
    AGENT --> VAR[/var]
    AGENT --> PROC[/proc]
    AGENT --> SYS[/sys]
    AGENT --> OPT[/opt/qualys]
```

### Installation Steps

1. **Create the namespace and credentials:**

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
      namespace: qualys
```

3. **Deploy the Cloud Agent:**

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

### CoreOS Considerations

CoreOS uses an immutable root filesystem with atomic updates. The agent handles this by:

- Storing persistent data in `/opt/qualys` (mounted from the host)
- Using the host's `/etc/machine-id` for consistent host identification
- Running as a privileged container to access host namespaces

```mermaid
flowchart TB
    subgraph coreos[CoreOS Node]
        RO[Read-Only: /usr]
        RW1[Writable: /etc]
        RW2[Writable: /var]
        RW3[Writable: /opt]
        POD[Agent Pod]
    end

    POD -->|reads| RO
    POD -->|reads| RW1
    POD -->|reads| RW2
    POD -->|writes| RW3
```

## Container Security Sensor on OpenShift

The Container Security Sensor monitors container images and running containers for vulnerabilities, malware, and secrets.

### Runtime Detection

The operator automatically detects the container runtime on OpenShift:

```mermaid
flowchart TD
    START[Start Reconciliation]
    CHECK{Runtime in CR?}
    USE_SPEC[Use specified runtime]
    OCP{Is OpenShift?}
    USE_CRIO[Use CRI-O]
    NODE[Query node info]

    START --> CHECK
    CHECK -->|Yes| USE_SPEC
    CHECK -->|No| OCP
    OCP -->|Yes| USE_CRIO
    OCP -->|No| NODE
    NODE --> DETECT[Detect from node]
```

### Sensor Architecture

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

### Installation Steps

1. **Ensure platform config exists** (from previous section)

2. **Deploy the Container Security Sensor:**

```yaml
apiVersion: qualys.qualys.io/v1alpha1
kind: QualysContainerSecurity
metadata:
  name: qualys-container-sensor
  namespace: qualys
spec:
  platformConfigRef:
    name: qualys-platform
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
    OP[Operator]

    subgraph sccs[Created SCCs]
        SCC1[Cloud Agent SCC]
        SCC2[Sensor SCC]
    end

    subgraph sas[Service Accounts]
        SA1[cloud-agent-sa]
        SA2[sensor-sa]
    end

    OP --> SCC1
    OP --> SCC2
    SCC1 --> SA1
    SCC2 --> SA2
```

**Cloud Agent SCC grants:**
- `privileged: true`
- `hostPID: true`
- `SYS_ADMIN` capability

**Container Sensor SCC grants:**
- `hostNetwork: true`
- Container runtime socket access

## Reconciliation Flow

The operator continuously reconciles the desired state with the actual state:

```mermaid
flowchart LR
    A[CR Created] --> B[Validate Config]
    B --> C{Credentials Ready?}
    C -->|No| D[Wait]
    D --> C
    C -->|Yes| E[Create Resources]
    E --> F{Pods Ready?}
    F -->|No| G[Degraded]
    F -->|Yes| H[Available]
    G --> E
    H --> I[Watch for Changes]
    I --> B
```

## Monitoring Deployment Status

Check the status of your deployments:

```bash
# Platform config status
kubectl get qualysplatformconfig qualys-platform -o yaml

# Cloud Agent status
kubectl get qualyscloudagent -n qualys -o wide

# Container Sensor status
kubectl get qualyscontainersecurity -n qualys -o wide

# View pods across all nodes
kubectl get pods -n qualys -o wide
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

    loop Host Scanning
        CA->>CA: Scan packages
        CA->>QP: Upload VM findings
    end

    loop Container Monitoring
        CS->>CS: Scan images
        CS->>QP: Upload vulnerabilities
    end

    QP->>QP: Correlate data
```

## Troubleshooting

### Agent Not Starting

```mermaid
flowchart TD
    START[Pod not starting]
    A{SCC exists?}
    B[Check operator logs]
    C{Secret exists?}
    D[Create secret]
    E{Config ready?}
    F[Fix PlatformConfig]
    G[Pod running]

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

## Conclusion

The Qualys Nanny Operator simplifies deploying security agents across OpenShift clusters by:

- Handling the complexity of privileged container configuration
- Automatically managing OpenShift SCCs
- Supporting both immutable (CoreOS) and traditional (RHEL) node operating systems
- Providing Kubernetes-native status reporting through CR conditions

For more information, see the [Qualys Nanny GitHub repository](https://github.com/nelssec/qualys-nanny).
