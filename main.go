package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	host := flag.String("host", "", "FTP server hostname")
	user := flag.String("user", "anonymous", "Username")
	pass := flag.String("pass", "", "Password")
	flag.Parse()

	fmt.Printf("Attempting to create FTP connection to: %s with username/pass: %s/%s\n", *host, *user, *pass)

	ftpConn, err := NewFTPConnection(*host, *user, *pass)
	if err != nil {
		log.Fatal(err)
	}
	defer ftpConn.Close()

	ftpConn.StartREPL()
}
