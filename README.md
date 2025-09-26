# Go FTP Client

A lightweight, RFC 959-compliant FTP client built from scratch in Go. Features real-time progress tracking, modern passive mode support, and an interactive command-line interface.

## Features

- **Complete FTP Implementation**: RFC 959 compliant with proper multi-line response parsing
- **Real-time Progress**: Download/upload progress with percentage and rate limiting
- **Dual Passive Mode**: Both PASV and EPSV support for NAT/firewall compatibility
- **Interactive REPL**: Clean command-line interface with extensible command system
- **Connection Management**: Background keepalive prevents server timeouts
- **Graceful Handling**: Proper TCP shutdown eliminates connection hang issues

## Quick Start

```bash
# Build the client
go build -o goftp .

# Connect to a server
./goftp -host test.rebex.net -user anonymous -pass test@example.com

# Use the interactive shell
go-ftp> auth
go-ftp> pasv
go-ftp> list
go-ftp> retr filename.txt
go-ftp> quit
```

## Available Commands

- `auth` - Authenticate with server
- `pwd` - Show current directory
- `list` - List directory contents
- `cwd <dir>` - Change directory
- `cdup` - Go to parent directory
- `retr <file>` - Download file with progress
- `stor <file>` - Upload file with progress
- `pasv` / `epsv` - Enter passive mode
- `size <file>` - Get file size
- `help` - Show all commands

## Architecture

Clean modular design split across focused files:
- `main.go` - CLI argument handling and entry point
- `ftp_connection.go` - Core FTP protocol and connection management
- `command_registry.go` - Extensible command dispatch system
- `ftp_commands.go` - Individual command implementations

## Example Session

```
$ ./goftp -host test.rebex.net -user anonymous -pass test@example.com
220-Welcome to test.rebex.net!
go-ftp> auth
230 User 'anonymous' logged in.
go-ftp> pasv
227 Entering Passive Mode (194,108,117,16,4,15)
go-ftp> retr pocketftp.png
150 Opening 'BINARY' data connection.
Progress: 58024/58024 bytes (100.0%)
Downloaded pocketftp.png (58024 bytes)
226 Transfer complete.
```

## Requirements

- Go 1.24.2 or later
- Network access to FTP servers

## Notes

Built as a learning exercise to understand FTP protocol internals and network programming in Go. Demonstrates clean architecture patterns and RFC-compliant protocol implementation.