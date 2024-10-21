package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/Scalingo/sclng-backend-test-v1/model"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v66/github"

	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"

	"golang.org/x/time/rate"
)

type GithubService interface {
	FetchLastHundredRepositories(ctx *gin.Context, seachQuery model.SearchQuery) ([]model.GithubRepository, error)
	GetRepositoriesLanguages(repos []model.GithubRepository) ([]model.GithubRepository, error)
	FetchLanguagesForSingleRepository(r model.GithubRepository, swg *sizedwaitgroup.SizedWaitGroup, ch chan<- model.GithubRepositoryLanguages) error

	HandleRequestErrors(err error) error
}

type githubService struct {
	githubClient      *github.Client
	githubRateLimiter *rate.Limiter
	config            config.Config
}

// we have two github request with different rate limit
// but the search limit is higher, so we limit to the ListLanguages
// ListLanguages rate limit = 60 calls per hour for non-authenticated and 5000 calls for authenticated
// Search = 30 calls per minute = 1800 calls per hour
func NewGithubService(config config.Config, githubClient *github.Client, rateLimiter *rate.Limiter) GithubService {
	return githubService{
		githubClient:      githubClient,
		githubRateLimiter: rateLimiter,
		config:            config,
	}
}

func (s githubService) FetchLastHundredRepositories(c *gin.Context, seachQuery model.SearchQuery) ([]model.GithubRepository, error) {
	if !s.githubRateLimiter.Allow() {
		log.Warning("the Github rate limit has been reached. Use a token or wait until the limit reset")
		return []model.GithubRepository{}, fmt.Errorf("RATE_LIMIT_REACHED")
	}

	log.WithFields(log.Fields{
		"owner":    seachQuery.Owner,
		"licence":  seachQuery.License,
		"language": seachQuery.Language,
	}).Info("fetch last 100 repositories from github with filters")

	// search repositories that match the query filters
	// using this we can limit the number of results directly using Github search API
	// this will limit the number of loops required to filter afterwards
	repos, _, err := s.githubClient.Search.Repositories(
		context.Background(),
		seachQuery.ToGithubQuery(true),
		&github.SearchOptions{
			Sort:  "created",
			Order: "desc",
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: 100,
			},
		},
	)

	if err != nil {
		return []model.GithubRepository{}, fmt.Errorf("FETCH_ERROR")
	}

	// build output format for each repo
	repositoriesAggregated := make([]model.GithubRepository, 0)

	for _, r := range repos.Repositories {

		if r == nil || r.FullName == nil || r.Owner == nil || r.Owner.Login == nil || r.Name == nil {
			log.WithFields(log.Fields{
				"repositoryID": r.ID,
			}).Debug("repository found with invalid information. skipped")

			return []model.GithubRepository{}, fmt.Errorf("INVALID_DATA_FOUND")
		}

		repositoryAggregated := model.GithubRepository{
			ID:               *r.ID,
			FullName:         *r.FullName,
			Owner:            *r.Owner.Login,
			Repository:       *r.Name,
			MostUsedLanguage: r.Language,
		}

		// extract licence info
		// licence can be null or empty for some repositories
		if r.License != nil {
			repositoryAggregated.Licence = r.License.GetKey()
		}

		repositoriesAggregated = append(repositoriesAggregated, repositoryAggregated)
	}

	// count number of repositories where the languages are available for loading
	// if there is not enought request on rate limiter to load all of them, return an error here
	// this avoid to load the languages not completly
	reposWithLanguagesToLoad := 0

	for _, r := range repositoriesAggregated {
		if r.MostUsedLanguage != nil {
			reposWithLanguagesToLoad += 1
		}
	}

	// rate limit check: consume tokens/requests for each repo that we need to load languages from
	// if there is not enought requests, return an error to avoid loading for only a part of repositories
	if !s.githubRateLimiter.AllowN(time.Now(), reposWithLanguagesToLoad) {
		log.WithField("repositoriesToLoad", reposWithLanguagesToLoad).Warning("not enought requests in rate limiter to load languages for all repositories")
		return []model.GithubRepository{}, fmt.Errorf("RATE_LIMIT_REACHED")
	}

	log.WithFields(log.Fields{
		"numberOfRepositories": reposWithLanguagesToLoad,
	}).Debug("will load languages from all repositories found with main language available")

	// aggregate and fetch the languages used for each repo using goroutines
	repositoriesAggregated, err = s.GetRepositoriesLanguages(repositoriesAggregated)

	if err != nil {
		log.WithError(err).Error("unable to get repositories languages")
		return []model.GithubRepository{}, fmt.Errorf("FETCH_ERROR")
	}

	return repositoriesAggregated, nil
}

// getRepositoriesLanguages will fetch the languages used for each repository in parameters
// this function use wait groups to parallelize the requests for each repository
func (s githubService) GetRepositoriesLanguages(repos []model.GithubRepository) ([]model.GithubRepository, error) {

	// create a group to wait for all goroutines to finish
	swg := sizedwaitgroup.New(s.config.Tasks.MaxParallelTasksAllowed)

	// create a channel to collect response for all repositories in an map
	// the map contain the repository ID as key and languages as value
	// we will assign together when all tasks are finished
	results := make(chan model.GithubRepositoryLanguages, len(repos))

	for _, r := range repos {

		// to avoid to many requests for nothing
		// check if the main language (most used) is available for the repo
		// if yes, it means at least one language can be found using ListLanguages
		// if not, the ListLanguages willl return nil (or empty) and we can avoid executing the request
		// this will save some requests regarding to the rate limit
		if r.MostUsedLanguage == nil {
			log.WithFields(log.Fields{
				"repositoryID": r.ID,
			}).Debug("repository without most used language. skipped from loading languages list")

			results <- model.GithubRepositoryLanguages{RepositoryID: r.ID, Languages: map[string]int{}}
		} else {
			swg.Add()
			go s.FetchLanguagesForSingleRepository(r, &swg, results)
		}
	}

	// wait for all tasks to be finished
	log.Debug("waiting for all threads for loading repositories to be finished")
	swg.Wait()
	log.Debug("all threads for loading repositories languages finished")

	// close the channel
	close(results)

	// associate languages with repositories
	// I guess is better to use an array of GithubRepositoryLanguages
	// rather than chan of map directly (even if it require to create an intermediate map here)
	langMap := make(map[int64]map[string]int)
	for result := range results {
		langMap[result.RepositoryID] = result.Languages
	}

	for i := range repos {
		if lang, found := langMap[repos[i].ID]; found {
			repos[i].Languages = lang
		}
	}

	return repos, nil
}

// FetchLanguagesForSingleRepository get the languages for a specific repository
// It will add the results to a channel and use a goroutine
// note: we are not checking the rate limit in this function, because done in the parent function
// note: take care if you call this function from another function
func (s githubService) FetchLanguagesForSingleRepository(r model.GithubRepository, swg *sizedwaitgroup.SizedWaitGroup, ch chan<- model.GithubRepositoryLanguages) error {
	defer swg.Done()

	log.WithFields(log.Fields{
		"repositoryID":     r.ID,
		"mostUsedLanguage": r.MostUsedLanguage,
	}).Debug("fetch languages for repository")

	res, _, err := s.githubClient.Repositories.ListLanguages(
		context.Background(),
		r.Owner,
		r.Repository,
	)

	if err != nil {
		return s.HandleRequestErrors(err)
	}

	ch <- model.GithubRepositoryLanguages{RepositoryID: r.ID, Languages: res}
	return nil
}

// HandleRequestErrors manage errors including github rate limit errors at the same location
// If error is a rate limit error, this function will update the local rate limiter to consume all available requests
// this can help us to keep the local rate limiter up to date
func (s githubService) HandleRequestErrors(err error) error {
	if _, ok := err.(*github.RateLimitError); ok {
		if !s.githubRateLimiter.AllowN(time.Now(), s.githubRateLimiter.Burst()) {
			return fmt.Errorf("RATE_LIMITER_ERROR")
		}

		log.Warning("the Github rate limit has been reached. Use a token or wait until the limit reset")
		return fmt.Errorf("RATE_LIMIT_REACHED")
	}

	log.WithError(err).Error("error catched when fetching data from github")
	return fmt.Errorf("FETCH_ERROR")
}
