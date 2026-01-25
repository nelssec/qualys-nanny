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

## Quick Start

### Option A: Install from OperatorHub (Recommended)

**OpenShift:**
1. Navigate to **Operators → OperatorHub** in the OpenShift Console
2. Search for "**Qualys Nanny**"
3. Click **Install** and follow the prompts

**Kubernetes with OLM:**
```bash
kubectl create -f https://operatorhub.io/install/qualys-nanny.yaml
kubectl get csv -n operators
```

### Option B: Install from Manifests

```bash
kubectl apply -f https://raw.githubusercontent.com/nelssec/qualys-nanny/main/dist/install.yaml
```

Or clone and apply locally:
```bash
kubectl apply -f dist/install.yaml
```

### 1. Create Credentials Secret

```bash
kubectl create secret generic qualys-credentials \
  --namespace qualys \
  --from-literal=CUSTOMER_ID=<your-customer-id> \
  --from-literal=ACTIVATION_ID=<your-activation-id>
```

### 2. Create Platform Configuration

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

### 3. Deploy Container Security

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
```

## Privilege Modes

The Container Sensor supports four privilege modes to balance security requirements with scanning capabilities:

| Mode | Runs As | Capabilities | Features |
|------|---------|--------------|----------|
| `unprivileged` | UID 65534 | None | Image scanning only |
| `minimal` | Root | SYS_PTRACE | Image + container scanning |
| `standard` | Root | SYS_ADMIN, SYS_PTRACE, SYS_CHROOT, DAC_READ_SEARCH | All features (recommended) |
| `privileged` | Root | Full privileged | All features + Runtime Sensor |

**Note:** The Container Sensor does NOT require `privileged: true`. Use `standard` mode for full functionality. Only the Runtime Sensor (eBPF) requires privileged mode.

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

## License

Apache License 2.0
