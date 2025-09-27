# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A fully functional FTP client (`goftp`) written in Go 1.24.2, implementing RFC 959 with modern features like real-time progress tracking, keepalive connections, and graceful error handling.

## Development Commands

### Build and Run
```bash
go build .                                    # Build the project
./goftp -host <server> -user <user> -pass <pass>  # Run with connection parameters
```

Example usage:
```bash
./goftp -host test.rebex.net -user anonymous -pass test@example.com
```

### Code Quality
```bash
go fmt ./...                 # Format all Go files
go vet ./...                 # Static analysis for potential issues
go mod tidy                  # Clean up module dependencies
```

## Architecture Overview

### Multi-File Structure
- **`main.go`** - Entry point and command-line argument handling
- **`ftp_connection.go`** - Core FTP connection management, protocol handling, and connection utilities
- **`command_registry.go`** - Command dispatch system with extensible registry pattern
- **`ftp_commands.go`** - Implementation of all FTP command handlers

### Key Components

#### FTPConnection Management
- **Control Connection**: Single persistent connection for commands (RFC 959)
- **Data Connections**: Ephemeral connections for file transfers (PASV/EPSV modes)
- **Multi-line Response Parser**: RFC 959 compliant parsing for server responses
- **Keepalive System**: Background NOOP commands prevent server timeouts

#### Command System
- **Registry Pattern**: Extensible command dispatch in `commandRegistry` map
- **Consistent Interface**: All handlers follow `func(*FTPConnection, []string) error`
- **Authentication State**: Commands automatically check auth requirements
- **Error Propagation**: Clean error handling from protocol to user interface

#### Data Transfer Features
- **Progress Tracking**: Real-time download progress with rate limiting
- **Graceful Shutdown**: Proper TCP connection termination prevents server errors
- **File Size Detection**: Uses SIZE command (RFC 3659) for accurate progress percentages
- **Binary/Text Support**: Handles all file types through `io.Copy` operations

### Protocol Implementation
- **RFC 959 Core**: Complete FTP protocol implementation
- **RFC 3659 Extensions**: SIZE command for file information
- **RFC 2428 Support**: EPSV (Extended Passive Mode) for NAT/firewall compatibility
- **Robust Parsing**: Handles multi-line responses, various server implementations

### Connection Patterns
- **PASV/EPSV**: Passive mode data connections (preferred for NAT traversal)
- **Response Validation**: Checks server response codes for proper error handling
- **Connection Reuse**: Control connection maintained throughout session
- **Cleanup**: Proper resource management with defer patterns

## Adding New Commands

1. Add command entry to `commandRegistry` in `command_registry.go`
2. Implement handler function in `ftp_commands.go` following the signature:
   ```go
   func handleNewCommand(conn *FTPConnection, args []string) error
   ```
3. Use `requireAuth(conn)` for commands needing authentication
4. Follow existing patterns for data connection commands vs control-only commands

## Known Issues & Improvements Needed

### Security Issues (High Priority)
- **Password Exposure**: `main.go:15` prints password in plaintext to console
- **Path Traversal**: No input validation on user-provided filenames - could allow directory traversal attacks
- **Non-standard Port**: Hardcoded port 2121 instead of standard FTP port 21 in `ftp_connection.go:25`

### Error Handling (Medium Priority)
- **Ignored Errors**: `ftp_connection.go:56` ignores error from `net.SplitHostPort`
- **No Timeouts**: Missing timeout handling on network operations
- **No Retry Logic**: No retry mechanism for transient network failures
- **Response Code Validation**: Missing response code validation in most command handlers

### Resource Management (Medium Priority)
- **Connection Leaks**: Data connections aren't stored/tracked, potential for leaks
- **Keepalive Cleanup**: `startKeepAlive()` can be called multiple times without proper cleanup
- **Graceful Shutdown**: No graceful shutdown handling for keepalive goroutine on program exit

### Protocol Compliance (Low Priority)
- **Transfer Modes**: No handling of different transfer modes (ASCII vs Binary)
- **EPSV Robustness**: EPSV implementation may not handle all server response formats
- **Extended Commands**: Missing support for other RFC 3659 extensions (MDTM, MLSD, etc.)

### Code Quality (Low Priority)
- **Global State**: Global `commandRegistry` variable instead of proper encapsulation
- **Mixed Responsibilities**: `StartREPL()` handles parsing, execution, and I/O
- **Missing Tests**: No unit tests for critical parsing functions like `parseAddr()`
- **Error Messages**: Inconsistent error message formatting across commands

### Recommended Fix Priority
1. Remove password from console output
2. Add filename validation/sanitization
3. Add proper error handling for `net.SplitHostPort`
4. Implement connection timeout mechanisms
5. Add response code validation
6. Refactor global registry into struct-based approach

## Future FTP Server Considerations

### Concurrent File Access Management
When building the FTP server component, careful consideration of shared filesystem state is critical:

**File-level Race Conditions:**
- Multiple clients uploading same filename simultaneously
- Read/write conflicts on active files
- Partial upload corruption from concurrent access

**Recommended Solutions:**
- **Atomic Operations**: Use temporary files with atomic rename (`file.txt.tmp.clientID` → `file.txt`)
- **Exclusive Creation**: Use `os.O_EXCL` flag to prevent simultaneous uploads to same filename
- **File Locking**: Implement file-level locks or upload tracking map with mutex protection
- **Protocol Response**: Return `550 File busy` or `550 Permission denied` for conflicts

**Example Implementation Pattern:**
```go
type FTPServer struct {
    activeUploads map[string]string  // filename -> clientID
    uploadMutex   sync.RWMutex
    rootDir       string             // jail root directory
}
```

### Security: Filesystem Access Control
Critical security requirement to prevent unauthorized filesystem access:

**Path Traversal Prevention:**
- **Chroot Jail**: Restrict all client operations to designated directory tree
- **Path Validation**: Sanitize all file paths to prevent `../` directory traversal
- **Absolute Path Resolution**: Convert all relative paths to absolute within jail
- **Symlink Handling**: Decide policy on following symbolic links outside jail

**Recommended Implementation:**
```go
func (s *ClientSession) validatePath(userPath string) (string, error) {
    // Clean and resolve path
    cleanPath := filepath.Clean(userPath)

    // Ensure path stays within jail
    absPath := filepath.Join(s.server.rootDir, cleanPath)
    if !strings.HasPrefix(absPath, s.server.rootDir) {
        return "", fmt.Errorf("550 Access denied: path outside allowed directory")
    }

    return absPath, nil
}
```

**Additional Security Considerations:**
- File permission enforcement based on user authentication
- Disk quota limits per user/session
- Filename character restrictions (prevent special chars, control chars)
- Maximum file size limits for uploads
- Rate limiting for operations and bandwidth