# asset_cache.go

**File:** server/assetcachepojo/asset_cache.go

## Code Summary

This file implements asset caching functionality that provides a local cache layer for cloud storage files. It handles downloading, uploading, and managing files between local cache and various cloud storage providers.

### Type: AssetFolderCache (lines 19-24)
**Fields:**
- `LocalSyncPath string` - Local filesystem path for cache
- `Keyname string` - Cloud storage key/path
- `CloudStore rootpojo.CloudStore` - Cloud storage configuration
- `Credentials map[string]interface{}` - Cached authentication credentials

### Function: GetFileByName() (lines 26-51)
**Purpose:** Retrieves a file from local cache or downloads from cloud storage
**Inputs:**
- `fileName string` - Name of file to retrieve

**Process:**
1. **Local Cache Lookup (lines 27-33):**
   - Line 27: **PATH TRAVERSAL RISK:** Constructs path without validating fileName
   - Line 30: Attempts to open file from local cache

2. **Cloud Download Fallback (lines 36-48):**
   - Line 36: Checks if file not found and cloud store is remote
   - Line 40: Downloads file from cloud storage
   - Line 47: Attempts to open downloaded file

**Edge Cases:**
- **Line 27:** `fileName` not validated for path traversal (../../../etc/passwd)
- **No file size limits:** Can download arbitrarily large files
- **No authentication checks:** No validation of user permissions
- **Race conditions:** Multiple concurrent requests for same file

### Function: downloadFileFromCloudStore() (lines 54-143)
**Purpose:** Downloads specific file from cloud storage to local cache
**Inputs:**
- `fileName string` - File to download

**Process:**
1. **Credential Setup (lines 56-69):**
   - Lines 58-60: Extracts config name from root path
   - Lines 61-68: **CREDENTIAL EXPOSURE:** Sets credentials in global rclone config

2. **Path Construction (lines 71-76):**
   - Line 74: **PATH INJECTION:** Constructs source path without validation
   - Line 76: **PATH TRAVERSAL:** Constructs destination path with user input

3. **Directory Creation (lines 78-83):**
   - Line 80: Creates directories with fixed permissions (0755)
   - No validation of created directory paths

4. **File Download (lines 85-142):**
   - Line 86: Creates temporary file for atomic download
   - Lines 95-96: **TIMEOUT:** 5-minute timeout for downloads
   - Lines 99-108: **EXTERNAL SERVICE:** Connects to cloud storage
   - Lines 127-130: **UNBOUNDED COPY:** No size limits on file copy
   - Line 136: **ATOMIC RENAME:** Uses rename for atomicity

**Edge Cases:**
- **Line 56:** `fileName` not validated for path traversal
- **Line 74:** Source path construction vulnerable to injection
- **Line 86:** Temporary file path predictable
- **Lines 61-68:** Credentials stored in global config without cleanup
- **Line 127:** No size limits on file downloads
- **No access control:** Downloads any requested file without authorization

### Function: DeleteFileByName() (lines 144-148)
**Purpose:** Deletes file from local cache
**Inputs:**
- `fileName string` - File to delete

**Process:**
- Line 146: **PATH TRAVERSAL:** Direct path construction without validation

**Edge Cases:**
- **Critical Path Traversal:** Can delete any file accessible to process
- **No authorization:** No validation of delete permissions
- **No audit logging:** Deletions not logged

### Function: GetPathContents() (lines 150-176)
**Purpose:** Lists contents of cache directory
**Inputs:**
- `path string` - Directory path to list

**Process:**
- Line 152: **PATH TRAVERSAL:** Constructs path without validation
- Lines 160-172: Builds file information maps

**Edge Cases:**
- **Line 152:** Path traversal allows listing any directory
- **Information disclosure:** Reveals filesystem structure
- **No access control:** No validation of listing permissions

### Function: UploadFiles() (lines 178-221)
**Purpose:** Uploads multiple files to local cache
**Inputs:**
- `files []interface{}` - Array of file objects

**Process:**
1. **File Processing Loop (lines 180-217):**
   - Line 181: **TYPE ASSERTION:** Can panic if not map
   - Lines 188-194: **BASE64 DECODING:** Decodes file contents
   - Lines 199-201: Validates file name presence
   - Line 204: **TYPE ASSERTION:** Can panic if path not string
   - Line 207: **TYPE ASSERTION:** Can panic if name not string
   - Line 210: **FILE WRITE:** Writes with full permissions (os.ModePerm = 0777)

**Edge Cases:**
- **Line 181:** Type assertion panic if array contains non-map
- **Line 195:** Base64 decode without size limits
- **Line 204:** Path traversal through user-provided path
- **Line 207:** Path traversal through user-provided filename
- **Line 210:** Files created with overly permissive 0777 permissions
- **No size limits:** Can upload arbitrarily large files
- **No disk space checks:** Can fill up disk space

### Function: createDirIfNotExist() (lines 223-230)
**Purpose:** Helper to create directories
**Inputs:**
- `dir string` - Directory path to create

**Process:**
- Line 225: Creates directory with 0755 permissions
- Line 227: **PANIC ON ERROR:** Panics instead of returning error

**Edge Cases:**
- **Line 227:** Application crash instead of graceful error handling
- **No path validation:** Can create directories anywhere

## Critical Issues Found

### 🚨 Critical Security Vulnerabilities
1. **Line 27:** Path traversal in `GetFileByName()` - fileName not validated (../../../etc/passwd)
2. **Line 146:** Path traversal in `DeleteFileByName()` - can delete any system file
3. **Line 152:** Path traversal in `GetPathContents()` - can list any directory
4. **Lines 204,207:** Path traversal in `UploadFiles()` - user-controlled file paths
5. **Line 210:** Files written with 0777 permissions exposing sensitive data

### ⚠️ Runtime Safety Issues
6. **Line 181:** Type assertion `files[i].(map[string]interface{})` can panic
7. **Line 204:** Type assertion `file["path"].(string)` can panic
8. **Line 207:** Type assertion `file["name"].(string)` can panic
9. **Line 227:** `panic(err)` crashes application instead of error handling
10. **Race conditions:** Concurrent access to same files not handled

### 🔐 Authentication and Authorization Issues
11. **No access control:** No validation of user permissions for file operations
12. **No authorization checks:** Any user can read/write/delete any cached file
13. **Credential exposure:** Cloud credentials stored in global rclone config
14. **No audit logging:** File operations not logged for security monitoring

### 📂 File System Security Issues
15. **Arbitrary file access:** Can access any file on the system through path traversal
16. **Arbitrary file deletion:** Can delete critical system files
17. **Directory traversal:** Can create files/directories anywhere on filesystem
18. **Overly permissive file permissions:** Files created with 0777 permissions
19. **Predictable temp files:** Temporary file names are predictable

### 🌐 Cloud Storage Security Issues
20. **Credential pollution:** Cloud credentials stored in global configuration
21. **No credential cleanup:** Credentials persist in global config after use
22. **Arbitrary cloud access:** Can download from any configured cloud storage
23. **No rate limiting:** No limits on cloud storage operations
24. **External service dependency:** Vulnerable to cloud storage service attacks

### 💾 Resource Management Issues
25. **No size limits:** Files can be arbitrarily large causing disk exhaustion
26. **No disk space checks:** No validation of available disk space
27. **Memory usage:** Large files processed entirely in memory
28. **No cleanup:** Temporary files may not be properly cleaned up
29. **Unbounded downloads:** No limits on download duration or bandwidth

### ⚙️ Operational Issues
30. **No retry logic:** Failed operations not retried
31. **Hard-coded timeouts:** 5-minute timeout may be insufficient for large files
32. **No progress tracking:** No way to monitor large file operations
33. **No cancellation:** Operations cannot be cancelled once started
34. **Error masking:** Some errors silently ignored or converted to panics

### 🔒 Data Security Issues
35. **Information disclosure:** Directory listings reveal filesystem structure
36. **File content exposure:** Cached files accessible without authorization
37. **Metadata leakage:** File modification times and sizes exposed
38. **No encryption:** Cached files stored in plaintext
39. **No integrity checks:** No validation of downloaded file integrity

### 🏗️ Design Issues
40. **Global state pollution:** Uses global rclone configuration
41. **Mixed concerns:** Combines local caching with cloud storage operations
42. **No abstraction:** Direct filesystem operations without abstraction layer
43. **Hard-coded paths:** Uses OS-specific path separators inappropriately
44. **No configuration:** No way to configure cache behavior or limits