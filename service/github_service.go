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
	IsRateLimitReached() bool

	FetchLastHundredRepositories(ctx *gin.Context) ([]model.GithubRepository, error)
	GetRepositoriesLanguages(repos []model.GithubRepository) ([]model.GithubRepository, error)
	FetchLanguagesForSingleRepository(r model.GithubRepository, swg *sizedwaitgroup.SizedWaitGroup, ch chan<- model.GithubRepositoryLanguages) error
}

type githubService struct {
	githubClient                 *github.Client
	githubRateLimiter            *rate.Limiter
	githubRateLimiterReservation *rate.Reservation
	config                       config.Config
}

// we have two github request with different rate limit
// but the search limit is higher, so we limit to the ListLanguages
// ListLanguages rate limit = 60 calls per hour for non-authenticated and 5000 calls for authenticated
// Search = 30 calls per minute = 1800 calls per hour
func NewGithubService(config config.Config) GithubService {
	return githubService{
		githubClient:                 github.NewClient(nil),
		githubRateLimiter:            rate.NewLimiter(rate.Every(time.Hour), 60),
		githubRateLimiterReservation: nil,
		config:                       config,
	}
}

func (s githubService) IsRateLimitReached() bool {
	if s.githubRateLimiterReservation == nil || s.githubRateLimiterReservation.Delay() <= 0 {
		return false
	}

	return true
}

func (s githubService) FetchLastHundredRepositories(c *gin.Context) ([]model.GithubRepository, error) {
	if s.IsRateLimitReached() {
		return []model.GithubRepository{}, fmt.Errorf("rate limit reached")
	}

	repos, _, err := s.githubClient.Search.Repositories(
		context.Background(),
		"is:public",
		&github.SearchOptions{
			Sort: "updated",
			ListOptions: github.ListOptions{
				Page:    1,
				PerPage: 100,
			},
		},
	)

	if err != nil {
		log.WithError(err).Error("unable to get last hundred repositories")
		return []model.GithubRepository{}, err
	}

	// build output format for each repo
	repositoriesAggregated := make([]model.GithubRepository, 0)

	for _, r := range repos.Repositories {

		if r == nil || r.FullName == nil || r.Owner == nil || r.Owner.Login == nil || r.Name == nil || r.LanguagesURL == nil {
			return []model.GithubRepository{}, fmt.Errorf("invalid repository found")
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
			repositoryAggregated.Licence = r.License.Key
		}

		// TODO: for licence filtering, skip append step if current licence doesn't match the filter
		repositoriesAggregated = append(repositoriesAggregated, repositoryAggregated)
	}

	// aggregate and fetch the languages used for each repo using goroutines
	repositoriesAggregated, err = s.GetRepositoriesLanguages(repositoriesAggregated)

	if err != nil {
		log.WithError(err).Error("unable to get repositories languages")
		return []model.GithubRepository{}, err
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
		swg.Add()
		s.githubRateLimiterReservation = s.githubRateLimiter.Reserve()
		go s.FetchLanguagesForSingleRepository(r, &swg, results)
	}

	// wait for all tasks to be finished
	swg.Wait()

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

func (s githubService) FetchLanguagesForSingleRepository(r model.GithubRepository, swg *sizedwaitgroup.SizedWaitGroup, ch chan<- model.GithubRepositoryLanguages) error {
	defer swg.Done()

	if s.IsRateLimitReached() {
		return fmt.Errorf("rate limit reached")
	}

	// to avoid to many requests for nothing
	// check if the main language (most used) is available for the repo
	// if yes, it means at least one language can be found using ListLanguages
	// if not, the ListLanguages willl return nil (or empty) and we can avoid executing the request
	// this will save some requests regarding to the rate limit
	if r.MostUsedLanguage == nil {
		ch <- model.GithubRepositoryLanguages{RepositoryID: r.ID, Languages: map[string]int{}}
		return nil
	}

	s.githubRateLimiterReservation = s.githubRateLimiter.Reserve()

	res, _, err := s.githubClient.Repositories.ListLanguages(
		context.Background(),
		r.Owner,
		r.Repository,
	)

	if err != nil {
		log.WithError(err).Error("unable to get repositories languages")
		return err
	}

	ch <- model.GithubRepositoryLanguages{RepositoryID: r.ID, Languages: res}
	return nil
}
