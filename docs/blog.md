# Qualys Container Security on Kubernetes with the Qualys Nanny Operator

This guide covers deploying Qualys Container Security components on Kubernetes and OpenShift clusters using the Qualys Nanny Operator.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                   Qualys Namespace                        │  │
│  │  ┌─────────────────┐    ┌─────────────────────────────┐   │  │
│  │  │ Platform Config │───▶│ Credentials Secret          │   │  │
│  │  └────────┬────────┘    └─────────────────────────────┘   │  │
│  │           │                                               │  │
│  │           ▼                                               │  │
│  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  │           QualysContainerSecurity CR                │  │  │
│  │  └─────────────────────────────────────────────────────┘  │  │
│  │           │                                               │  │
│  │     ┌─────┴─────┬─────────────┬─────────────┐            │  │
│  │     ▼           ▼             ▼             ▼            │  │
│  │  Container   Cluster     Admission      Runtime          │  │
│  │  Sensor      Sensor      Controller     Sensor           │  │
│  │  (DaemonSet) (Deploy)    (Deploy)       (DaemonSet)      │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐             │
│  │  Worker 1   │  │  Worker 2   │  │  Worker N   │             │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │             │
│  │ │Container│ │  │ │Container│ │  │ │Container│ │             │
│  │ │ Sensor  │ │  │ │ Sensor  │ │  │ │ Sensor  │ │             │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │             │
│  └─────────────┘  └─────────────┘  └─────────────┘             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │ Qualys Platform │
                    └─────────────────┘
```

## Components

### Container Sensor (DaemonSet)
Scans container images and running containers for vulnerabilities.

**Features:**
- Image vulnerability scanning via CRI API
- Running container scanning
- SCA (Software Composition Analysis)

### Cluster Sensor (Deployment)
Monitors the Kubernetes API for cluster-wide visibility.

**Features:**
- Workload inventory
- K8s compliance scanning
- Network policy analysis
- RBAC visibility
- Host scanning (via Jobs)

### Admission Controller (Deployment + Webhook)
Enforces security policies on resource creation and updates.

**Features:**
- Block vulnerable images
- Audit or block mode
- Namespace-scoped enforcement
- Configurable failure policy

### Runtime Sensor (DaemonSet)
Uses eBPF for kernel-level visibility into container behavior.

**Features:**
- Real-time file integrity monitoring
- Process execution tracking
- Behavioral anomaly detection

**Note:** Runtime Sensor requires `privileged: true` for eBPF kernel access. This is the only component that requires privileged mode.

## Installation

### Prerequisites
- Kubernetes 1.25+ or OpenShift 4.12+
- kubectl or oc CLI
- Qualys subscription with Container Security module
- CUSTOMER_ID and ACTIVATION_ID from Qualys portal

### Step 1: Install the Operator

```bash
kubectl apply -f dist/install.yaml
```

This creates:
- `qualys` namespace
- CRDs for QualysPlatformConfig and QualysContainerSecurity
- Operator deployment with required RBAC

### Step 2: Create Credentials

```bash
kubectl create secret generic qualys-credentials \
  --namespace qualys \
  --from-literal=CUSTOMER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --from-literal=ACTIVATION_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

### Step 3: Configure Platform URLs

Create a QualysPlatformConfig with your regional URLs:

```yaml
apiVersion: qualys.io/v1
kind: QualysPlatformConfig
metadata:
  name: qualys-platform
spec:
  platform:
    serverUri: "https://cmsqagpublic.qg1.apps.qualys.ca/ContainerSensor"
    gatewayUrl: "https://gateway.qg1.apps.qualys.ca"
  credentials:
    sourceType: secret
    secretRef:
      name: qualys-credentials
      namespace: qualys
```

Find your regional URLs: https://www.qualys.com/platform-identification

### Step 4: Deploy Container Security

```yaml
apiVersion: qualys.io/v1
kind: QualysContainerSecurity
metadata:
  name: qualys-container-security
  namespace: qualys
spec:
  platformConfigRef:
    name: qualys-platform
  containerSensor:
    enabled: true
    privilegeMode: standard
    scanning:
      enableImageScan: true
      enableContainerScan: true
      enableScaScan: true
  clusterSensor:
    enabled: true
    cloudProvider: AWS
    clusterName: my-cluster
    k8sCompliance: true
    hostScanner:
      enabled: true
  admissionController:
    enabled: false
  runtimeSensor:
    enabled: false
```

## Privilege Modes

The Container Sensor supports four privilege modes:

### unprivileged
- Runs as: UID 65534 (nobody)
- Capabilities: None (all dropped)
- Features: Image scanning only
- PSS Profile: Restricted
- Use when: Strict security requirements, image scanning sufficient

### minimal
- Runs as: Root (UID 0)
- Capabilities: SYS_PTRACE
- Features: Image + container scanning
- PSS Profile: Baseline
- Use when: Need container scanning without full privileges

### standard (Recommended)
- Runs as: Root (UID 0)
- Capabilities: SYS_ADMIN, SYS_PTRACE, SYS_CHROOT, DAC_READ_SEARCH
- Features: All scanning features
- PSS Profile: Privileged namespace required
- Use when: Full functionality needed

### privileged
- Runs as: Root (UID 0)
- Capabilities: Full privileged mode
- Features: All scanning + Runtime Sensor
- PSS Profile: Privileged
- Use when: Runtime Sensor (eBPF) required

**Key Point:** The Container Sensor does NOT require `privileged: true`. Standard mode provides full scanning capabilities with specific Linux capabilities instead of full privileges.

## RBAC Permissions

### Operator Permissions

The operator requires these permissions to manage resources:

```yaml
rules:
- apiGroups: [""]
  resources: [serviceaccounts, configmaps, secrets, services, namespaces, nodes, pods, endpoints]
  verbs: [get, list, watch, create, update, patch, delete]
- apiGroups: [apps]
  resources: [daemonsets, deployments]
  verbs: [get, list, watch, create, update, patch, delete]
- apiGroups: [rbac.authorization.k8s.io]
  resources: [clusterroles, clusterrolebindings, roles, rolebindings]
  verbs: [get, list, watch, create, update, patch, delete]
- apiGroups: [admissionregistration.k8s.io]
  resources: [validatingwebhookconfigurations]
  verbs: [get, list, watch, create, update, patch, delete]
- apiGroups: ["*"]
  resources: ["*"]
  verbs: [get, list]
```

### Component Permissions

**Container Sensor:**
- Runtime socket access (read-only)
- Host filesystem access (for scanning)

**Cluster Sensor:**
- Cluster-wide read access to all resources
- Watch on pods, namespaces, nodes, services

**Admission Controller:**
- Read access to pods, deployments, daemonsets
- ValidatingWebhookConfiguration management

## OpenShift Security Context Constraints

On OpenShift, the operator creates SCCs automatically:

| Component | SCC Name | Privileges |
|-----------|----------|------------|
| Container Sensor | `{name}-container-scc` | hostNetwork, hostPID, capabilities |
| Cluster Sensor | `{name}-cluster-scc` | Non-privileged (user 555) |
| Runtime Sensor | `{name}-runtime-scc` | Privileged (eBPF required) |

## Cloud Provider Configuration

### AWS (EKS/ROSA)
```yaml
clusterSensor:
  cloudProvider: AWS
  clusterID: "arn:aws:eks:us-east-1:123456789:cluster/my-cluster"
```

### Azure (AKS)
```yaml
clusterSensor:
  cloudProvider: AZURE
  clusterID: "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.ContainerService/managedClusters/xxx"
  clusterRegion: "eastus"
```

### GCP (GKE)
```yaml
clusterSensor:
  cloudProvider: GCP
  clusterID: "projects/my-project/locations/us-central1/clusters/my-cluster"
```

### OCI (OKE)
```yaml
clusterSensor:
  cloudProvider: OCI
  clusterID: "ocid1.cluster.oc1..."
  clusterName: "my-cluster"
```

### Self-Managed
```yaml
clusterSensor:
  cloudProvider: SELF_MANAGED_K8S
  clusterName: "my-cluster"
```

## Verification

### Check Operator Status
```bash
kubectl get pods -n qualys -l control-plane=controller-manager
```

### Check Platform Config
```bash
kubectl get qualysplatformconfig qualys-platform -o yaml
```

### Check Container Security Status
```bash
kubectl get qualyscontainersecurity -n qualys
```

Expected output:
```
NAME                       CONTAINER   CLUSTER   ADMISSION   RUNTIME   AGE
qualys-container-security  true        true      false       false     5m
```

### Check Component Pods
```bash
kubectl get pods -n qualys
```

### Check DaemonSet Status
```bash
kubectl get daemonset -n qualys
```

## Troubleshooting

### Pods Not Starting

1. Check operator logs:
```bash
kubectl logs -n qualys -l control-plane=controller-manager
```

2. Check SCC (OpenShift):
```bash
oc get scc | grep qualys
```

3. Verify credentials secret exists:
```bash
kubectl get secret qualys-credentials -n qualys
```

### Sensors Not Connecting to Qualys Platform

1. Verify platform URLs are correct for your region
2. Check sensor pod logs:
```bash
kubectl logs -n qualys -l app.kubernetes.io/component=container-sensor
```

3. Verify network connectivity to Qualys platform

### Admission Controller Issues

1. Check webhook configuration:
```bash
kubectl get validatingwebhookconfigurations | grep qualys
```

2. Check admission controller logs:
```bash
kubectl logs -n qualys -l app.kubernetes.io/component=admission-controller
```

## Sample Configurations

The operator includes sample configurations in `config/samples/`:

| File | Description |
|------|-------------|
| `qualys_operator_platformconfig.yaml` | Platform configuration template |
| `qualys_operator_containersecurity_recommended.yaml` | Recommended setup (standard mode) |
| `qualys_operator_containersecurity_standard.yaml` | Full configuration with all options |
| `qualys_operator_containersecurity_minimal.yaml` | Minimal privileges |
| `qualys_operator_containersecurity_unprivileged.yaml` | Unprivileged mode |
| `qualys_operator_containersecurity_privileged.yaml` | Full features + Runtime Sensor |

## License

Apache License 2.0
