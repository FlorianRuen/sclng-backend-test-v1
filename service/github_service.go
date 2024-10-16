package service

import (
	"context"
	"fmt"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/Scalingo/sclng-backend-test-v1/model"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v66/github"

	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
)

type GithubService interface {
	FetchLastHundredRepositories(ctx *gin.Context) ([]model.GithubRepository, error)
	GetRepositoriesLanguages(repos []model.GithubRepository) error
	FetchLanguagesForSingleRepository(r model.GithubRepository, swg *sizedwaitgroup.SizedWaitGroup, ch chan<- model.GithubRepositoryLanguages) error
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

func (s githubService) FetchLastHundredRepositories(c *gin.Context) ([]model.GithubRepository, error) {
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
			ID:           *r.ID,
			FullName:     *r.FullName,
			Owner:        *r.Owner.Login,
			Repository:   *r.Name,
			LanguagesUrl: *r.LanguagesURL,
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
	err = s.GetRepositoriesLanguages(repositoriesAggregated)

	if err != nil {
		log.WithError(err).Error("unable to get repositories languages")
		return []model.GithubRepository{}, err
	}

	return repositoriesAggregated, nil
}

// getRepositoriesLanguages will fetch the languages used for each repository in parameters
// this function use wait groups to parallelize the requests for each repository
func (s githubService) GetRepositoriesLanguages(repos []model.GithubRepository) error {

	// create a group to wait for all goroutines to finish
	swg := sizedwaitgroup.New(s.config.Tasks.MaxParallelTasksAllowed)

	// create a channel to collect response for all repositories in an map
	// the map contain the repository ID as key and languages as value
	// we will assign together when all tasks are finished
	results := make(chan model.GithubRepositoryLanguages, len(repos))

	for _, r := range repos {
		swg.Add()
		go s.FetchLanguagesForSingleRepository(r, &swg, results)
	}

	// wait for all tasks to be finished
	swg.Wait()

	// close the channel
	close(results)

	// TODO: instead of print, match the ID and build a DTO with repository details and languages
	// TODO: we will return this array for filtering in parent function
	// TODO: handle rate limiting, because to get 100 repositories, we are faciing 403 rate limiting errors from Github
	for result := range results {
		fmt.Printf("Repository ID %d utilise les languages : %v\n", result.RepositoryID, result.Languages)
	}

	return nil
}

func (s githubService) FetchLanguagesForSingleRepository(r model.GithubRepository, swg *sizedwaitgroup.SizedWaitGroup, ch chan<- model.GithubRepositoryLanguages) error {
	defer swg.Done()

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
