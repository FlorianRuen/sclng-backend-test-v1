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

// NewGithubService will create an instance of GithubService
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

	// Search repositories that match the specified query filters.
	// By applying filters directly in the GitHub Search API, we can reduce the
	// number of results returned, minimizing the need for additional filtering
	// and processing after retrieval. This optimizes performance and reduces unnecessary iterations.
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

	// Construct the output format for each repository.
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

		// Extract license information.
		// The license field can be null or empty for some repositories,
		if r.License != nil {
			repositoryAggregated.License = r.License.GetKey()
		}

		repositoriesAggregated = append(repositoriesAggregated, repositoryAggregated)
	}

	// Count the number of repositories that have languages available for loading.
	// If the rate limiter doesn't have enough available requests to load all languages,
	// return an error to prevent partially loading the data. This ensures that
	// language data is either fully loaded or not loaded at all, maintaining consistency.
	reposWithLanguagesToLoad := 0

	for _, r := range repositoriesAggregated {
		if r.MostUsedLanguage != nil {
			reposWithLanguagesToLoad += 1
		}
	}

	// Rate limit check: consume tokens for each repository that requires language loading.
	// If there are not enough available requests, return an error to prevent
	// loading data for only a subset of repositories.
	if !s.githubRateLimiter.AllowN(time.Now(), reposWithLanguagesToLoad) {
		log.WithField("repositoriesToLoad", reposWithLanguagesToLoad).Warning("not enought requests in rate limiter to load languages for all repositories")
		return []model.GithubRepository{}, fmt.Errorf("RATE_LIMIT_REACHED")
	}

	log.WithFields(log.Fields{
		"numberOfRepositories": reposWithLanguagesToLoad,
	}).Debug("will load languages from all repositories found with main language available")

	// Aggregate and fetch the languages used in each repository concurrently using goroutines.
	repositoriesAggregated, err = s.GetRepositoriesLanguages(repositoriesAggregated)

	if err != nil {
		log.WithError(err).Error("unable to get repositories languages")
		return []model.GithubRepository{}, fmt.Errorf("FETCH_ERROR")
	}

	return repositoriesAggregated, nil
}

// GetRepositoriesLanguages fetches the languages used by each repository provided in the input parameters.
// This function employs wait groups to parallelize API requests for each repository,
func (s githubService) GetRepositoriesLanguages(repos []model.GithubRepository) ([]model.GithubRepository, error) {
	swg := sizedwaitgroup.New(s.config.Tasks.MaxParallelTasksAllowed)

	// Create a channel to collect responses from all repositories.
	// The responses will be stored in a map with repository IDs as keys and their corresponding languages as values.
	// This map will be populated once all concurrent tasks have completed.
	results := make(chan model.GithubRepositoryLanguages, len(repos))

	for _, r := range repos {

		// To prevent unnecessary API requests, check if the main language (most used) is available for the repository.
		// If a main language is present, it indicates that at least one language can be retrieved using ListLanguages.
		// If not, calling ListLanguages will return nil or an empty result, allowing us to skip the request
		if r.MostUsedLanguage == nil {
			log.WithFields(log.Fields{
				"repositoryID": r.ID,
			}).Debug("repository without most used language. skipped from loading languages list")

			results <- model.GithubRepositoryLanguages{RepositoryID: r.ID, Languages: map[string]int{}}
		} else {
			swg.Add()

			go func(repo model.GithubRepository) {
				defer swg.Done()
				err := s.FetchLanguagesForSingleRepository(repo, &swg, results)
				if err != nil {
					log.WithFields(log.Fields{
						"repositoryID": repo.ID,
					}).WithError(err).Error("unable to fetch languages for specific repository")
				}
			}(r)
		}
	}

	// Wait for all tasks to be finished
	log.Debug("waiting for all threads for loading repositories to be finished")
	swg.Wait()
	log.Debug("all threads for loading repositories languages finished")

	// Close the channel
	close(results)

	// It is preferable to use an array instead of directly using a channel of maps.
	// Although this approach requires creating an intermediate map, it provides a clearer and more structured representation
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

// FetchLanguagesForSingleRepository retrieves the languages for a specific repository.
// The results are sent to a channel and processed in a separate goroutine.
// Note: Rate limiting is not checked within this function, as it is handled in the parent function.
func (s githubService) FetchLanguagesForSingleRepository(r model.GithubRepository, swg *sizedwaitgroup.SizedWaitGroup, ch chan<- model.GithubRepositoryLanguages) error {
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

// HandleRequestErrors manages various errors, including GitHub rate limit errors
// If a rate limit error occurs, this function updates the local rate limiter to consume all available requests,
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
