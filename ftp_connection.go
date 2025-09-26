package main

import (
	"bufio"
	"fmt"
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
}

func NewFTPConnection(host, user, pass string) (FTPConnection, error) {
	addr := host + ":2121"
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return FTPConnection{}, err
	}
	return FTPConnection{
		conn:            conn,
		user:            user,
		pass:            pass,
		reader:          bufio.NewReader(conn),
		isAuthenticated: false,
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
	fmt.Fprintf(f.conn, "%s\r\n", cmd)

	return f.readResponse()
}

func (f *FTPConnection) startKeepAlive() {
	f.keepaliveStop = make(chan struct{})
	f.keepaliveDone = make(chan struct{})

	go func() {
		defer close(f.keepaliveDone)
		ticker := time.NewTicker(4 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if f.isAuthenticated {
					resp, err := f.sendCommand("NOOP")
					if err != nil {
						fmt.Printf("\n Keepalive failed: %v\n", err)
						return
					}
					fmt.Printf("\nKeepalive: %s", resp)
					fmt.Print("go-ftp> ")
				}
			case <-f.keepaliveStop:
				return
			}
		}
	}()
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
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("go-ftp> ")
		scanner.Scan()
		args := cleanInput(scanner.Text())
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

	}
}
