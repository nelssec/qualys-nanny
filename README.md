# Qualys Nanny Operator

A Kubernetes operator for automatically deploying and managing Qualys Cloud Agent and Container Security Sensor on OpenShift clusters.

## Overview

The Qualys Nanny Operator simplifies the deployment of Qualys security agents across your OpenShift cluster by:

- Managing Qualys Cloud Agent as a DaemonSet for host-level vulnerability and compliance scanning
- Managing Qualys Container Security Sensor for container image and runtime scanning
- Automatically creating required SecurityContextConstraints on OpenShift
- Supporting both native Kubernetes Secrets and External Secrets Operator for credential management
- Auto-detecting container runtime (containerd, CRI-O, Docker)

## Architecture

The operator manages three Custom Resources:

| CRD | Scope | Purpose |
|-----|-------|---------|
| `QualysPlatformConfig` | Cluster | Shared Qualys platform settings and credentials |
| `QualysCloudAgent` | Namespace | Cloud Agent DaemonSet configuration |
| `QualysContainerSecurity` | Namespace | Container Security Sensor configuration |

## Prerequisites

- OpenShift 4.12+ or Kubernetes 1.25+
- Qualys subscription with Cloud Agent and/or Container Security
- Qualys activation ID and customer ID
- `kubectl` or `oc` CLI configured

## Installation

### Option 1: Deploy from Source

```bash
# Clone the repository
git clone https://github.com/nelssec/qualys-nanny.git
cd qualys-nanny

# Build and push the operator image
make docker-build docker-push IMG=<your-registry>/qualys-nanny:v0.1.0

# Install CRDs
make install

# Deploy the operator
make deploy IMG=<your-registry>/qualys-nanny:v0.1.0
```

### Option 2: Deploy via OLM (OperatorHub)

```bash
# Build and push the bundle
make bundle-build bundle-push BUNDLE_IMG=<your-registry>/qualys-nanny-bundle:v0.1.0

# Run the bundle
operator-sdk run bundle <your-registry>/qualys-nanny-bundle:v0.1.0
```

## Quick Start

### 1. Create the Namespace

```bash
kubectl create namespace qualys-system
```

### 2. Create Credentials Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: qualys-credentials
  namespace: qualys-system
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
      namespace: qualys-system
```

### 4. Deploy Cloud Agent

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

### 5. Deploy Container Security Sensor (Optional)

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
| `spec.image.repository` | Container image | `nelssec/qualys-agent-bootstrapper` |
| `spec.image.tag` | Image tag | `v2.1.0` |
| `spec.config.logLevel` | Log verbosity (0-5) | `3` |
| `spec.config.cmdMaxTimeOut` | Command timeout (seconds) | `1800` |
| `spec.scheduling.tolerations` | Pod tolerations | All taints |
| `spec.scheduling.priorityClassName` | Priority class | `system-node-critical` |
| `spec.resources` | CPU/memory limits | None |

### QualysContainerSecurity

| Field | Description | Default |
|-------|-------------|---------|
| `spec.platformConfigRef.name` | Name of QualysPlatformConfig | Required |
| `spec.image.repository` | Container image | `qualys/qcs-sensor` |
| `spec.containerRuntime.type` | `auto`, `containerd`, `cri-o`, `docker` | `auto` |
| `spec.sensorConfig.mode` | `general`, `registry`, `cicd` | `general` |
| `spec.sensorConfig.k8sMode` | Enable Kubernetes integration | `true` |
| `spec.sensorConfig.scanning.enableImageScan` | Enable image scanning | `true` |
| `spec.sensorConfig.scanning.enableContainerScan` | Enable container scanning | `true` |

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
      namespace: qualys-system
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

Check deployment status:

```bash
# Platform config status
kubectl get qualysplatformconfig qualys-platform -o yaml

# Cloud Agent status
kubectl get qualyscloudagent -n qualys-system

# Container Security status
kubectl get qualyscontainersecurity -n qualys-system

# View DaemonSet pods
kubectl get pods -n qualys-system -l app.kubernetes.io/managed-by=qualys-nanny
```

## Uninstallation

```bash
# Delete CRs
kubectl delete qualyscloudagent -n qualys-system --all
kubectl delete qualyscontainersecurity -n qualys-system --all
kubectl delete qualysplatformconfig --all

# Uninstall operator
make undeploy

# Remove CRDs
make uninstall
```

## Development

```bash
# Run locally (outside cluster)
make install
make run

# Run tests
make test

# Generate manifests
make generate manifests

# Build binary
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
