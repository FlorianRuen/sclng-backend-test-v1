package service

import (
	"fmt"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/gin-gonic/gin"
)

type GithubService interface {
	FetchLastHundredRepositories(ctx *gin.Context) ([]string, error)
}

type githubService struct {
	config config.Config
}

func NewGithubService(config config.Config) GithubService {
	return githubService{
		config: config,
	}
}

func (s githubService) FetchLastHundredRepositories(c *gin.Context) ([]string, error) {
	fmt.Println("FetchLastHundredRepositories")
	return []string{}, nil
}
