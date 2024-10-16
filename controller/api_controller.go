package controller

import (
	"net/http"

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
	repos, err := s.githubService.FetchLastHundredRepositories(c)

	// TODO: return with uniformized error format, maybe using library or custom func ?
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, repos)
}
