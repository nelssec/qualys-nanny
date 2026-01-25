# qscanner CRI-O 1.31 Compatibility Analysis

## Problem Summary

qscanner 4.7.0 fails to perform static image scanning on CRI-O 1.31 (OpenShift 4.18) with the error:

```
ERROR: failed to create scan target: failed to create Image artifact: failed to copy layers.json file
```

The sensor reports error code `InvalidStorageDriver:10062`.

## Root Cause

CRI-O 1.31 uses containers/storage library v1.55+ which introduced changes to the storage layout and metadata format. qscanner's `crio-overlay` driver expects the older format used in CRI-O 1.28 and earlier.

## CRI-O 1.31 Storage Structure

```
/var/lib/containers/storage/
├── db.sql                    # SQLite database (NEW in recent versions)
├── overlay/                  # Layer filesystem data
│   ├── {layer-id}/
│   │   ├── diff/            # Actual layer content
│   │   ├── link             # Short link ID for lowerdir paths
│   │   ├── merged/          # Mount point (when container running)
│   │   ├── work/            # OverlayFS work directory
│   │   └── empty/           # Empty directory
│   └── l/                   # Symlinks using short IDs
│       └── {SHORT_ID} -> ../{layer-id}/diff
├── overlay-images/          # Image metadata
│   ├── images.json          # Image index
│   └── {image-id}/
│       ├── manifest         # OCI manifest
│       └── =base64(...)     # Additional manifests
├── overlay-layers/
│   └── layers.json          # Layer metadata
└── overlay-containers/      # Container metadata
```

### Key Changes in CRI-O 1.31

1. **Database Backend**: Added `db.sql` SQLite database for faster metadata queries
2. **layers.json Format**: New fields added to layer entries:
   ```json
   {
     "id": "layer-diff-id",
     "parent": "parent-layer-id",
     "created": "2026-01-25T04:08:59Z",
     "compressed-diff-digest": "sha256:...",
     "compressed-size": 79262296,
     "diff-digest": "sha256:...",
     "diff-size": 219794432,
     "compression": 2,
     "uidset": [0, 59],           // NEW: UID mappings
     "gidset": [0, 5, 12, 22]     // NEW: GID mappings
   }
   ```
3. **Image Metadata Location**: Big data blobs stored as base64-encoded filenames
4. **Link Files**: Short symbolic link IDs in `overlay/l/` directory

## Required qscanner Changes

### 1. Update layers.json Parser

**Current Issue**: qscanner expects older layers.json format and fails when encountering new fields.

**Fix**: Update the Go struct used to unmarshal layers.json to include new fields:

```go
type LayerEntry struct {
    ID                   string   `json:"id"`
    Parent               string   `json:"parent,omitempty"`
    Created              string   `json:"created"`
    CompressedDiffDigest string   `json:"compressed-diff-digest"`
    CompressedSize       int64    `json:"compressed-size"`
    DiffDigest           string   `json:"diff-digest"`
    DiffSize             int64    `json:"diff-size"`
    Compression          int      `json:"compression"`
    // NEW fields for CRI-O 1.31+
    UIDSet               []int    `json:"uidset,omitempty"`
    GIDSet               []int    `json:"gidset,omitempty"`
    Flags                map[string]interface{} `json:"flags,omitempty"`
    TOCDigest            string   `json:"toc-digest,omitempty"`
    UncompressedDigest   string   `json:"uncompressed-digest,omitempty"`
}
```

### 2. Handle SQLite Database Fallback

**Current Issue**: qscanner only reads JSON files, doesn't query db.sql.

**Fix**: Add optional SQLite database support for metadata queries:

```go
func getLayerInfo(storageRoot string, layerID string) (*LayerEntry, error) {
    // Try JSON first (backwards compatible)
    layersJSON := filepath.Join(storageRoot, "overlay-layers", "layers.json")
    if entry, err := readLayerFromJSON(layersJSON, layerID); err == nil {
        return entry, nil
    }

    // Fall back to SQLite database
    dbPath := filepath.Join(storageRoot, "db.sql")
    if _, err := os.Stat(dbPath); err == nil {
        return readLayerFromDB(dbPath, layerID)
    }

    return nil, fmt.Errorf("layer %s not found", layerID)
}
```

### 3. Fix Layer Path Resolution

**Current Issue**: qscanner may not correctly resolve layer paths through the `l/` symlink directory.

**Fix**: Handle both direct paths and symlinked paths:

```go
func resolveLayerPath(storageRoot string, layerID string) (string, error) {
    // Direct path
    directPath := filepath.Join(storageRoot, "overlay", layerID, "diff")
    if _, err := os.Stat(directPath); err == nil {
        return directPath, nil
    }

    // Read link file for short ID
    linkFile := filepath.Join(storageRoot, "overlay", layerID, "link")
    shortID, err := os.ReadFile(linkFile)
    if err == nil {
        symlinkPath := filepath.Join(storageRoot, "overlay", "l", strings.TrimSpace(string(shortID)))
        if resolved, err := filepath.EvalSymlinks(symlinkPath); err == nil {
            return resolved, nil
        }
    }

    return "", fmt.Errorf("cannot resolve layer path for %s", layerID)
}
```

### 4. Handle Image Manifest Base64 Filenames

**Current Issue**: qscanner expects manifest files with predictable names.

**Fix**: Parse base64-encoded manifest filenames in image directories:

```go
func findImageManifest(imageDir string) (string, error) {
    entries, _ := os.ReadDir(imageDir)
    for _, entry := range entries {
        name := entry.Name()
        // Check for base64-encoded manifest filename
        if strings.HasPrefix(name, "=") {
            decoded, err := base64.StdEncoding.DecodeString(name[1:])
            if err == nil && strings.Contains(string(decoded), "manifest") {
                return filepath.Join(imageDir, name), nil
            }
        }
        // Also check for plain "manifest" file
        if name == "manifest" {
            return filepath.Join(imageDir, name), nil
        }
    }
    return "", fmt.Errorf("manifest not found in %s", imageDir)
}
```

### 5. Update Error Handling

**Current Issue**: qscanner returns generic "failed to copy layers.json" without specifics.

**Fix**: Add detailed error messages for debugging:

```go
func copyLayersJSON(src, dst string) error {
    data, err := os.ReadFile(src)
    if err != nil {
        return fmt.Errorf("failed to read layers.json from %s: %w", src, err)
    }

    // Validate JSON structure before copying
    var layers []LayerEntry
    if err := json.Unmarshal(data, &layers); err != nil {
        return fmt.Errorf("failed to parse layers.json (may be incompatible format): %w", err)
    }

    // Log detected format version for debugging
    if len(layers) > 0 && len(layers[0].UIDSet) > 0 {
        log.Debug("Detected CRI-O 1.31+ layers.json format with uidset/gidset fields")
    }

    return os.WriteFile(dst, data, 0644)
}
```

## Testing Requirements

1. **Unit Tests**: Add test cases with CRI-O 1.31 layers.json format
2. **Integration Tests**: Test against actual CRI-O 1.31 storage on RHCOS 4.18
3. **Regression Tests**: Ensure backwards compatibility with CRI-O 1.28, 1.29, 1.30
4. **Edge Cases**:
   - Images with many layers (100+)
   - Images with large layers (10GB+)
   - Concurrent scanning of multiple images
   - Partially pulled images

## Workarounds Until Fix

1. **Use Dynamic Scanning**: Set `scanningPolicy: DynamicScanningOnly` to skip qscanner entirely
2. **Use DynamicWithStaticScanningAsFallback**: qscanner will fail but sensor falls back to dynamic scanning
3. **Downgrade CRI-O**: Not recommended, but CRI-O 1.28 works with qscanner 4.7.0

## Version Compatibility Matrix

| qscanner | CRI-O 1.28 | CRI-O 1.29 | CRI-O 1.30 | CRI-O 1.31 |
|----------|------------|------------|------------|------------|
| 4.6.x    | ✅         | ✅         | ⚠️         | ❌         |
| 4.7.0    | ✅         | ✅         | ✅         | ❌         |
| 4.8.0+   | ✅         | ✅         | ✅         | ✅ (needed)|

## References

- [containers/storage releases](https://github.com/containers/storage/releases)
- [CRI-O storage documentation](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md)
- [OCI Image Specification](https://github.com/opencontainers/image-spec)
