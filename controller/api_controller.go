package controller

import (
	"net/http"
	"strings"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/Scalingo/sclng-backend-test-v1/model"
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

func NewAPIController(config config.Config, service service.GithubService) APIController {
	return apiController{
		githubService: service,
		config:        config,
	}
}

func (s apiController) GetRepositories(c *gin.Context) {
	var searchQuery model.SearchQuery
	if err := c.ShouldBindQuery(&searchQuery); err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	// execute the request
	repos, err := s.githubService.FetchLastHundredRepositories(c, searchQuery)
	if err != nil {
		if strings.Contains(err.Error(), "RATE_LIMIT_REACHED") {
			c.JSON(http.StatusTooManyRequests, model.NewAPIError(err))
			return
		}

		c.JSON(http.StatusInternalServerError, model.NewAPIError(err))
		return
	}

	c.JSON(http.StatusOK, repos)
}
