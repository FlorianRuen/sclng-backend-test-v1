package controller

import (
	"fmt"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/Scalingo/sclng-backend-test-v1/service"
	"github.com/gin-gonic/gin"
)

type APIController interface {
	GetRepositories(ctx *gin.Context)
}

type apiController struct {
	githubService service.GithubService
	config        config.Config
}

func NewApiController(config config.Config, service service.GithubService) APIController {
	return apiController{
		githubService: service,
		config:        config,
	}
}

func (s apiController) GetRepositories(c *gin.Context) {
	fmt.Println("GetRepositories")
	repos, err := s.githubService.FetchLastHundredRepositories(c)

	fmt.Println(repos)
	fmt.Println(err)
}
