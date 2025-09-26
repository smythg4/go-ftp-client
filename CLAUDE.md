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