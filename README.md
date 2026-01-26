# Qualys Nanny Operator

Kubernetes operator for deploying and managing Qualys security components on OpenShift and Kubernetes clusters.

[![OperatorHub](https://img.shields.io/badge/OperatorHub.io-qualys--nanny-blue)](https://operatorhub.io/operator/qualys-nanny)
[![Quay.io](https://img.shields.io/badge/Quay.io-nelssec%2Fqualys--nanny-red)](https://quay.io/repository/nelssec/qualys-nanny)
[![Version](https://img.shields.io/badge/version-0.1.1-green)](https://github.com/nelssec/qualys-nanny/releases/tag/v0.1.1)

## Quick Reference

| Topic | Link |
|-------|------|
| Privilege Modes | [What works in each mode](#privilege-modes) |
| Platform Compatibility | [CRI-O and OpenShift versions](#platform-compatibility) |
| ROSA/SELinux | [Special considerations](#rosaselinux-considerations) |
| Troubleshooting | [Common issues and fixes](#troubleshooting) |
| Full Compatibility Docs | [docs/compatibility-overview.md](docs/compatibility-overview.md) |

## Components Managed

| Component | Type | Description |
|-----------|------|-------------|
| Container Sensor | DaemonSet | Scans container images and running containers for vulnerabilities |
| Cluster Sensor | Deployment | Monitors K8s API for workload inventory, network activity, compliance |
| Admission Controller | Deployment + Webhook | Enforces security policies on resource creation |
| Runtime Sensor | DaemonSet | eBPF-based file and process event tracking |
| Cloud Agent | DaemonSet | Host-level vulnerability and compliance scanning |

## Feature Compatibility Matrix

### What Works in Each Privilege Mode

| Feature | Unprivileged | Minimal | Standard | Privileged |
|---------|:------------:|:-------:|:--------:|:----------:|
| Image Scanning | - | Yes | Yes | Yes |
| Container Scanning | - | Yes | Yes | Yes |
| Static Scanning (CRI-O < 1.31) | - | Yes | Yes | Yes |
| Dynamic Scanning | - | Yes | Yes | Yes |
| SCA (Software Composition) | - | Yes | Yes | Yes |
| Malware Detection | - | - | Yes | Yes |
| Secret Detection | - | - | Yes | Yes |
| Runtime Monitoring (eBPF) | - | - | - | Yes |
| **Pod Security Standards** | restricted | baseline | baseline | privileged |
| **Runs as Root** | No | Yes | Yes | Yes |
| **Privileged Container** | No | No | No | Yes |

**Legend:** Yes = Supported, - = Not Supported

### Why Unprivileged Mode Doesn't Work

The Qualys sensor binary requires root (UID 0) to execute. Attempts to run as non-root result in:
```
exec container process '/usr/bin/tini': Permission denied
```

## Platform Compatibility

### OpenShift / CRI-O Versions

| OpenShift | CRI-O | Static Scanning | Dynamic Scanning | Recommended Mode |
|-----------|-------|:---------------:|:----------------:|------------------|
| 4.16.x | 1.29.x | Yes | Yes | minimal |
| 4.17.x | 1.30.x | Yes | Yes | minimal |
| 4.18.x | 1.31.x | **No*** | Yes | minimal + DynamicScanningOnly |

*qscanner 4.7.0 is incompatible with CRI-O 1.31's new `layers.json` format. Error: `InvalidStorageDriver:10062`

### Container Runtime Support

| Runtime | Static Scanning | Dynamic Scanning | Notes |
|---------|:---------------:|:----------------:|-------|
| CRI-O 1.30 and earlier | Yes | Yes | Full support |
| CRI-O 1.31+ | No | Yes | Use DynamicScanningOnly |
| containerd | Yes | Yes | Full support |
| Docker | Yes | Yes | Full support |

### Kubernetes Versions

| Kubernetes | Operator Support | Notes |
|------------|:----------------:|-------|
| 1.25 - 1.30 | Yes | Tested |
| 1.31+ | Yes | Should work, not yet tested |

## Privilege Modes

### Minimal Mode (Recommended)

Best balance of security and functionality. Works on ROSA, EKS, AKS, GKE.

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      enableImageScan: true
      enableContainerScan: true
    storage:
      usePersistentStorage: false
    logging:
      enableConsoleLogs: true
      logLevel: 4
```

**Security Context:**
- `privileged: false`
- `runAsUser: 0` (required by sensor)
- `allowPrivilegeEscalation: false`
- Capabilities: `SYS_PTRACE` only
- Compliant with **baseline** Pod Security Standards

### Standard Mode

Adds malware and secret detection.

```yaml
spec:
  containerSensor:
    privilegeMode: standard
    scanning:
      enableImageScan: true
      enableContainerScan: true
      enableMalwareDetection: true
      enableSecretDetection: true
```

**Additional Capabilities:** `SYS_ADMIN`, `DAC_READ_SEARCH`, `SYS_CHROOT`

### Privileged Mode

Full functionality including Runtime Sensor (eBPF).

```yaml
spec:
  containerSensor:
    privilegeMode: privileged
  runtimeSensor:
    enabled: true
```

**Security Context:** `privileged: true`

## ROSA/SELinux Considerations

Red Hat OpenShift Service on AWS (ROSA) and other SELinux-enabled clusters require special handling.

### Storage Recommendations for ROSA

| Storage Type | SELinux Compatible | Recommended |
|--------------|:------------------:|:-----------:|
| `usePersistentStorage: false` | Yes | Yes |
| `usePersistentStorage: true` | Requires SCC | No* |
| hostPath volumes | Requires SCC | No |
| emptyDir volumes | Yes | Yes |

*Persistent storage works but requires additional SCC configuration for SELinux contexts.

### Recommended ROSA Configuration

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
    privilegeMode: minimal
    scanning:
      enableImageScan: true
      enableContainerScan: true
      scanningPolicy: StaticScanningOnly  # Use DynamicScanningOnly for OCP 4.18+
    storage:
      usePersistentStorage: false
    logging:
      enableConsoleLogs: true
      logLevel: 4
  clusterSensor:
    enabled: true
    cloudProvider: AWS
    clusterName: my-rosa-cluster
```

### SELinux Cache Directory Fix (v0.1.1)

Version 0.1.1 includes a fix for SELinux cache directory permissions. The sensor now uses an emptyDir volume for the qscanner cache, which automatically gets proper SELinux labels.

**Previous error (fixed in v0.1.1):**
```
failed to create cache directory: /usr/local/qualys/qpa/data/.cache/qualys/qscanner. Error: permission denied
```

## Prerequisites

- Kubernetes 1.25+ or OpenShift 4.12+
- `kubectl` or `oc` CLI configured with cluster-admin access
- Qualys subscription with Container Security module
- `CUSTOMER_ID` and `ACTIVATION_ID` from Qualys portal (Container Security -> Sensors -> New Sensor)

## Installation

### Step 1: Install the Operator

**Option A: OperatorHub (OpenShift)**
```bash
# Via OpenShift Console: Operators -> OperatorHub -> Search "Qualys Nanny" -> Install
# Or via CLI:
oc apply -f https://operatorhub.io/install/qualys-nanny.yaml
```

**Option B: OperatorHub (Kubernetes with OLM)**
```bash
# Install OLM first if not present
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.28.0/install.sh | bash -s v0.28.0

# Install the operator
kubectl create -f https://operatorhub.io/install/qualys-nanny.yaml
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded csv -n operators -l operators.coreos.com/qualys-nanny.operators --timeout=120s
```

**Option C: Direct Manifests (no OLM)**
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
    privilegeMode: minimal
    scanning:
      enableImageScan: true
      enableContainerScan: true
    storage:
      usePersistentStorage: false
    logging:
      enableConsoleLogs: true
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

## Scanning Policies

| Policy | Description | Use When |
|--------|-------------|----------|
| `StaticScanningOnly` | Scans images from local storage | CRI-O < 1.31, containerd, Docker |
| `DynamicScanningOnly` | Pulls and scans images via runtime API | CRI-O 1.31+, or when static fails |
| `StaticWithDynamicScanningAsFallback` | Tries static first, falls back to dynamic | Mixed environments |
| `DynamicWithStaticScanningAsFallback` | Tries dynamic first, falls back to static | When dynamic preferred |

## Troubleshooting

### Common Issues

| Error | Cause | Solution |
|-------|-------|----------|
| `InvalidStorageDriver:10062` | CRI-O 1.31 incompatibility | Use `scanningPolicy: DynamicScanningOnly` |
| `Permission denied` on cache dir | SELinux on ROSA | Upgrade to v0.1.1 |
| `exec /usr/bin/tini: Permission denied` | Running as non-root | Use `minimal` mode (requires root) |
| Sensor exits immediately | Storage mismatch | Set `usePersistentStorage: false` |
| SCC permission denied | Missing SCC binding | See [OpenShift SCC](#openshift-support) section |

### Diagnostic Commands

```bash
# Check pod status and events
kubectl describe pod -n qualys -l app.kubernetes.io/component=container-sensor

# Check sensor logs
kubectl logs -n qualys -l app.kubernetes.io/component=container-sensor -f

# Check operator logs
kubectl logs -n qualys -l control-plane=controller-manager

# Verify CRI-O version (OpenShift)
oc debug node/<node-name> -- chroot /host crio --version

# Check SCC assignments (OpenShift)
oc get scc | grep qualys
oc adm policy who-can use scc qualys-container-sensor-minimal
```

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

### Self-Managed Kubernetes
```yaml
clusterSensor:
  cloudProvider: SELF_MANAGED_K8S
  clusterName: "my-cluster"
```

## OpenShift Support

On OpenShift, the operator automatically creates SecurityContextConstraints (SCCs):

| Component | SCC Name | Key Permissions |
|-----------|----------|-----------------|
| Container Sensor (minimal) | qualys-container-sensor-minimal | SYS_PTRACE, hostPath |
| Container Sensor (standard) | qualys-container-sensor-standard | SYS_ADMIN, DAC_READ_SEARCH |
| Container Sensor (privileged) | qualys-container-sensor-privileged | privileged |
| Cluster Sensor | qualys-cluster-sensor | Non-privileged (runs as user 555) |
| Runtime Sensor | qualys-runtime-sensor | Privileged (required for eBPF) |

## Sample Configurations

| File | Use Case | Privilege Mode |
|------|----------|----------------|
| `qualys_operator_containersecurity_recommended.yaml` | ROSA/SELinux compatible | minimal |
| `qualys_operator_containersecurity_minimal.yaml` | Baseline PSS compliant | minimal |
| `qualys_operator_containersecurity_standard.yaml` | Full scanning + malware | standard |
| `qualys_operator_containersecurity_privileged.yaml` | All features + Runtime | privileged |

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

## Release History

| Version | Date | Key Changes |
|---------|------|-------------|
| 0.1.1 | 2026-01-25 | Fixed SELinux cache directory permissions, added qscanner-cache volume |
| 0.1.0 | 2026-01-20 | Initial release |

## License

Apache License 2.0
