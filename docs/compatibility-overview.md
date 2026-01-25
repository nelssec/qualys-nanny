# Qualys Container Security - Compatibility & Privilege Mode Overview

## Executive Summary

This document outlines the compatibility requirements, privilege modes, and known limitations for deploying Qualys Container Security on OpenShift/Kubernetes clusters. Key findings include CRI-O version compatibility issues with static scanning and the minimum privilege requirements for different scanning modes.

## Platform Compatibility Matrix

### OpenShift Versions

| OpenShift | CRI-O Version | Static Scanning | Dynamic Scanning | Notes |
|-----------|---------------|-----------------|------------------|-------|
| 4.17.x | 1.30.x | ✅ Works | ✅ Works | Recommended for static scanning |
| 4.18.x | 1.31.x | ❌ Fails | ✅ Works | qscanner 4.7.0 incompatible with new storage format |
| 4.16.x | 1.29.x | ✅ Works | ✅ Works | Supported |

### Container Runtime Support

| Runtime | Static Scanning | Dynamic Scanning | Storage Driver |
|---------|-----------------|------------------|----------------|
| CRI-O 1.30 and earlier | ✅ | ✅ | overlay |
| CRI-O 1.31+ | ❌ | ✅ | overlay (new format) |
| containerd | ✅ | ✅ | overlayfs |
| Docker | ✅ | ✅ | overlay2 |

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

## Privilege Modes

The operator supports four privilege modes, each with different capabilities and security implications.

### Mode Comparison

| Feature | Unprivileged | Minimal | Standard | Privileged |
|---------|--------------|---------|----------|------------|
| Container Scanning | ❌ | ✅ | ✅ | ✅ |
| Image Scanning | ❌ | ✅ | ✅ | ✅ |
| Static Scanning | ❌ | ✅ | ✅ | ✅ |
| Dynamic Scanning | ❌ | ✅ | ✅ | ✅ |
| Malware Detection | ❌ | ❌ | ✅ | ✅ |
| Runtime Monitoring | ❌ | ❌ | ❌ | ✅ |
| SCA Scanning | ❌ | ✅ | ✅ | ✅ |
| Run as Root | ❌ | ✅ | ✅ | ✅ |
| Privileged Container | ❌ | ❌ | ❌ | ✅ |

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

**Status**: ✅ Fully functional for image and container scanning.

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
```

**Limitations**:
- No malware detection
- No runtime monitoring

### Standard Mode

**Status**: ✅ Adds malware detection capability.

**Security Context**:
```yaml
securityContext:
  privileged: false
  runAsUser: 0
  allowPrivilegeEscalation: true
  capabilities:
    drop: ["ALL"]
    add: ["SYS_PTRACE", "SYS_ADMIN", "DAC_READ_SEARCH"]
```

**Additional Capabilities**:
- `SYS_ADMIN`: Required for malware detection filesystem operations
- `DAC_READ_SEARCH`: Bypass file read permission checks

### Privileged Mode

**Status**: ✅ Full functionality including runtime monitoring.

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

Attempts static scanning first, falls back to dynamic if static fails.

**Recommended for**: Mixed environments or when CRI-O version is unknown.

### StaticWithDynamicScanningAsFallback

Attempts static scanning first, falls back to dynamic if static fails. Same behavior as above but with different default preference.

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

## Deployment Recommendations

### For OpenShift 4.17 and Earlier

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: StaticScanningOnly
```

### For OpenShift 4.18+ (CRI-O 1.31)

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: DynamicScanningOnly
```

### For Mixed Environments

```yaml
spec:
  containerSensor:
    privilegeMode: minimal
    scanning:
      scanningPolicy: DynamicWithStaticScanningAsFallback
```

## Troubleshooting

### Static Scanning Fails on CRI-O 1.31

**Symptom**: Error code `InvalidStorageDriver:10062`

**Solution**: Switch to `DynamicScanningOnly` or use OpenShift 4.17.

### Sensor Pods Crash with "Permission denied"

**Symptom**: `exec container process '/usr/bin/tini': Permission denied`

**Cause**: Attempting to run in unprivileged mode (non-root)

**Solution**: Use `minimal` mode with `runAsUser: 0`

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

## Version Compatibility

### Sensor Versions

| Sensor Version | qscanner Version | CRI-O 1.30 | CRI-O 1.31 |
|----------------|------------------|------------|------------|
| 1.41.1-0 | 4.7.0 | ✅ | ❌ |
| TBD | 4.8.0+ | ✅ | ✅ (expected) |

### Operator Compatibility

| Operator Version | OpenShift | Kubernetes |
|------------------|-----------|------------|
| 0.1.0+ | 4.14 - 4.18 | 1.27 - 1.30 |

## References

- [CRI-O Storage Documentation](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)
- [containers/storage releases](https://github.com/containers/storage/releases)
- [OpenShift SCC Documentation](https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html)
- [Qualys Container Security Documentation](https://www.qualys.com/docs/qualys-container-security-user-guide.pdf)
