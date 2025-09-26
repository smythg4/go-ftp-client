package main

type cliCommand struct {
	callback    func(*FTPConnection, []string) error
	description string
	name        string
}

var commandRegistry map[string]cliCommand

func init() {
	commandRegistry = map[string]cliCommand{
		"auth": {
			name:        "auth",
			description: "Authenticate with saved username and password.",
			callback:    handleAuthenticate,
		},
		"pwd": {
			name:        "pwd",
			description: "Print working directory.",
			callback:    handlePWD,
		},
		"pasv": {
			name:        "pasv",
			description: "Request server-DTP to \"listen\" on a data port (which is not its default data port) and to wait for a connection",
			callback:    handlePasv,
		},
		"epsv": {
			name:        "epsv",
			description: "Enter into EPSV mode",
			callback:    handleEpsv,
		},
		"list": {
			name:        "list",
			description: "Fetch list from server to the passive DTP.",
			callback:    handleList,
		},
		"cwd": {
			name:        "cwd <pathname>",
			description: "Change the working directory with desired directory as argument.",
			callback:    handleCWD,
		},
		"cdup": {
			name:        "cdup",
			description: "Change working directory to parent directory.",
			callback:    handleCdup,
		},
		"retr": {
			name:        "retr <pathname>",
			description: "Transfer a copy of the file specified in the pathname from server-DTP",
			callback:    handleRetr,
		},
		"stor": {
			name:        "stor <filename>",
			description: "Upload a file to the server.",
			callback:    handleStor,
		},
		"stat": {
			name:        "stat <pathname> (optional)",
			description: "Receive status on action in progress",
			callback:    handleStat,
		},
		"size": {
			name:        "size <pathname>",
			description: "Display size of file on server.",
			callback:    handleSize,
		},
		"quit": {
			name:        "quit",
			description: "Exit the Go-FTP client.",
			callback:    handleExit,
		},
		"help": {
			name:        "help",
			description: "Display a help message.",
			callback:    handleHelpMenu,
		},
		"serverhelp": {
			name:        "serverhelp",
			description: "Display a help message from the server.",
			callback:    handleHelp,
		},
	}
}
