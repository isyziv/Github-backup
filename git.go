package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	git "gopkg.in/src-d/go-git.v4"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

type GitAuth struct {
	Username string `yaml:"Username"`
	Token    string `yaml:"Tokens"`
}

type GitRepos struct {
	repos        string
	lastModified time.Time
	orgs         string
}

type Repository struct {
	RepoName     string
	LastModified string
}

func (g *GitAuth) getRepoList() ([]GitRepos, error) {
	repos, err := getReposInfo(g.Token)
	if err != nil {
		return nil, err
	}
	repoArray := strings.Split(repos, ",")
	gitRepos := []GitRepos{}
	for _, repo := range repoArray {
		if strings.Contains(repo, "\"full_name\":\"") {
			tmp := strings.Split(repo, "\"")
			name := strings.Split(tmp[3], "/")
			app := GitRepos{}
			app.repos = tmp[3]
			app.orgs = name[0]
			app.lastModified, err = g.getRepoLastModified(name[1], name[0])
			gitRepos = append(gitRepos, app)
		}
	}
	return gitRepos, nil
}

func (g *GitAuth) getRepoLastModified(repo string, owner string) (time.Time, error) {

	token := g.Token
	var tt time.Time
	// 创建GitHub API请求URL
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?per_page=1", owner, repo)

	// 创建HTTP客户端
	client := &http.Client{}

	// 创建HTTP请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return tt, err
	}

	// 设置请求头，包括GitHub API访问令牌
	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"Authorization":        "Bearer " + token,
		"X-GitHub-Api-Version": "2022-11-28",
	}
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	// 发送HTTP请求
	resp, err := client.Do(req)
	if err != nil {
		return tt, err
	}
	defer resp.Body.Close()

	// 解析API响应
	if resp.StatusCode != http.StatusOK {
		return tt, err
	}

	// 解码JSON响应
	var commits []struct {
		Commit struct {
			Author struct {
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	err = json.NewDecoder(resp.Body).Decode(&commits)
	if err != nil {
		return tt, err
	}

	if len(commits) > 0 {
		lastCommitTime := commits[0].Commit.Author.Date
		return lastCommitTime, nil
	}
	return tt, err
}

func getReposInfo(token string) (string, error) {
	var re string
	baseURL := "https://api.github.com/user/repos"
	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"Authorization":        "Bearer " + token,
		"X-GitHub-Api-Version": "2022-11-28",
	}

	client := &http.Client{}
	for i := 1; ; i++ {
		url := fmt.Sprintf("%s?page=%d", baseURL, i)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Println(err)
			return "", err
		}
		for key, value := range headers {
			req.Header.Add(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
			return "", err
		}
		defer resp.Body.Close()

		sitemap, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
			return "", err
		}
		if string(sitemap) == "[]" {
			break
		} else {
			re = re + string(sitemap)
		}
	}
	return re, nil
}

func (g *GitAuth) gitClone(repo string) error {
	url := "https://github.com/" + repo
	savePath := repo
	auth := githttp.BasicAuth{
		Username: g.Username,
		Password: g.Token,
	}
	opts := git.CloneOptions{
		URL:  url,
		Auth: &auth,
	}
	_, err := git.PlainClone(savePath, false, &opts)
	return err
}

func containsString(str string, strArray []string) bool {
	strMap := make(map[string]bool)
	for _, s := range strArray {
		strMap[s] = true
	}
	return strMap[str]
}
