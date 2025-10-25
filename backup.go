package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/semaphore"
)

const per_page = 20
const concurrent_download = 8

type UserRepo struct {
	Name  string `json:"name"`
	Owner struct {
		Name string `json:"login"`
	} `json:"owner"`
	FullName string `json:"full_name"`
	Branches []string
}

func backup(args []string) {
	log.Println("Fetching user repositories...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	token := getAuthOrLogin()

	ghBackupFolder := homeDir + "/.ghbackup"

	if len(args) > 0 {
		ghBackupFolder = args[0]
	} 

	var userRepos []UserRepo

	page := 1
	for {
		client := &http.Client{}

		req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/user/repos?type=all&per_page=%d&page=%d", per_page, page), nil)
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

		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("error reading request response: %v", err)
		}

		var urs []UserRepo
		err = json.Unmarshal(body, &urs)
		if err != nil {
			log.Fatalf("error unmarshaling user repos: %v", err)
		}

		results := make(chan UserRepo, len(urs))

		for _, ur := range urs {
			go getBranches(token, ur, results)
		}

		for i := 0; i < len(urs); i++ {
			userRepos = append(userRepos, <-results)
		}

		if len(urs) == 0 || len(urs) != per_page {
			break
		} else {
			page++
		}
	}

	var repos []string

	for _, ur := range userRepos {
		for _, branch := range ur.Branches {
			repos = append(repos, fmt.Sprintf("https://github.com/%s/%s", ur.FullName, branch))
		}
	}

	if err = os.Mkdir(ghBackupFolder, 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("error making home data directory: %v", err)
	}

	ghReposFileName := fmt.Sprintf("%s/gh_repos", ghBackupFolder)

	ghRepos, err := os.OpenFile(ghReposFileName, os.O_RDWR, 0666)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error opening gh_repos: %v", err)
	} else if os.IsNotExist(err) {
		ghRepos, err = os.Create(ghReposFileName)

		if err != nil {
			log.Fatalf("error creating gh_repos: %v", err)
		}
	}

	var existingRepos []string
	scanner := bufio.NewScanner(ghRepos)
	for scanner.Scan() {
		existingRepos = append(existingRepos, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error scanning gh_repos: %v", err)
	}

	var newRepos []string

	for _, repo := range existingRepos {
		found := false
		for _, r := range repos {
			if r == repo {
				found = true
				break
			}
		}

		if !found {
			newRepos = append(newRepos, repo)
		}
	}

	newRepos = append(newRepos, repos...)

	list := ""

	for _, repo := range newRepos {
		list += repo + "\n"
	}

	defer ghRepos.Close()

	if _, err := ghRepos.Seek(0, 0); err != nil {
		panic(err)
	}

	if _, err := ghRepos.WriteString(list); err != nil {
		panic(err)
	}

	if err := ghRepos.Truncate(int64(len(list))); err != nil {
		panic(err)
	}

	log.Println("User repositories grabbed and recorded in gh_repos")
	log.Println("Downloading repositories and backing up data")

	if err = os.Mkdir(fmt.Sprintf("%s/data", ghBackupFolder), 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("error making data directory: %v", err)
	}

	var tarballs []struct {
		UserRepo
		Branch string
	}

	for _, ur := range userRepos {
		for _, branch := range ur.Branches {
			tarballs = append(tarballs, struct {
				UserRepo
				Branch string
			}{
				UserRepo: ur,
				Branch:   branch,
			})
		}
	}

	bar := progressbar.Default(int64(len(tarballs)))

	sem := semaphore.NewWeighted(concurrent_download)
	var wg sync.WaitGroup

	for _, tb := range tarballs {
		wg.Add(1)
		go download(token, ghBackupFolder, tb.UserRepo, tb.Branch, bar, sem, &wg)
	}

	wg.Wait()

	log.Println("Backed up all user repositories")
}

func getBranches(token string, userRepo UserRepo, results chan<- UserRepo) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/branches", userRepo.FullName), nil)
	if err != nil {
		log.Fatalf("error creating request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer " + token)
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("error getting branches: %v", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error reading request response: %v", err)
	}

	var branches []struct {
		Name string `json:"name"`
	}

	err = json.Unmarshal(body, &branches)
	if err != nil {
		log.Fatalf("error unmarshaling user repos: %v", err)
	}

	for _, branch := range branches {
		if !strings.Contains(branch.Name, "dependabot") {
			userRepo.Branches = append(userRepo.Branches, branch.Name)
		}
	}

	results <- userRepo
}

func download(token string, dataDir string, userRepo UserRepo, branch string, bar *progressbar.ProgressBar, sem *semaphore.Weighted, wg *sync.WaitGroup) {
	defer wg.Done()

	url := fmt.Sprintf("https://api.github.com/repos/%s/tarball/%s", userRepo.FullName, branch)

	if err := os.Mkdir(dataDir+"/data/"+userRepo.Owner.Name, 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("error making data directory: %v", err)
	}

	fileName := fmt.Sprintf("%s/data/%s/%s-%s.tar.xz", dataDir, userRepo.Owner.Name, userRepo.Name, branch)
	out, err := os.Create(fileName)
	if err != nil && !os.IsExist(err) {
		log.Fatalf("error creating tarball file: %v", err)
	} else if os.IsExist(err) {
		err := os.Remove(fileName)
		if err != nil {
			log.Fatalf("error removing old tarball: %v", err)
		}

		out, err = os.Create(fileName)
		if err != nil {
			log.Fatalf("error creating tarball file: %v", err)
		}
	}

	defer out.Close()

	ctx := context.Background()
	if err := sem.Acquire(ctx, 1); err != nil {
		log.Fatalf("download (%s) failed to acquire semaphore: %v\n", url, err)
		return
	}
	defer sem.Release(1)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 0 {
				req.Header.Set("Authorization", via[0].Header.Get("Authorization"))
				req.Header.Set("User-Agent", via[0].Header.Get("User-Agent"))
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("error creating request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer " + token)
	req.Header.Add("Accept", "application/vnd.github+json")
	req.Header.Add("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("error getting tarball: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("https://api.github.com/repos/%s/tarball/%s\n", userRepo.FullName, branch)
		log.Printf("Status: %v\nBody: %s\n", resp.Status, body)
		bar.Add(1)
		return
	}

	if _, err = io.Copy(out, resp.Body); err != nil {
		log.Printf("https://api.github.com/repos/%s/tarball/%s\n", userRepo.FullName, branch)
		log.Fatalf("error copying tarball to file: %v", err)
	}

	bar.Add(1)
}