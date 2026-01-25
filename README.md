# Qualys Nanny Operator

Kubernetes operator for deploying and managing Qualys security components on OpenShift and Kubernetes clusters.

[![OperatorHub](https://img.shields.io/badge/OperatorHub.io-qualys--nanny-blue)](https://operatorhub.io/operator/qualys-nanny)
[![Quay.io](https://img.shields.io/badge/Quay.io-nelssec%2Fqualys--nanny-red)](https://quay.io/repository/nelssec/qualys-nanny)

## Components Managed

| Component | Type | Description |
|-----------|------|-------------|
| Container Sensor | DaemonSet | Scans container images and running containers for vulnerabilities |
| Cluster Sensor | Deployment | Monitors K8s API for workload inventory, network activity, compliance |
| Admission Controller | Deployment + Webhook | Enforces security policies on resource creation |
| Runtime Sensor | DaemonSet | eBPF-based file and process event tracking |
| Cloud Agent | DaemonSet | Host-level vulnerability and compliance scanning |

## Prerequisites

- Kubernetes 1.25+ or OpenShift 4.12+
- `kubectl` or `oc` CLI configured with cluster-admin access
- Qualys subscription with Container Security module
- `CUSTOMER_ID` and `ACTIVATION_ID` from Qualys portal (Container Security → Sensors → New Sensor)

## Installation

### Step 1: Install the Operator

**Option A: OperatorHub (OpenShift)**
```bash
# Via OpenShift Console: Operators → OperatorHub → Search "Qualys Nanny" → Install
# Or via CLI:
oc apply -f https://operatorhub.io/install/qualys-nanny.yaml
```

**Option A: OperatorHub (Kubernetes with OLM)**
```bash
# Install OLM first if not present
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.28.0/install.sh | bash -s v0.28.0

# Install the operator
kubectl create -f https://operatorhub.io/install/qualys-nanny.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded csv -n operators -l operators.coreos.com/qualys-nanny.operators --timeout=120s
```

**Option B: Direct Manifests (no OLM)**
```bash
kubectl apply -f https://raw.githubusercontent.com/nelssec/qualys-nanny/main/dist/install.yaml
```

### Step 2: Create Credentials Secret

```bash
kubectl create namespace qualys --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic qualys-credentials \
  --namespace qualys \
  --from-literal=CUSTOMER_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --from-literal=ACTIVATION_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

### Step 3: Create Platform Configuration

Get your regional URLs from https://www.qualys.com/platform-identification

```bash
cat <<EOF | kubectl apply -f -
apiVersion: qualys.io/v1
kind: QualysPlatformConfig
metadata:
  name: qualys-platform
spec:
  platform:
    serverUri: "https://cmsqagpublic.qg1.apps.qualys.com/ContainerSensor"  # US1 - change for your region
    gatewayUrl: "https://gateway.qg1.apps.qualys.com"                       # US1 - change for your region
  credentials:
    sourceType: secret
    secretRef:
      name: qualys-credentials
      namespace: qualys
EOF
```

### Step 4: Deploy Sensors

```bash
cat <<EOF | kubectl apply -f -
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
    privilegeMode: standard    # unprivileged | minimal | standard | privileged
    scanning:
      enableImageScan: true
      enableContainerScan: true
      enableScaScan: true
  clusterSensor:
    enabled: true
    cloudProvider: AWS         # AWS | AZURE | GCP | OCI | SELF_MANAGED_K8S
    clusterName: my-cluster
EOF
```

### Step 5: Verify Deployment

```bash
# Check operator
kubectl get pods -n qualys -l control-plane=controller-manager

# Check sensors
kubectl get qualyscontainersecurity -n qualys
kubectl get pods -n qualys
kubectl get daemonset,deployment -n qualys

# Check sensor logs
kubectl logs -n qualys -l app.kubernetes.io/component=container-sensor --tail=50
```

## Privilege Modes

The Container Sensor supports four privilege modes to balance security requirements with scanning capabilities:

| Mode | Runs As | Capabilities | Features |
|------|---------|--------------|----------|
| `unprivileged` | UID 65534 | None | ❌ Not supported (sensor requires root) |
| `minimal` | Root | SYS_PTRACE | Image + container scanning |
| `standard` | Root | SYS_ADMIN, SYS_PTRACE, SYS_CHROOT, DAC_READ_SEARCH | All features + malware detection |
| `privileged` | Root | Full privileged | All features + Runtime Sensor |

**Note:** The Container Sensor does NOT require `privileged: true`. Use `minimal` mode for basic scanning or `standard` mode for full functionality including malware detection. Only the Runtime Sensor (eBPF) requires privileged mode.

### Minimum Privilege Configuration (Recommended)

For environments requiring minimum privileges, use `minimal` mode:

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: StaticScanningOnly  # or DynamicScanningOnly for CRI-O 1.31+
```

This runs with:
- `privileged: false`
- `runAsUser: 0` (root required by sensor binary)
- Only `SYS_PTRACE` capability
- Read-only access to container storage

## CRI-O Compatibility

> ⚠️ **Important**: Static scanning has known compatibility issues with newer CRI-O versions.

| CRI-O Version | OpenShift | Static Scanning | Dynamic Scanning |
|---------------|-----------|-----------------|------------------|
| 1.30.x | 4.17.x | ✅ Works | ✅ Works |
| 1.31.x | 4.18.x | ❌ Fails* | ✅ Works |

*qscanner 4.7.0 is incompatible with CRI-O 1.31's new `layers.json` format. Error: `InvalidStorageDriver:10062`

**Workaround for CRI-O 1.31+**: Use `scanningPolicy: DynamicScanningOnly`

For detailed compatibility information, see [docs/compatibility-overview.md](docs/compatibility-overview.md).

## Sample Configurations

| File | Use Case |
|------|----------|
| `qualys_operator_containersecurity_recommended.yaml` | Recommended default (standard mode) |
| `qualys_operator_containersecurity_standard.yaml` | Full configuration with all options |
| `qualys_operator_containersecurity_minimal.yaml` | Baseline PSS compliant |
| `qualys_operator_containersecurity_unprivileged.yaml` | Restricted PSS compliant |
| `qualys_operator_containersecurity_privileged.yaml` | All features + Runtime Sensor |

## Operator Permissions

The operator requires the following RBAC permissions:

### Core Resources
- ServiceAccounts, ConfigMaps, Secrets, Services: full CRUD
- Nodes, Pods, Namespaces: get, list, watch
- Events: create, patch

### Workload Resources
- DaemonSets, Deployments: full CRUD
- Jobs, CronJobs: get, list, watch, create, delete

### RBAC Resources
- ClusterRoles, ClusterRoleBindings, Roles, RoleBindings: full CRUD

### Admission Resources
- ValidatingWebhookConfigurations: full CRUD

### OpenShift Resources
- SecurityContextConstraints: full CRUD (OpenShift only)

### Cluster-wide Read Access
- All resources: get, list (required for Cluster Sensor)

## Platform URLs

Find your regional Qualys platform URLs at: https://www.qualys.com/platform-identification

| Region | Container Sensor URL | Gateway URL |
|--------|---------------------|-------------|
| US1 | https://cmsqagpublic.qg1.apps.qualys.com/ContainerSensor | https://gateway.qg1.apps.qualys.com |
| US2 | https://cmsqagpublic.qg2.apps.qualys.com/ContainerSensor | https://gateway.qg2.apps.qualys.com |
| US3 | https://cmsqagpublic.qg3.apps.qualys.com/ContainerSensor | https://gateway.qg3.apps.qualys.com |
| EU1 | https://cmsqagpublic.qg1.apps.qualys.eu/ContainerSensor | https://gateway.qg1.apps.qualys.eu |
| CA1 | https://cmsqagpublic.qg1.apps.qualys.ca/ContainerSensor | https://gateway.qg1.apps.qualys.ca |

## Cloud Provider Configuration

For the Cluster Sensor, specify your cloud provider and cluster identifier:

### AWS
```yaml
clusterSensor:
  cloudProvider: AWS
  clusterID: "arn:aws:eks:us-east-1:123456789:cluster/my-cluster"
```

### Azure
```yaml
clusterSensor:
  cloudProvider: AZURE
  clusterID: "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.ContainerService/managedClusters/xxx"
  clusterRegion: "eastus"
```

### GCP
```yaml
clusterSensor:
  cloudProvider: GCP
  clusterID: "projects/my-project/locations/us-central1/clusters/my-cluster"
```

### Self-Managed Kubernetes
```yaml
clusterSensor:
  cloudProvider: SELF_MANAGED_K8S
  clusterName: "my-cluster"
```

## OpenShift Support

On OpenShift, the operator automatically creates SecurityContextConstraints (SCCs) for each component:

| Component | SCC Permissions |
|-----------|----------------|
| Container Sensor | hostNetwork, hostPID, runtime socket access |
| Cluster Sensor | Non-privileged (runs as user 555) |
| Runtime Sensor | Privileged (required for eBPF) |
| Cloud Agent | Privileged, hostPID, SYS_ADMIN |

## Verification

```bash
kubectl get qualysplatformconfig
kubectl get qualyscontainersecurity -n qualys
kubectl get pods -n qualys
kubectl get daemonset -n qualys
kubectl get deployment -n qualys
```

## Building from Source

```bash
make build
make docker-build IMG=<your-registry>/qualys-nanny:latest
make docker-push IMG=<your-registry>/qualys-nanny:latest
make build-installer IMG=<your-registry>/qualys-nanny:latest
```

## Documentation

- [Compatibility Overview](docs/compatibility-overview.md) - Detailed privilege modes, CRI-O compatibility, and troubleshooting
- [CRI-O 1.31 Analysis](docs/qscanner-crio-131-compatibility.md) - Technical analysis of qscanner compatibility issues

## License

Apache License 2.0
