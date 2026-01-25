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

### Option A: Install from OperatorHub (Recommended)

The Qualys Nanny Operator is available on [OperatorHub.io](https://operatorhub.io/operator/qualys-nanny).

**OpenShift Console:**
1. Navigate to **Operators → OperatorHub**
2. Search for "**Qualys Nanny**"
3. Click **Install**
4. Select the target namespace and approval strategy
5. Click **Install** to deploy the operator

**Kubernetes with OLM:**

First, install OLM if not already present:
```bash
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.28.0/install.sh | bash -s v0.28.0
```

Then install the operator:
```bash
kubectl create -f https://operatorhub.io/install/qualys-nanny.yaml
kubectl get csv -n operators
```

Wait for the operator to be ready:
```bash
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded csv -n operators -l operators.coreos.com/qualys-nanny.operators
```

### Option B: Install from Manifests

For environments without OLM, install directly from manifests:

```bash
kubectl apply -f https://raw.githubusercontent.com/nelssec/qualys-nanny/main/dist/install.yaml
```

Or clone the repository and apply locally:
```bash
git clone https://github.com/nelssec/qualys-nanny.git
cd qualys-nanny
kubectl apply -f dist/install.yaml
```

This creates:
- `qualys` namespace
- CRDs for QualysPlatformConfig and QualysContainerSecurity
- Operator deployment with required RBAC

### Step 1: Create Credentials

```bash
kubectl create secret generic qualys-credentials \
  --namespace qualys \
  --from-literal=CUSTOMER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --from-literal=ACTIVATION_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

### Step 2: Configure Platform URLs

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

### Step 3: Deploy Container Security

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

## Scanning Policies

The Container Sensor supports three scanning policies to control how vulnerabilities are detected:

### DynamicWithStaticScanningAsFallback (Default)
```yaml
containerSensor:
  scanning:
    scanningPolicy: DynamicWithStaticScanningAsFallback
```
- Attempts dynamic scanning first (running containers)
- Falls back to static image scanning if dynamic fails
- Best balance of accuracy and coverage

### DynamicScanningOnly
```yaml
containerSensor:
  scanning:
    scanningPolicy: DynamicScanningOnly
```
- Only scans running containers
- Requires container to be running for vulnerability detection
- Most accurate for runtime state

### StaticScanningOnly
```yaml
containerSensor:
  scanning:
    scanningPolicy: StaticScanningOnly
```
- Scans images directly from container storage
- Works in unprivileged/standard modes on CRI-O
- Does not require containers to be running
- Best for CI/CD and air-gapped environments

**Note:** Static scanning on CRI-O uses the `--storage-driver crio-overlay` option which allows direct access to image layers without requiring privileged user namespaces.

## RBAC Permissions

### Operator Manager Permissions

The operator manager requires permissions to create and manage the sensor components:

| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `""` (core) | configmaps, secrets, serviceaccounts, services | create, delete, get, list, patch, update, watch | Manage sensor configurations and credentials |
| `""` (core) | namespaces, nodes, pods, endpoints | get, list, watch | Monitor cluster state |
| `""` (core) | events | create, patch | Emit Kubernetes events |
| `apps` | daemonsets, deployments, replicasets, statefulsets | create, delete, get, list, patch, update, watch | Deploy sensor workloads |
| `batch` | jobs, cronjobs | create, delete, get, list, watch | Host scanner jobs |
| `rbac.authorization.k8s.io` | clusterroles, clusterrolebindings, roles, rolebindings | create, delete, get, list, patch, update, watch | Manage sensor RBAC |
| `admissionregistration.k8s.io` | validatingwebhookconfigurations | create, delete, get, list, patch, update, watch | Admission controller webhooks |
| `qualys.io` | qualyscontainersecurities, qualysplatformconfigs | create, delete, get, list, patch, update, watch | Manage custom resources |
| `*` | `*` | get, list | Cluster-wide read access for inventory |

### Container Sensor Permissions

| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `""` (core) | nodes, nodes/status | get, list, watch | Node inventory |
| `""` (core) | pods, pods/status | get, list, watch | Pod/container inventory |
| `""` (core) | pods/exec | create | Container scanning |
| `""` (core) | namespaces, services, configmaps, secrets | get, list, watch | Workload context |
| `""` (core) | replicationcontrollers/status | get, list, watch | Workload status |
| `apps` | deployments, replicasets, daemonsets, statefulsets (+ /status) | get, list, watch | Workload inventory |
| `batch` | jobs, cronjobs (+ /status) | get, list, watch | Job inventory |
| `batch` | jobs | create, delete | Image scanning jobs |

**Host Access:**
| Path | Access | Purpose |
|------|--------|---------|
| `/var/run/crio/crio.sock` or equivalent | Read | CRI API for image/container inspection |
| `/var/lib/containers` | Read | Container layer access for scanning |
| `/etc/os-release` | Read | Host OS detection |

### Cluster Sensor Permissions

| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `""` (core) | pods, namespaces, nodes, services, serviceaccounts | watch | Real-time workload tracking |
| `rbac.authorization.k8s.io` | roles, rolebindings, clusterroles, clusterrolebindings | watch | RBAC visibility |
| `discovery.k8s.io` | endpointslices | watch | Service discovery |
| `networking.k8s.io` | ingresses | watch | Ingress inventory |
| `*` | `*` | get, list | Full cluster inventory |

**OpenShift Additional:**
| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `security.openshift.io` | securitycontextconstraints | create, get, list, watch, update, patch, delete | SCC management |

### Admission Controller Permissions

| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `""` (core) | pods, namespaces | get, list, watch | Pod admission decisions |
| `apps` | deployments, daemonsets, replicasets, statefulsets | get, list, watch | Workload context |
| `batch` | jobs, cronjobs | get, list, watch | Job context |

### Runtime Sensor Permissions

| API Group | Resources | Verbs | Purpose |
|-----------|-----------|-------|---------|
| `""` (core) | nodes, pods | get, list, watch | Container/host correlation |

**Host Access:**
| Path | Access | Purpose |
|------|--------|---------|
| `/sys/kernel/debug` | Read | eBPF program loading |
| `/proc` | Read | Process inspection |
| Host network namespace | Full | Network visibility |
| Host PID namespace | Full | Process visibility |

**Note:** Runtime Sensor requires `privileged: true` for eBPF kernel access.

## OpenShift Security Context Constraints

On OpenShift, the operator creates SCCs automatically based on the privilege mode:

### Container Sensor SCC (`{name}-container-scc`)

| Setting | Unprivileged | Minimal | Standard | Privileged |
|---------|--------------|---------|----------|------------|
| `allowPrivilegedContainer` | false | false | false | false |
| `allowHostNetwork` | false | true | true | true |
| `allowHostPID` | false | true | true | true |
| `allowHostPorts` | false | true | true | true |
| `runAsUser` | MustRunAs (65534) | RunAsAny | RunAsAny | RunAsAny |
| `seLinuxContext` | MustRunAs | RunAsAny | RunAsAny | RunAsAny |
| `fsGroup` | MustRunAs | RunAsAny | RunAsAny | RunAsAny |
| `allowedCapabilities` | [] | [SYS_PTRACE] | [SYS_ADMIN, SYS_PTRACE, SYS_CHROOT, DAC_READ_SEARCH] | [*] |
| `volumes` | configMap, emptyDir, secret, persistentVolumeClaim | + hostPath | + hostPath | + hostPath |

### Cluster Sensor SCC (`{name}-cluster-scc`)

| Setting | Value |
|---------|-------|
| `allowPrivilegedContainer` | false |
| `allowHostNetwork` | true |
| `allowHostPID` | false |
| `runAsUser` | MustRunAs (555) |
| `seLinuxContext` | MustRunAs |
| `volumes` | configMap, emptyDir, secret |

### Runtime Sensor SCC (`{name}-runtime-scc`)

| Setting | Value |
|---------|-------|
| `allowPrivilegedContainer` | true |
| `allowHostNetwork` | true |
| `allowHostPID` | true |
| `runAsUser` | RunAsAny |
| `seLinuxContext` | RunAsAny |
| `allowedCapabilities` | [*] |
| `volumes` | configMap, emptyDir, secret, hostPath |

**Note:** Runtime Sensor requires privileged mode for eBPF kernel access. This is the only component that truly requires `privileged: true`.

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

## Updating the Operator

### OperatorHub Updates

If installed via OperatorHub, updates are managed automatically based on your approval strategy:

- **Automatic:** Updates install automatically when new versions are released
- **Manual:** You'll be notified of updates and must approve them

To check for updates in OpenShift:
1. Navigate to **Operators → Installed Operators**
2. Look for the **Upgrade available** indicator
3. Click to review and approve the update

### Manual Updates

For manifest-based installations:
```bash
kubectl apply -f https://raw.githubusercontent.com/nelssec/qualys-nanny/main/dist/install.yaml
```

The operator will automatically update the managed sensors when the Custom Resources are reconciled.

## Resources

- **GitHub Repository:** https://github.com/nelssec/qualys-nanny
- **OperatorHub:** https://operatorhub.io/operator/qualys-nanny
- **Container Images:** https://quay.io/repository/nelssec/qualys-nanny
- **Qualys Platform URLs:** https://www.qualys.com/platform-identification
- **Qualys Container Security Docs:** https://www.qualys.com/docs/qualys-container-security-user-guide.pdf

## License

Apache License 2.0
