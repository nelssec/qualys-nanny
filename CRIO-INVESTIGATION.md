# CRI-O qscanner Bug Investigation

## Summary

This documents a bug where qscanner fails with "failed to copy layers.json" on CRI-O 1.32 when invoked by the qpa sensor. **Root cause confirmed: qpa sets umask 0117, which prevents qscanner from accessing its own temp directories.**

## Environment

- OpenShift 4.19.22 (ROSA HCP)
- CRI-O 1.32.11-2.rhaos4.19
- qpa sensor 1.41.1-0
- qscanner 4.7.0-1

## The Problem

Image scans fail immediately with:
```
failed to create scan target: failed to create Image artifact: failed to copy layers.json file
```

## Root Cause: umask 0117

The qpa process runs with an unusual umask:
```
qpamon: Umask 0022  (normal)
qpa:    Umask 0117  (broken)
```

When qscanner is forked from qpa, it inherits umask 0117. This causes temp directories to be created as `drw-rw----` (0660) instead of `drwxr-xr-x` (0755) - **missing the execute bit**.

Without the execute bit, you cannot enter a directory, even if you own it:
```bash
$ umask 0117
$ mkdir /tmp/test
$ cd /tmp/test
sh: cd: /tmp/test: Permission denied
```

## Proof

Reproduced the exact failure by manually setting umask 0117:

```bash
# With normal umask - WORKS
$ umask 0022
$ qscanner image <id> ...
# Creates: drwxr-xr-x /tmp/qscanner-833468
# Result: SUCCESS

# With qpa's umask - FAILS
$ umask 0117
$ qscanner image <id> ...
# Creates: drw-rw---- /tmp/qscanner-833693
# Result: failed to copy layers.json file
```

qscanner creates `/tmp/qscanner-XXXXX`, but with umask 0117 it can't enter that directory to copy files into it.

## What I Tested

| Configuration | /tmp | sensor-data | Manual | Sensor |
|--------------|------|-------------|--------|--------|
| emptyDir everywhere | emptyDir | emptyDir | Works | Fails |
| No /tmp mount | native | emptyDir | Works | Fails |
| All hostPath | native | hostPath | Works | Fails |
| Privileged mode | N/A | hostPath | Works | Works |

Also tried different cache locations, initContainers, MountPropagation settings, StaticScanningOnly policy, and single-threaded scanning. None of it made a difference because the real issue is the umask.

## Why Privileged Mode Works

In privileged mode, SELinux enforcement is relaxed. The directory permission check that fails with `drw-rw----` likely gets bypassed. It's not a real fix - it's just hiding the bug.

## Other Things Checked

- **SELinux context**: Same for shell and qpa (`system_u:system_r:spc_t:s0`) - not the issue
- **File descriptors**: Normal set of FDs, no obvious leaks
- **Signal handlers**: Standard configuration
- **layers.json permissions**: `-rw-------` owned by root - accessible

## Workaround

CRI-O deployments need privileged mode until Qualys fixes the umask issue:

```yaml
spec:
  containerSensor:
    privilegeMode: privileged
```

## For the Bug Report

1. qpa process sets umask 0117 (check `/proc/<pid>/status`)
2. qscanner inherits this umask when forked
3. Temp directories created without execute bit
4. qscanner cannot enter its own temp directory
5. Copying layers.json fails

The fix should be for qpa to either:
- Not set umask 0117 in the first place
- Reset umask to 0022 before forking qscanner
- Have qscanner explicitly set its own umask on startup
