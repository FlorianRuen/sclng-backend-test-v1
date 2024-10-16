package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v66/github"
)

type GithubService interface {
	FetchLastHundredRepositories(ctx *gin.Context) ([]string, error)
}

type githubService struct {
	githubClient *github.Client
	config       config.Config
}

func NewGithubService(config config.Config) GithubService {
	return githubService{
		githubClient: github.NewClient(nil),
		config:       config,
	}
}

func (s githubService) FetchLastHundredRepositories(c *gin.Context) ([]string, error) {

	// TODO: test if using search instead of ListAll, we can get 100 repositories in one time
	// TODO: because listAll will require to browse multiple pages to fetch the latest 100
	// create search filter
	t := time.Now().Format(time.RFC3339)

	repos, _, err := s.githubClient.Search.Repositories(
		context.Background(),
		"created:<"+t,
		&github.SearchOptions{
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: 100,
			},
		},
	)

	// TODO: add logger for error in this block
	if err != nil {
		return []string{}, err
	}

	fmt.Println(*repos.Total)
	// JsonPrettyPrint(repos.Repositories)
	return []string{}, nil
}

func JsonPrettyPrint(data interface{}) {
	b, err := json.MarshalIndent(data, "", "  ")

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(string(b))
}
