package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		backup([]string{})
		return 
	}

	switch(args[0]) {
		case "backup": {
			backup(args[1:])
		}

		case "login": {
			login()
		}

		case "help": {
			fmt.Println(`ghbackup is a tool to backup repositories off your github profile and organizations. It will backup all branches off every repository it can access. The two commands are backup and login, and backup can take an argument for a folder to backup things. Your credentials are stored in ~/.ghbackup/.auth and ~/.ghbackup is the default backup folder.`)
		}
	}
}