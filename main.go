package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

type Config struct {
	GitMaxConcurrency   int       `yaml:"GitMaxConcurrency"`
	GitAuth             GitAuth   `yaml:"GitAuth"`
	Owner               []string  `yaml:"owners"`
	GraphMaxConcurrency int       `yaml:"graphMaxConcurrency"`
	GraphAuth           GraphAuth `yaml:"GraphAuth"`
}

func main() {
	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("无法读取配置文件: %v", err)
	}
	var config Config
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		log.Fatalf("无法解析YAML: %v", err)
	}
	gitAuth := config.GitAuth
	owner := config.Owner
	graphAuth := config.GraphAuth
	site := graphAuth.SITE
	siteID := graphAuth.SITE_ID

	jsonTimePatch := "time.json"
	repoTime, err := readJSONFile(jsonTimePatch)
	if err != nil {
		log.Fatalf("Error reading JSON file: %v", err)
	}

	repos, err := gitAuth.getRepoList()
	if err != nil {
		log.Fatalf("Error getting repo list: %v", err)
	}
	var tarFiles []string
	for _, repo := range repos {
		if len(owner) == 0 || containsString(repo.orgs, owner) {
			if repo.lastModified.After(repoTime[repo.repos]) {
				fmt.Println("Cloning", repo.repos)
				err := gitAuth.gitClone(repo.repos)
				if err != nil {
					log.Fatalf("Error cloning repo: %v", err)
				}
				defer func(repoPath string) {
					fmt.Println("Cleaning up", repoPath)
					err := os.RemoveAll(repoPath)
					if err != nil {
						fmt.Println("Error deleting file:", err)
					}
				}(repo.repos)
				fmt.Println("Compressing", repo.repos)
				tar := repo.repos + ".tar.gz"
				err = compress(repo.repos, tar)
				if err != nil {
					log.Fatalf("Error compressing repo: %v", err)
				}
				tarFiles = append(tarFiles, tar)
				repoTime[repo.repos] = repo.lastModified
				fmt.Println(repo.repos, "Done")
			}
		}
	}
	err = writeJSONFile(jsonTimePatch, repoTime)
	if err != nil {
		fmt.Println("Error writing JSON:", err)
		return
	}
	fmt.Println("JSON data written to", jsonTimePatch)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, config.GraphMaxConcurrency)
	for _, tarfile := range tarFiles {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(tarfile string) {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			g := graphAuth.NewGraphHelper()
			g.StartUpdate(site, siteID, tarfile)

			err := os.Remove(tarfile)
			if err != nil {
				fmt.Println("Error deleting file:", err)
			}
		}(tarfile)
	}
	wg.Wait()
	close(semaphore)
}
