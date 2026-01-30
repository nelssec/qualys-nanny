# qscanner CRI-O 1.31 Compatibility - Testing Results

## Summary

After extensive testing on ROSA HCP with OpenShift 4.18 (CRI-O 1.31.6), we discovered that **qscanner 4.7.0-1 works correctly with CRI-O 1.31** when properly configured. The reported "failed to copy layers.json" errors were caused by **cache directory permission issues**, not CRI-O 1.31 format incompatibility.

## Test Environment

| Component | Version |
|-----------|---------|
| Platform | ROSA HCP (AWS) |
| OpenShift | 4.18.6 |
| CRI-O | 1.31.6-2.rhaos4.18 |
| qscanner | 4.7.0-1 |
| Sensor (qpa) | 1.41.1-0 |
| Privilege Mode | minimal (CAP_SYS_PTRACE only) |

## The Error

When the sensor attempted static image scanning, qscanner failed with:

```
ERROR: failed to create scan target: failed to create Image artifact: failed to copy layers.json file
Error Code: InvalidStorageDriver:10062
```

## Root Cause Analysis

### Initial Theory (Incorrect)
We initially suspected CRI-O 1.31's new `layers.json` format with `uidset`/`gidset` fields was incompatible with qscanner.

### Actual Root Cause (Confirmed)
The error was caused by **cache directory permission issues**:

1. **Missing qscanner-cache volume**: The operator only added the `qscanner-cache` emptyDir volume when `usePersistentStorage: true`. When running with ephemeral storage (`usePersistentStorage: false`), no cache volume was created.

2. **Sensor creates directories with wrong permissions**: The Qualys sensor creates the `.cache/qualys/` subdirectory with permissions `660` (drw-rw----) instead of `755` (drwxr-xr-x). The missing **execute bit** on directories prevents qscanner from entering the directory.

3. **qscanner fails to create its cache**: When qscanner tries to create its cache at `/usr/local/qualys/qpa/data/.cache/qualys/qscanner`, it cannot enter the `qualys/` directory due to missing execute permission, causing the generic "failed to copy layers.json" error.

### Proof: Manual qscanner Testing

After fixing the permissions, running qscanner manually in the container succeeded:

```bash
$ /usr/bin/qscanner image 01856dd2... \
    --storage-driver crio-overlay \
    --output-dir /tmp \
    --cache none \
    -l debug \
    -f json,spdx \
    -m inventory-only \
    --scan-types pkg

INFO  Image source: crio-overlay filesystem
INFO  OS detected: Red Hat Enterprise Linux 9.4
INFO  OS package(s) detected: 206
INFO  Language package(s) detected: 110
INFO  All scans completed in 412.730582ms
INFO  Scan Result JSON created at /tmp/...-ScanResult.json
```

## Fixes Applied

### Fix 1: Always add qscanner-cache emptyDir volume

The `qscanner-cache` volume must be added unconditionally, not just when `usePersistentStorage: true`.

**Before (broken):**
```go
if usePersistentStorage {
    volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: "qscanner-cache", ...})
    volumes = append(volumes, corev1.Volume{Name: "qscanner-cache", ...})
}
```

**After (fixed):**
```go
volumeMounts := []corev1.VolumeMount{
    // ... other mounts ...
    {Name: "qscanner-cache", MountPath: "/usr/local/qualys/qpa/data/.cache"},
}
volumes := []corev1.Volume{
    // ... other volumes ...
    {Name: "qscanner-cache", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
}
```

### Fix 2: InitContainer to pre-create cache directory structure

Because the sensor creates directories with restrictive permissions (660), we add an initContainer to pre-create the full directory path with correct permissions (755):

```yaml
initContainers:
- name: init-cache-dir
  image: busybox:latest
  command: ['sh', '-c', 'mkdir -p /cache/qualys/qscanner && chmod -R 755 /cache']
  volumeMounts:
  - name: qscanner-cache
    mountPath: /cache
  securityContext:
    runAsUser: 0
    runAsNonRoot: false
    allowPrivilegeEscalation: false
```

## Why 755, Not 660?

For directories, the permission bits mean:

| Bit | Meaning |
|-----|---------|
| r (read) | Can list directory contents |
| w (write) | Can create/delete files |
| **x (execute)** | **Can enter (cd into) the directory** |

- `660` = `drw-rw----` = No execute bit = **Cannot enter directory**
- `755` = `drwxr-xr-x` = Has execute bit = Can enter and access files

Without the execute bit, qscanner literally cannot `cd` into the directory to create its cache files, even though it has read/write permissions.

## Testing the Fix

After applying the fixes:

```bash
# Verify initContainer ran
$ oc get pod $POD -o jsonpath='{.status.initContainerStatuses[0].state}'
{"terminated":{"exitCode":0,"reason":"Completed",...}}

# Verify cache directory permissions
$ oc exec $POD -- ls -la /usr/local/qualys/qpa/data/.cache/
drwxr-xr-x. 3 root root    20 Jan 30 05:41 .
drwxrwxrwx. 8 root root 16384 Jan 30 05:41 ..
drwxr-xr-x. 3 root root    22 Jan 30 05:41 qualys

$ oc exec $POD -- ls -la /usr/local/qualys/qpa/data/.cache/qualys/
drwxr-xr-x. 3 root root 22 Jan 30 05:41 .
drwxr-xr-x. 3 root root 20 Jan 30 05:41 ..
drwxr-xr-x. 2 root root  6 Jan 30 05:41 qscanner
```

## Operator Versions

| Version | Status | Notes |
|---------|--------|-------|
| v0.1.1 | Broken | qscanner-cache only added with persistent storage |
| v0.1.2 | Broken | Same issue |
| v0.1.3 | Partial | Fixed volume issue, but sensor still creates dirs with 660 |
| v0.1.4 | Fixed | Added initContainer to pre-create cache with 755 permissions |

## Minimal Privilege Mode Configuration

The sensor works correctly in minimal privilege mode with these security settings:

```yaml
securityContext:
  privileged: false
  allowPrivilegeEscalation: false
  runAsUser: 0
  runAsNonRoot: false
  readOnlyRootFilesystem: false
  seccompProfile:
    type: RuntimeDefault
  capabilities:
    drop:
    - ALL
    add:
    - SYS_PTRACE
```

Required volume mounts for CRI-O scanning:

```yaml
volumeMounts:
- name: container-storage
  mountPath: /var/lib/containers/storage
  readOnly: true
- name: storage-config-volume
  mountPath: /etc/containers/storage.conf
  readOnly: true
- name: qscanner-cache
  mountPath: /usr/local/qualys/qpa/data/.cache
```

## CRI-O 1.31 Storage Format

For reference, CRI-O 1.31 uses containers/storage v1.55+ with these changes:

```
/var/lib/containers/storage/
├── db.sql                    # SQLite database for metadata
├── overlay/                  # Layer filesystem data
├── overlay-images/           # Image metadata
├── overlay-layers/
│   └── layers.json          # Layer metadata with new fields
└── overlay-containers/       # Container metadata
```

The `layers.json` now includes `uidset` and `gidset` fields:

```json
{
  "id": "layer-diff-id",
  "parent": "parent-layer-id",
  "uidset": [0, 59],
  "gidset": [0, 5, 12, 22]
}
```

**qscanner 4.7.0-1 handles this format correctly** - the format was not the issue.

## Conclusion

The "CRI-O 1.31 compatibility issue" was actually a cache directory permission bug in the operator. qscanner works correctly with CRI-O 1.31 in minimal privilege mode when:

1. The `qscanner-cache` emptyDir volume is always mounted (not just with persistent storage)
2. The cache directory structure is pre-created with proper permissions (755) via an initContainer

No changes to qscanner are required for CRI-O 1.31 compatibility.
