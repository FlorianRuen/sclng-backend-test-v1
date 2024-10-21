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
	log "github.com/sirupsen/logrus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.WithError(err).Error("unable to load configuration")
	}

	// configure logger
	logger.Setup(*cfg)

	// setup handlers and services
	githubService := service.NewGithubService(*cfg)
	apiController := controller.NewApiController(*cfg, githubService)

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
