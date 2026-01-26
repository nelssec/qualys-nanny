# Qualys Container Security - Compatibility & Privilege Mode Overview

## Executive Summary

This document outlines the compatibility requirements, privilege modes, and known limitations for deploying Qualys Container Security on OpenShift/Kubernetes clusters. Key findings include CRI-O version compatibility issues with static scanning and the minimum privilege requirements for different scanning modes.

## Quick Compatibility Reference

### What Works Where

| Environment | Privilege Mode | Static Scan | Dynamic Scan | Malware | Runtime |
|-------------|----------------|:-----------:|:------------:|:-------:|:-------:|
| OpenShift 4.17 (CRI-O 1.30) | minimal | Yes | Yes | - | - |
| OpenShift 4.17 (CRI-O 1.30) | standard | Yes | Yes | Yes | - |
| OpenShift 4.18 (CRI-O 1.31) | minimal | - | Yes | - | - |
| OpenShift 4.18 (CRI-O 1.31) | standard | - | Yes | Yes | - |
| ROSA (any) | minimal | Yes* | Yes | - | - |
| EKS (containerd) | minimal | Yes | Yes | - | - |
| GKE (containerd) | minimal | Yes | Yes | - | - |
| AKS (containerd) | minimal | Yes | Yes | - | - |

*Static scanning on ROSA requires CRI-O < 1.31 (OpenShift 4.17 or earlier)

## Platform Compatibility Matrix

### OpenShift Versions

| OpenShift | CRI-O Version | Static Scanning | Dynamic Scanning | Notes |
|-----------|---------------|:---------------:|:----------------:|-------|
| 4.14.x | 1.27.x | Yes | Yes | Supported |
| 4.15.x | 1.28.x | Yes | Yes | Supported |
| 4.16.x | 1.29.x | Yes | Yes | Supported |
| 4.17.x | 1.30.x | Yes | Yes | **Recommended for static scanning** |
| 4.18.x | 1.31.x | - | Yes | qscanner 4.7.0 incompatible |

### Container Runtime Support

| Runtime | Static Scanning | Dynamic Scanning | Storage Driver |
|---------|:---------------:|:----------------:|----------------|
| CRI-O 1.30 and earlier | Yes | Yes | overlay |
| CRI-O 1.31+ | - | Yes | overlay (new format) |
| containerd | Yes | Yes | overlayfs |
| Docker | Yes | Yes | overlay2 |

### Managed Kubernetes Services

| Service | Container Runtime | Static Scanning | Dynamic Scanning | Tested |
|---------|-------------------|:---------------:|:----------------:|:------:|
| ROSA (4.17) | CRI-O 1.30 | Yes | Yes | Yes |
| ROSA (4.18) | CRI-O 1.31 | - | Yes | Yes |
| EKS | containerd | Yes | Yes | Yes |
| GKE | containerd | Yes | Yes | - |
| AKS | containerd | Yes | Yes | - |

## CRI-O 1.31 Incompatibility

### Problem

qscanner 4.7.0 fails to perform static image scanning on CRI-O 1.31 (OpenShift 4.18) with the error:

```
ERROR: failed to create scan target: failed to create Image artifact: failed to copy layers.json file
Error Code: InvalidStorageDriver:10062
```

### Root Cause

CRI-O 1.31 uses containers/storage library v1.55+ which introduced changes to the storage layout:

1. **New layers.json fields**: Added `uidset`, `gidset`, `toc-digest` fields
2. **SQLite database**: Added `db.sql` for faster metadata queries
3. **Base64-encoded manifest filenames**: Changed image metadata storage

### Workarounds

1. **Use Dynamic Scanning Only**: Set `scanningPolicy: DynamicScanningOnly`
2. **Use OpenShift 4.17**: Deploy on clusters with CRI-O 1.30
3. **Wait for qscanner 4.8.0+**: Expected to include CRI-O 1.31 support

For detailed technical analysis, see [qscanner-crio-131-compatibility.md](qscanner-crio-131-compatibility.md).

## Privilege Modes

The operator supports four privilege modes, each with different capabilities and security implications.

### Mode Comparison

| Feature | Unprivileged | Minimal | Standard | Privileged |
|---------|:------------:|:-------:|:--------:|:----------:|
| Container Scanning | - | Yes | Yes | Yes |
| Image Scanning | - | Yes | Yes | Yes |
| Static Scanning | - | Yes | Yes | Yes |
| Dynamic Scanning | - | Yes | Yes | Yes |
| Malware Detection | - | - | Yes | Yes |
| Secret Detection | - | - | Yes | Yes |
| Runtime Monitoring | - | - | - | Yes |
| SCA Scanning | - | Yes | Yes | Yes |
| Run as Root | - | Yes | Yes | Yes |
| Privileged Container | - | - | - | Yes |
| **Pod Security Standard** | restricted | baseline | baseline | privileged |

### Unprivileged Mode

**Status**: Not currently supported by the Qualys sensor binary.

The sensor binary requires root (UID 0) to execute. Attempts to run as non-root (UID 65534) result in:
```
exec container process '/usr/bin/tini': Permission denied
```

**Security Context**:
```yaml
securityContext:
  privileged: false
  runAsUser: 65534
  runAsNonRoot: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
    add: ["DAC_READ_SEARCH"]
```

### Minimal Mode (Recommended for Most Use Cases)

**Status**: Fully functional for image and container scanning.

This is the lowest privilege mode that works with the current sensor. It provides image and container scanning without requiring privileged access.

**Security Context**:
```yaml
securityContext:
  privileged: false
  runAsUser: 0
  runAsNonRoot: false
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: false
  seccompProfile:
    type: RuntimeDefault
  capabilities:
    drop: ["ALL"]
    add: ["SYS_PTRACE"]
```

**Required Capabilities**:
- `SYS_PTRACE`: Required for container inspection

**Volume Mounts**:
```yaml
volumes:
  - name: runtime-socket
    hostPath:
      path: /var/run/crio/crio.sock  # or containerd.sock
      type: Socket
  - name: host-root
    hostPath:
      path: /
      type: Directory
  - name: container-storage
    hostPath:
      path: /var/lib/containers/storage
      type: Directory
  - name: storage-config-volume
    hostPath:
      path: /etc/containers/storage.conf
      type: File
  - name: qscanner-cache  # Added in v0.1.1
    emptyDir: {}
```

**Limitations**:
- No malware detection
- No secret detection
- No runtime monitoring

### Standard Mode

**Status**: Adds malware and secret detection capability.

**Security Context**:
```yaml
securityContext:
  privileged: false
  runAsUser: 0
  allowPrivilegeEscalation: true
  capabilities:
    drop: ["ALL"]
    add: ["SYS_PTRACE", "SYS_ADMIN", "DAC_READ_SEARCH", "SYS_CHROOT"]
```

**Additional Capabilities**:
- `SYS_ADMIN`: Required for malware detection filesystem operations
- `DAC_READ_SEARCH`: Bypass file read permission checks
- `SYS_CHROOT`: Required for certain scanning operations

### Privileged Mode

**Status**: Full functionality including runtime monitoring.

**Security Context**:
```yaml
securityContext:
  privileged: true
  runAsUser: 0
```

**Use Cases**:
- Runtime threat detection
- Full system visibility
- eBPF-based monitoring

## ROSA/SELinux Specific Considerations

### Tested Configuration (v0.1.1)

The following configuration has been tested and verified on ROSA (OpenShift 4.17.46, CRI-O 1.30):

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
      scanningPolicy: StaticScanningOnly
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

**Problem**: On SELinux-enabled clusters (ROSA, OCP), the sensor failed with:
```
failed to create cache directory: /usr/local/qualys/qpa/data/.cache/qualys/qscanner. Error: permission denied
```

**Root Cause**: The qscanner binary uses `XDG_CACHE_HOME` for its cache directory, but hostPath volumes don't automatically get the correct SELinux labels.

**Solution (v0.1.1)**:
1. Added a dedicated `qscanner-cache` emptyDir volume mounted at `/usr/local/qualys/qpa/data/.cache`
2. Set `XDG_CACHE_HOME=/usr/local/qualys/qpa/data/.cache`
3. emptyDir volumes automatically receive correct SELinux labels

### Storage Recommendations

| Storage Option | SELinux Compatible | Performance | Recommended |
|----------------|:------------------:|:-----------:|:-----------:|
| `usePersistentStorage: false` with emptyDir | Yes | Good | Yes |
| `usePersistentStorage: true` with PVC | Yes | Best | For high-volume scanning |
| hostPath for data | Requires SCC | Best | Not on ROSA |

## Scanning Policies

### StaticScanningOnly

Performs image scanning by directly reading container storage on the node. Requires:
- Read access to `/var/lib/containers/storage`
- Access to container runtime socket
- Compatible CRI-O version (< 1.31) or containerd

**Advantages**:
- Faster scanning
- Lower resource usage
- No container execution required

**Limitations**:
- Not compatible with CRI-O 1.31+
- Requires storage access

### DynamicScanningOnly

Scans images by pulling and analyzing them through the container runtime API.

**Advantages**:
- Works with all CRI-O versions
- No direct storage access needed

**Limitations**:
- Slower scanning
- Higher network usage
- Requires image pull

### DynamicWithStaticScanningAsFallback

Attempts dynamic scanning first, falls back to static if dynamic fails.

**Recommended for**: When you prefer dynamic scanning but want static as backup.

### StaticWithDynamicScanningAsFallback

Attempts static scanning first, falls back to dynamic if static fails.

**Recommended for**: Mixed environments or when CRI-O version is unknown.

## OpenShift SecurityContextConstraints (SCC)

### Minimal Mode SCC

```yaml
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: qualys-container-sensor-minimal
allowHostDirVolumePlugin: true
allowHostIPC: false
allowHostNetwork: false
allowHostPID: false
allowHostPorts: false
allowPrivilegeEscalation: false
allowPrivilegedContainer: false
allowedCapabilities:
  - SYS_PTRACE
defaultAddCapabilities: null
fsGroup:
  type: RunAsAny
readOnlyRootFilesystem: false
requiredDropCapabilities:
  - ALL
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: MustRunAs
supplementalGroups:
  type: RunAsAny
volumes:
  - hostPath
  - emptyDir
  - secret
  - configMap
  - projected
```

### Standard Mode SCC

```yaml
apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: qualys-container-sensor-standard
allowHostDirVolumePlugin: true
allowHostIPC: false
allowHostNetwork: false
allowHostPID: false
allowHostPorts: false
allowPrivilegeEscalation: true
allowPrivilegedContainer: false
allowedCapabilities:
  - SYS_PTRACE
  - SYS_ADMIN
  - DAC_READ_SEARCH
  - SYS_CHROOT
defaultAddCapabilities: null
fsGroup:
  type: RunAsAny
readOnlyRootFilesystem: false
requiredDropCapabilities:
  - ALL
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: MustRunAs
supplementalGroups:
  type: RunAsAny
volumes:
  - hostPath
  - emptyDir
  - secret
  - configMap
  - projected
```

## Deployment Recommendations

### For OpenShift 4.17 and Earlier (CRI-O < 1.31)

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: StaticScanningOnly
    storage:
      usePersistentStorage: false
    logging:
      enableConsoleLogs: true
```

### For OpenShift 4.18+ (CRI-O 1.31)

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: DynamicScanningOnly
    storage:
      usePersistentStorage: false
    logging:
      enableConsoleLogs: true
```

### For ROSA Clusters

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: StaticScanningOnly  # or DynamicScanningOnly for OCP 4.18
    storage:
      usePersistentStorage: false  # Important for SELinux compatibility
    logging:
      enableConsoleLogs: true
      logLevel: 4
  clusterSensor:
    cloudProvider: AWS
```

### For Mixed Environments

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: StaticWithDynamicScanningAsFallback
```

## Troubleshooting

### Static Scanning Fails on CRI-O 1.31

**Symptom**: Error code `InvalidStorageDriver:10062`

**Solution**: Switch to `scanningPolicy: DynamicScanningOnly`

### Sensor Pods Crash with "Permission denied"

**Symptom**: `exec container process '/usr/bin/tini': Permission denied`

**Cause**: Attempting to run in unprivileged mode (non-root)

**Solution**: Use `minimal` mode with `runAsUser: 0`

### Cache Directory Permission Denied (Fixed in v0.1.1)

**Symptom**: `failed to create cache directory: /usr/local/qualys/qpa/data/.cache/qualys/qscanner. Error: permission denied`

**Cause**: SELinux blocking access to hostPath volumes

**Solution**: Upgrade to v0.1.1 which uses emptyDir for qscanner cache

### Storage Conflict Error

**Symptom**: `Constants::runSensorWithoutPersistentStorage : true, isPersistentStorageMapped : true`

**Cause**: Mismatch between storage configuration flag and volume mounts

**Solution**: Ensure `usePersistentStorage` setting matches actual volume configuration

### SCC Permission Denied

**Symptom**: Pod fails to schedule due to SCC restrictions

**Solution**: Ensure the service account has access to the appropriate SCC:
```bash
oc adm policy add-scc-to-user qualys-container-sensor-minimal \
  -z qualys-container-sensor-sa -n qualys
```

### Sensor Exits Immediately with Code 0

**Symptom**: Sensor pod starts but exits immediately with code 0

**Possible Causes**:
1. Invalid credentials
2. Network connectivity issues to Qualys platform
3. Configuration mismatch

**Diagnostic Steps**:
```bash
# Check pod logs
kubectl logs -n qualys -l app.kubernetes.io/component=container-sensor

# Enable verbose logging
spec:
  containerSensor:
    logging:
      logLevel: 5
      enableConsoleLogs: true
```

## Version Compatibility

### Sensor Versions

| Sensor Version | qscanner Version | CRI-O 1.30 | CRI-O 1.31 |
|----------------|------------------|:----------:|:----------:|
| 1.41.1-0 | 4.7.0 | Yes | - |
| TBD | 4.8.0+ | Yes | Yes (expected) |

### Operator Compatibility

| Operator Version | OpenShift | Kubernetes | Key Changes |
|------------------|-----------|------------|-------------|
| 0.1.0 | 4.14 - 4.18 | 1.25 - 1.30 | Initial release |
| 0.1.1 | 4.14 - 4.18 | 1.25 - 1.30 | SELinux cache fix, qscanner-cache volume |

## References

- [CRI-O Storage Documentation](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)
- [containers/storage releases](https://github.com/containers/storage/releases)
- [OpenShift SCC Documentation](https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html)
- [Qualys Container Security Documentation](https://www.qualys.com/docs/qualys-container-security-user-guide.pdf)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
