package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/Scalingo/sclng-backend-test-v1/controller"
	"github.com/Scalingo/sclng-backend-test-v1/logger"
	"github.com/Scalingo/sclng-backend-test-v1/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v66/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.WithError(err).Error("unable to load configuration")
	}

	// configure logger
	logger.Setup(*cfg)

	// setup github client
	// we do here and pass the client to Github service to easily improve tests with mock client
	githubClient := github.NewClient(nil)

	if cfg.Github.Token != "" {
		log.Debug("will setup github client with authorization token")
		githubClient = githubClient.WithAuthToken(cfg.Github.Token)
	}

	// setup local rate limiter
	// execute first request to github to fetch current rate limits
	log.Debug("loading current rate limit from github")
	rateLimits, _, err := githubClient.RateLimit.Get(context.Background())
	if err != nil {
		log.WithError(err).Panic("unable to load current github rate limits")
	}

	log.WithFields(log.Fields{
		"totalAvailable":    rateLimits.Core.Limit,
		"remainingRequests": rateLimits.Core.Remaining,
	}).Debug("will setup local rate limiter with rate limits infos from github")

	// setup rate limiter
	// consume X tokens according to the number of remaining tokens
	// this help us to have a right rate limiter even if external requests are made
	rateLimiter := rate.NewLimiter(rate.Every(time.Hour), rateLimits.Core.Limit)

	if !rateLimiter.AllowN(time.Now(), rateLimits.Core.Limit-rateLimits.Core.Remaining) {
		log.WithError(err).Panic("unable to configure the github rate limiter")
	}

	// setup handlers and services
	githubService := service.NewGithubService(*cfg, githubClient, rateLimiter)
	apiController := controller.NewAPIController(*cfg, githubService)

	// setup server and define all routes
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	server := &http.Server{
		Addr:    ":" + cfg.API.ListenPort,
		Handler: router,
	}

	router.Use(
		cors.New(cors.Config{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET"},
			AllowHeaders: []string{"Content-Type, Content-Length, Accept-Encoding, Host, accept, Origin, Cache-Control, X-Requested-With"},
			MaxAge:       12 * time.Hour,
		}),
	)

	api := router.Group("")
	{
		api.GET("/repos", apiController.GetRepositories)
	}

	// start with configuration
	go func() {
		log.Info("server listening on port " + cfg.API.ListenPort)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Error("error while starting server")
		}

	}()

	// create context with 15 seconds timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// wait for interrupt signal to gracefully shut down the server with a timeout of 15 seconds.
	// context is used to inform the server it has 5 seconds to finish the request it is currently handling
	// kill default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Do some actions here : close DB connections, ...
	log.Info("SIGINT, SIGTERM received, will shut down server ...")

	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Server forced to shutdown")
	} else {
		log.Info("Application stopped gracefully !")
	}
}
