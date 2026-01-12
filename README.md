# Qualys Nanny Operator

A Kubernetes operator for deploying and managing Qualys security agents on OpenShift and Kubernetes clusters.

## Overview

The Qualys Nanny Operator simplifies deployment of Qualys security agents by:

- Managing Qualys Cloud Agent for host-level vulnerability and compliance scanning
- Managing Qualys Container Security components (Container Sensor, Cluster Sensor, Admission Controller, Runtime Sensor)
- Automatically detecting node OS (CoreOS, RHEL, Debian) and selecting appropriate deployment mode
- Automatically creating required SecurityContextConstraints on OpenShift
- Supporting both native Kubernetes Secrets and External Secrets Operator for credential management
- Auto-detecting container runtime (containerd, CRI-O, Docker)

## Components

### QualysCloudAgent

Host-level security agent for vulnerability management and compliance scanning.

| Mode | OS Types | Image | Description |
|------|----------|-------|-------------|
| `bootstrapper` | RHEL, CentOS, Debian, Ubuntu | `nelssec/qualys-agent-bootstrapper` | Installs agent on host via nsenter |
| `coreos` | CoreOS, RHCOS, Flatcar | `qualys/qagent-rhcos` | Runs agent in container (immutable OS) |

Set `deploymentMode: auto` (default) to let the operator detect the appropriate mode.

### QualysContainerSecurity

Container security suite with four deployable components:

| Component | Type | Default | Description |
|-----------|------|---------|-------------|
| Container Sensor | DaemonSet | Enabled | Scans container images and running containers for vulnerabilities |
| Cluster Sensor | Deployment | Enabled | Monitors K8s API for cluster events, network activity, and workload inventory |
| Admission Controller | Deployment + Webhook | Disabled | Enforces security policies on resource creation/updates |
| Runtime Sensor | DaemonSet | Disabled | Uses eBPF to track file and process events in containers |

## Architecture

The operator manages three Custom Resources:

| CRD | Scope | Purpose |
|-----|-------|---------|
| `QualysPlatformConfig` | Cluster | Shared Qualys platform settings and credentials |
| `QualysCloudAgent` | Namespace | Cloud Agent DaemonSet for host scanning |
| `QualysContainerSecurity` | Namespace | Container security components |

## Prerequisites

- OpenShift 4.12+ or Kubernetes 1.25+
- Qualys subscription with Cloud Agent and/or Container Security
- Qualys activation ID and customer ID
- `kubectl` or `oc` CLI configured

## Installation

### Option 1: Deploy from Source

```bash
git clone https://github.com/nelssec/qualys-nanny.git
cd qualys-nanny

make docker-build docker-push IMG=<your-registry>/qualys-nanny:v0.1.0
make install
make deploy IMG=<your-registry>/qualys-nanny:v0.1.0
```

### Option 2: Deploy via OLM (OperatorHub)

```bash
make bundle-build bundle-push BUNDLE_IMG=<your-registry>/qualys-nanny-bundle:v0.1.0
operator-sdk run bundle <your-registry>/qualys-nanny-bundle:v0.1.0
```

## Quick Start

### 1. Create the Namespace

```bash
kubectl create namespace qualys
```

### 2. Create Credentials Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: qualys-credentials
  namespace: qualys
type: Opaque
stringData:
  ACTIVATION_ID: "your-activation-id"
  CUSTOMER_ID: "your-customer-id"
```

### 3. Create Platform Configuration

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

### 4. Deploy Cloud Agent (Host Scanning)

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

### 5. Deploy Container Security

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
    image:
      repository: qualys/qcs-sensor
      tag: "latest"
    mode: general
    k8sMode: true
    scanning:
      enableImageScan: true
      enableContainerScan: true
      scanThreadPoolSize: 2
    storage:
      usePersistentStorage: true
      storageSize: "10Gi"
  clusterSensor:
    enabled: true
    replicas: 1
  admissionController:
    enabled: false
    replicas: 2
    failurePolicy: Ignore
  runtimeSensor:
    enabled: false
  containerRuntime:
    type: auto
  scheduling:
    nodeSelector:
      kubernetes.io/os: linux
    tolerations:
      - operator: Exists
        effect: NoSchedule
      - operator: Exists
        effect: NoExecute
    priorityClassName: system-node-critical
```

## Configuration Reference

### QualysPlatformConfig

| Field | Description | Required |
|-------|-------------|----------|
| `spec.platform.serverUri` | Qualys platform URL | Yes |
| `spec.platform.proxy` | Proxy configuration | No |
| `spec.credentials.sourceType` | `secret` or `externalSecret` | Yes |
| `spec.credentials.secretRef` | Reference to K8s Secret | When sourceType=secret |
| `spec.credentials.externalSecretRef` | Reference to ExternalSecret | When sourceType=externalSecret |

### QualysCloudAgent

| Field | Description | Default |
|-------|-------------|---------|
| `spec.platformConfigRef.name` | Name of QualysPlatformConfig | Required |
| `spec.deploymentMode` | `auto`, `bootstrapper`, or `coreos` | `auto` |
| `spec.image.repository` | Container image | Auto-selected based on mode |
| `spec.image.tag` | Image tag | `v2.1.0` (bootstrapper) / `latest` (coreos) |
| `spec.config.logLevel` | Log verbosity (0-5) | `3` |
| `spec.config.cmdMaxTimeOut` | Command timeout (seconds) | `1800` |
| `spec.scheduling.tolerations` | Pod tolerations | All taints |
| `spec.scheduling.priorityClassName` | Priority class | `system-node-critical` |
| `spec.resources` | CPU/memory limits | None |
| `spec.coreosConfig.cpuLimit` | CPU limit for CoreOS mode | `200m` |
| `spec.coreosConfig.providerName` | Cloud provider (AWS, AZURE, GCP, etc.) | `AUTO` |

### QualysContainerSecurity

#### Container Sensor (DaemonSet)

| Field | Description | Default |
|-------|-------------|---------|
| `spec.containerSensor.enabled` | Deploy the Container Sensor | `true` |
| `spec.containerSensor.image.repository` | Container image | `qualys/qcs-sensor` |
| `spec.containerSensor.mode` | `general`, `registry`, or `cicd` | `general` |
| `spec.containerSensor.k8sMode` | Enable Kubernetes integration | `true` |
| `spec.containerSensor.scanning.enableImageScan` | Enable image scanning | `true` |
| `spec.containerSensor.scanning.enableContainerScan` | Enable container scanning | `true` |
| `spec.containerSensor.scanning.enableMalwareDetection` | Enable malware detection | `false` |
| `spec.containerSensor.scanning.enableSecretDetection` | Enable secret detection | `false` |
| `spec.containerSensor.scanning.scanThreadPoolSize` | Concurrent scan threads | `2` |
| `spec.containerSensor.storage.usePersistentStorage` | Use persistent storage | `true` |
| `spec.containerSensor.storage.storageSize` | Persistent volume size | `10Gi` |
| `spec.containerSensor.logging.logLevel` | Log verbosity (0-5) | `3` |

#### Cluster Sensor (Deployment)

| Field | Description | Default |
|-------|-------------|---------|
| `spec.clusterSensor.enabled` | Deploy the Cluster Sensor | `true` |
| `spec.clusterSensor.image.repository` | Container image | `qualys/cluster-sensor` |
| `spec.clusterSensor.replicas` | Number of replicas | `1` |
| `spec.clusterSensor.logging.logLevel` | Log verbosity (0-5) | `3` |

#### Admission Controller (Deployment + Webhook)

| Field | Description | Default |
|-------|-------------|---------|
| `spec.admissionController.enabled` | Deploy the Admission Controller | `false` |
| `spec.admissionController.image.repository` | Container image | `qualys/admission-controller` |
| `spec.admissionController.replicas` | Number of replicas | `2` |
| `spec.admissionController.failurePolicy` | Webhook failure policy (`Fail` or `Ignore`) | `Ignore` |
| `spec.admissionController.namespaceSelector` | Limit namespaces for admission control | None |

#### Runtime Sensor (DaemonSet with eBPF)

| Field | Description | Default |
|-------|-------------|---------|
| `spec.runtimeSensor.enabled` | Deploy the Runtime Sensor | `false` |
| `spec.runtimeSensor.image.repository` | Container image | `qualys/runtime-sensor` |
| `spec.runtimeSensor.logging.logLevel` | Log verbosity (0-5) | `3` |

#### Shared Settings

| Field | Description | Default |
|-------|-------------|---------|
| `spec.containerRuntime.type` | `auto`, `containerd`, `cri-o`, `docker` | `auto` |
| `spec.scheduling.nodeSelector` | Node selector for DaemonSets | `kubernetes.io/os: linux` |
| `spec.scheduling.tolerations` | Pod tolerations | All taints |
| `spec.scheduling.priorityClassName` | Priority class | `system-node-critical` |

## External Secrets Integration

To use External Secrets Operator instead of native Secrets:

```yaml
apiVersion: qualys.qualys.io/v1alpha1
kind: QualysPlatformConfig
metadata:
  name: qualys-platform
spec:
  platform:
    serverUri: "https://qagpublic.qg2.apps.qualys.com/CloudAgent/"
  credentials:
    sourceType: externalSecret
    externalSecretRef:
      name: qualys-credentials
      namespace: qualys
      secretStoreRef:
        name: vault-backend
        kind: ClusterSecretStore
      keyMappings:
        activationId: "qualys/activation-id"
        customerId: "qualys/customer-id"
```

## Qualys Platform URLs

| Region | URL |
|--------|-----|
| US Platform 1 | `https://qagpublic.qg1.apps.qualys.com/CloudAgent/` |
| US Platform 2 | `https://qagpublic.qg2.apps.qualys.com/CloudAgent/` |
| EU Platform | `https://qagpublic.qg1.apps.qualys.eu/CloudAgent/` |

## Status and Monitoring

```bash
kubectl get qualysplatformconfig qualys-platform -o yaml
kubectl get qualyscloudagent -n qualys
kubectl get qualyscontainersecurity -n qualys
kubectl get pods -n qualys -l app.kubernetes.io/managed-by=qualys-nanny
```

Example output:

```
NAME                       CONTAINER   CLUSTER   ADMISSION   RUNTIME   AGE
qualys-container-security  true        true      false       false     2h
```

## Uninstallation

```bash
kubectl delete qualyscloudagent -n qualys --all
kubectl delete qualyscontainersecurity -n qualys --all
kubectl delete qualysplatformconfig --all
make undeploy
make uninstall
```

## Development

```bash
make install
make run
make test
make generate manifests
make build
```

## License

Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
