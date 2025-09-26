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

type ProgressReader struct {
	io.Reader
	total int64
	read  int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.read += int64(n)
	percentage := (float64(pr.read) / float64(pr.total)) * 100
	fmt.Printf("\rProgress: %d/%d bytes (%.1f%%)", pr.read, pr.total, percentage)
	time.Sleep(time.Second)
	return n, err
}

func isSuccessResponse(response string) bool {
	return len(response) > 0 && strings.HasPrefix(response, "2")
}

func requireAuth(conn *FTPConnection) error {
	if !conn.isAuthenticated {
		return fmt.Errorf("not authenticated - use 'auth' command first")
	}
	return nil
}

func handleAuthenticate(conn *FTPConnection, args []string) error {
	cmd := fmt.Sprintf("USER %s", conn.user)
	resp, err := conn.sendCommand(cmd)
	if err != nil {
		return err
	}
	fmt.Print(resp)

	if !strings.HasPrefix(resp, "331") {
		return fmt.Errorf("USER command failed: %s", strings.TrimSpace(resp))
	}

	cmd = fmt.Sprintf("PASS %s", conn.pass)
	resp, err = conn.sendCommand(cmd)
	if err != nil {
		return err
	}

	if !isSuccessResponse(resp) {
		return fmt.Errorf("PASS command failed: %s", strings.TrimSpace(resp))
	}
	fmt.Print(resp)
	conn.isAuthenticated = true
	conn.startKeepAlive()
	return nil
}

func handlePWD(conn *FTPConnection, args []string) error {
	if err := requireAuth(conn); err != nil {
		return err
	}
	resp, err := conn.sendCommand("PWD")
	if err != nil {
		return err
	}

	if !isSuccessResponse(resp) {
		return fmt.Errorf("PWD failed: %s", strings.TrimSpace(resp))
	}

	fmt.Print(resp)
	return nil
}

func handlePasv(conn *FTPConnection, args []string) error {
	if err := requireAuth(conn); err != nil {
		return err
	}

	resp, err := conn.sendCommand("PASV")
	if err != nil {
		return err
	}
	if !isSuccessResponse(resp) {
		return fmt.Errorf("PASV failed: %s", strings.TrimSpace(resp))
	}
	addr, err := parseAddr(resp)
	if err != nil {
		return err
	}
	conn.dataAddr = addr
	fmt.Print(resp)
	return nil
}

func handleEpsv(conn *FTPConnection, args []string) error {
	if err := requireAuth(conn); err != nil {
		return err
	}

	resp, err := conn.sendCommand("EPSV")
	if err != nil {
		return err
	}
	if !isSuccessResponse(resp) {
		return fmt.Errorf("EPSV failed: %s", strings.TrimSpace(resp))
	}
	addr, err := conn.parseEPSVAddr(resp)
	if err != nil {
		return err
	}
	conn.dataAddr = addr
	fmt.Print(resp)
	return nil
}

func handleList(conn *FTPConnection, args []string) error {
	if err := requireAuth(conn); err != nil {
		return err
	}

	if conn.dataAddr == "" {
		return fmt.Errorf("no data connection available - run 'pasv' command first")
	}

	listCmd := "LIST"
	if len(args) > 0 {
		listCmd = fmt.Sprintf("LIST %s", args[0])
	}

	resp, err := conn.sendCommand(listCmd)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "150") {
		return fmt.Errorf("LIST failed: %s", strings.TrimSpace(resp))
	}
	fmt.Print(resp)

	dataConn, err := net.Dial("tcp", conn.dataAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to data port: %v", err)
	}
	defer dataConn.Close()

	scanner := bufio.NewScanner(dataConn)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading directory listing: %v", err)
	}

	if tcpConn, ok := dataConn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
		tcpConn.CloseRead()
	}

	resp, err = conn.readResponse()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "226") {
		if strings.HasPrefix(resp, "426") {
			fmt.Printf("WARNING - transfer complete, but data connection didn't close gracefully\n")
			return nil
		} else {
			return fmt.Errorf("transfer did not complete successfully: %s", strings.TrimSpace(resp))
		}
	}
	fmt.Print(resp)

	return nil
}

func handleStor(conn *FTPConnection, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must provide filename to upload")
	}
	if err := requireAuth(conn); err != nil {
		return err
	}
	if conn.dataAddr == "" {
		return fmt.Errorf("no data connection avaialable - run 'pasv' command first")
	}

	filename := args[0]

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %v", filename, err)
	}
	defer file.Close()

	resp, err := conn.sendCommand(fmt.Sprintf("STOR %s", filename))
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "150") {
		return fmt.Errorf("STOR failed: %s", strings.TrimSpace(resp))
	}
	fmt.Print(resp)

	dataConn, err := net.Dial("tcp", conn.dataAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to data port: %v", err)
	}
	fileInfo, _ := file.Stat()
	totalSize := fileInfo.Size()

	progressReader := &ProgressReader{
		Reader: file,
		total:  totalSize,
	}

	n, err := io.Copy(dataConn, progressReader)
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}
	if totalSize > 0 {
		//print a new line if transfer was successful
		fmt.Println()
	}
	fmt.Printf("Uploaded %s (%d bytes)\n", filename, n)
	if tcpConn, ok := dataConn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
		tcpConn.CloseRead()
	}

	resp, err = conn.readResponse()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "226") {
		if strings.HasPrefix(resp, "426") {
			fmt.Printf("WARNING - transfer complete, but data connection didn't close gracefully\n")
			return nil
		} else {
			return fmt.Errorf("transfer did not complete successfully: %s", strings.TrimSpace(resp))
		}
	}
	fmt.Print(resp)
	return nil
}

func (conn *FTPConnection) getFileSize(filename string) (int64, error) {
	resp, err := conn.sendCommand(fmt.Sprintf("SIZE %s", filename))
	if err != nil {
		return 0, err
	}

	if strings.HasPrefix(resp, "213") {
		parts := strings.Fields(resp)
		if len(parts) >= 2 {
			return strconv.ParseInt(parts[1], 10, 64)
		}
	}

	return 0, fmt.Errorf("could not determine file size")
}

func handleSize(conn *FTPConnection, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must provide a filename")
	}
	if err := requireAuth(conn); err != nil {
		return err
	}
	if conn.dataAddr == "" {
		return fmt.Errorf("no data connection avaialable - run 'pasv' command first")
	}
	cmd := fmt.Sprintf("SIZE %s", args[0])
	resp, err := conn.sendCommand(cmd)
	if err != nil {
		return err
	}
	// SIZE command returns "213 <size>" on success
	if strings.HasPrefix(resp, "213") {
		parts := strings.Fields(resp)
		if len(parts) >= 2 {
			fmt.Printf("File size: %s bytes\n", parts[1])
		}
		fmt.Print(resp)
	} else {
		return fmt.Errorf("SIZE failed: %s", strings.TrimSpace(resp))
	}

	return nil
}

func handleRetr(conn *FTPConnection, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("must provide at least the filepath of the file you want to retrieve")
	}
	if err := requireAuth(conn); err != nil {
		return err
	}
	if conn.dataAddr == "" {
		return fmt.Errorf("no data connection avaialable - run 'pasv' command first")
	}
	filename := args[0]
	totalSize, err := conn.getFileSize(filename)
	if err != nil {
		fmt.Printf("Warning: could not get file size - %v\n", err)
		totalSize = 0
	}
	cmd := fmt.Sprintf("RETR %s", args[0])
	resp, err := conn.sendCommand(cmd)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(resp, "150") {
		return fmt.Errorf("RETR failed: %s", strings.TrimSpace(resp))
	}
	fmt.Print(resp)

	dataConn, err := net.Dial("tcp", conn.dataAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to data port: %v", err)
	}
	defer dataConn.Close()

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", filename, err)
	}
	defer file.Close()

	progressReader := &ProgressReader{
		Reader: dataConn,
		total:  totalSize,
	}
	n, err := io.Copy(file, progressReader)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	if totalSize > 0 {
		//print a new line if transfer was successful
		fmt.Println()
	}

	fmt.Printf("Downloaded %s (%d bytes)\n", filename, n)
	if tcpConn, ok := dataConn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
		tcpConn.CloseRead()
	}

	resp, err = conn.readResponse()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(resp, "226") {
		if strings.HasPrefix(resp, "426") {
			fmt.Printf("WARNING - transfer complete, but data connection didn't close gracefully\n")
			return nil
		} else {
			return fmt.Errorf("transfer did not complete successfully: %s", strings.TrimSpace(resp))
		}
	}
	fmt.Print(resp)
	return nil
}

func handleStat(conn *FTPConnection, args []string) error {
	// TODO: handle arguments (acts like list)
	cmd := "STAT"
	resp, err := conn.sendCommand(cmd)
	if err != nil {
		return err
	}
	if !isSuccessResponse(resp) {
		return fmt.Errorf("STAT failed: %s", strings.TrimSpace(resp))
	}
	fmt.Print(resp)
	return nil
}

func handleCdup(conn *FTPConnection, args []string) error {
	if err := requireAuth(conn); err != nil {
		return err
	}

	resp, err := conn.sendCommand("CDUP")
	if err != nil {
		return err
	}
	if !isSuccessResponse(resp) {
		return fmt.Errorf("CDUP failed: %s", strings.TrimSpace(resp))
	}
	fmt.Print(resp)

	return nil
}

func handleCWD(conn *FTPConnection, args []string) error {
	if err := requireAuth(conn); err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("must provide a destination directory")
	}

	cwdCmd := fmt.Sprintf("CWD %s", args[0])
	resp, err := conn.sendCommand(cwdCmd)
	if err != nil {
		return err
	}
	if !isSuccessResponse(resp) {
		return fmt.Errorf("CWD failed: %s", strings.TrimSpace(resp))
	}
	fmt.Print(resp)

	return nil
}

func handleHelp(conn *FTPConnection, args []string) error {
	resp, err := conn.sendCommand("HELP")
	if err != nil {
		return err
	}

	if !isSuccessResponse(resp) {
		return fmt.Errorf("PWD failed: %s", strings.TrimSpace(resp))
	}

	fmt.Print(resp)
	return nil
}

func handleHelpMenu(conn *FTPConnection, args []string) error {
	fmt.Println("Supported commands:")
	for _, v := range commandRegistry {
		fmt.Printf(" %s - %s\n", v.name, v.description)
	}
	fmt.Println()
	return nil
}

func handleExit(conn *FTPConnection, args []string) error {
	defer conn.stopKeepAlive()
	fmt.Println("Goodbye!")
	resp, err := conn.sendCommand("QUIT")
	if err != nil {
		return err
	}

	if !isSuccessResponse(resp) {
		return fmt.Errorf("PWD failed: %s", strings.TrimSpace(resp))
	}
	fmt.Println(resp)
	conn.Close()
	os.Exit(0)
	return nil
}
