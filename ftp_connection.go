package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type FTPConnection struct {
	conn            net.Conn
	user            string
	pass            string
	reader          *bufio.Reader
	isAuthenticated bool
	dataAddr        string
	keepaliveStop   chan struct{}
	keepaliveDone   chan struct{}
	connectionLost  chan struct{}
}

func NewFTPConnection(host, user, pass string) (FTPConnection, error) {
	addr := host + ":2121"
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return FTPConnection{}, err
	}

	return FTPConnection{
		conn:            conn,
		user:            user,
		pass:            pass,
		reader:          bufio.NewReader(conn),
		isAuthenticated: false,
		connectionLost:  make(chan struct{}),
	}, nil
}

func (f *FTPConnection) parseEPSVAddr(epsvResp string) (string, error) {
	start := strings.Index(epsvResp, "(")
	end := strings.Index(epsvResp, ")")

	if start == -1 || end == -1 {
		return "", fmt.Errorf("invalid EPSV response format")
	}

	portPart := epsvResp[start+1 : end]
	parts := strings.Split(portPart, "|")

	if len(parts) < 4 {
		return "", fmt.Errorf("invalid EPSV port format")
	}

	port := parts[3]
	controlAddr := f.conn.RemoteAddr().String()
	host, _, _ := net.SplitHostPort(controlAddr)
	return fmt.Sprintf("%s:%s", host, port), nil
}

func parseAddr(pasvResp string) (string, error) {
	// Find first '(' and split
	_, after, found := strings.Cut(pasvResp, "(")
	if !found {
		return "", fmt.Errorf("no opening parenthesis found")
	}

	// Find first ')' and split
	numbersStr, _, found := strings.Cut(after, ")")
	if !found {
		return "", fmt.Errorf("no closing parenthesis found")
	}

	parts := strings.Split(numbersStr, ",")
	if len(parts) != 6 {
		return "", fmt.Errorf("expected 6 numbers, got %d", len(parts))
	}
	for i, part := range parts[0:4] {
		if num, err := strconv.Atoi(part); err != nil || num < 0 || num > 255 {
			return "", fmt.Errorf("invalid IP octet at position %d: %s", i, part)
		}
	}
	addr := strings.Join(parts[0:4], ".")

	portH, err := strconv.Atoi(parts[4])
	if err != nil {
		return "", fmt.Errorf("error parsing port number high digit")
	}
	portL, err := strconv.Atoi(parts[5])
	if err != nil {
		return "", fmt.Errorf("error parsing port number low digit")
	}
	portVal := portH*256 + portL

	return fmt.Sprintf("%s:%d", addr, portVal), nil
}

func (f *FTPConnection) readResponse() (string, error) {
	// Refresh read deadline for this operation
	f.conn.SetReadDeadline(time.Now().Add(45 * time.Second))

	var fullResponse strings.Builder

	line, err := f.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	fullResponse.WriteString(line)

	// check if multiline
	if len(line) >= 4 && line[3] == '-' {
		code := line[:3]

		for {
			line, err = f.reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			fullResponse.WriteString(line)

			if len(line) >= 4 && line[:3] == code && line[3] == ' ' {
				break
			}
		}
	}

	return fullResponse.String(), nil
}

func (f *FTPConnection) sendCommand(cmd string) (string, error) {
	// Refresh write deadline for this operation
	f.conn.SetWriteDeadline(time.Now().Add(15 * time.Second))

	_, err := fmt.Fprintf(f.conn, "%s\r\n", cmd)
	if err != nil {
		return "", err
	}

	return f.readResponse()
}

func (f *FTPConnection) startKeepAlive() {
	f.keepaliveStop = make(chan struct{})
	f.keepaliveDone = make(chan struct{})

	go func() {
		defer close(f.keepaliveDone)
		normalInterval := 30 * time.Second
		extendedInterval := 2 * time.Minute
		ticker := time.NewTicker(normalInterval)
		consecutiveSuccess := 0
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if f.isAuthenticated {
					resp, err := f.sendCommand("NOOP")
					if err != nil {
						if f.isConnectionDead(err) {
							fmt.Printf("\n*** Server connection lost: %v ***\n", err)
							close(f.connectionLost) // signal to main
						} else {
							fmt.Printf("\n Keepalive failed: %v\n", err)
							consecutiveSuccess = 0
						}
						return
					}
					// Clean keepalive display - use \r to overwrite prompt temporarily
					fmt.Printf("\rKeepalive: %s\ngo-ftp> ", strings.TrimSpace(resp))
					consecutiveSuccess++
					if consecutiveSuccess > 5 {
						ticker.Reset(extendedInterval)
					} else {
						ticker.Reset(normalInterval)
					}
				}
			case <-f.keepaliveStop:
				return
			}
		}
	}()
}

func (f *FTPConnection) isConnectionDead(err error) bool {
	if err == nil {
		return false
	}

	if err == io.EOF {
		return true
	}

	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return true
		}
	}

	errStr := err.Error()
	if strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "network is unreachable") {
		return true
	}

	return false
}

func (f *FTPConnection) stopKeepAlive() {
	if f.keepaliveStop != nil {
		close(f.keepaliveStop)
		<-f.keepaliveDone // wait for goroutine to finish
	}
}

func cleanInput(input string) []string {
	return strings.Fields(strings.ToLower(strings.TrimSpace(input)))
}

func (f *FTPConnection) Close() error {
	return f.conn.Close()
}

func (f *FTPConnection) StartREPL() {

	welcome, err := f.readResponse()
	if err != nil {
		fmt.Printf("Error reading welcome message: %v\n", err)
		return
	}
	fmt.Print(welcome)

	// Create input channel and start input reader goroutine
	inputChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			if scanner.Scan() {
				inputChan <- scanner.Text()
			} else {
				close(inputChan)
				return
			}
		}
	}()

	// Initial prompt
	fmt.Print("go-ftp> ")

	// Main REPL loop
	for {
		select {
		case <-f.connectionLost:
			fmt.Printf("*** Shutting down gracefully ***\n")
			f.stopKeepAlive()
			f.Close()
			return
		case input, ok := <-inputChan:
			if !ok {
				// Input channel closed (EOF)
				fmt.Printf("\nGoodbye!\n")
				f.stopKeepAlive()
				f.Close()
				return
			}
			args := cleanInput(input)
			if len(args) > 0 {
				if cmd, ok := commandRegistry[args[0]]; ok {
					err := cmd.callback(f, args[1:])
					if err != nil {
						fmt.Printf("Error: %v\n", err)
					}
				} else {
					fmt.Println("Unknown command")
				}
			}
			fmt.Print("go-ftp> ")
		}
	}
}
