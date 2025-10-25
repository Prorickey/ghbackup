package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func login() {
	var token string 

	fmt.Println("Create a GitHub Personal Access Token and paste it below. You can create one at this link")
	fmt.Println("https://github.com/settings/tokens/new?scopes=repo,read:org&description=GitHub%20Backup%20Tool")
	fmt.Print("Enter your GitHub token: ")
	fmt.Scan(&token)

	client := http.Client{}

	req, err := http.NewRequest("GET", "https://api.github.com", nil)
	if err != nil {
		log.Fatalf("error creating request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer " + token)
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("error getting user repos: %v", err)
	}

	scopes := strings.Split(resp.Header["X-Oauth-Scopes"][0], ", ")

	correct := false 
	for _, scope := range(scopes) {
		if scope == "repo" {
			correct = true
			break
		}
	}

	if !correct {
		fmt.Println("Token missing (repo) permissions. Please try again. ")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Create(homeDir + "/.ghbackup/.auth")
	if err != nil {
		fmt.Printf("error creating auth file: %v", err)
		return
	}

	_, err = file.WriteString(token)
	if err != nil {
		fmt.Printf("error writing to auth file: %v", err)
		return
	}
}

func getAuthOrLogin() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	authFileName := fmt.Sprintf("%s/.ghbackup/.auth", homeDir)

	auth, err := os.ReadFile(authFileName)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error opening auth file: %v", err)
	} else if os.IsNotExist(err) {
		login()

		auth, err = os.ReadFile(authFileName)
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("error opening auth file (2): %v", err)
		} else if os.IsNotExist(err) {
			return getAuthOrLogin()
		}
	}

	return string(auth)
}