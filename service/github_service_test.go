package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/Scalingo/sclng-backend-test-v1/config"
	"github.com/Scalingo/sclng-backend-test-v1/model"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v66/github"
	githubMock "github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/remeh/sizedwaitgroup"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

// TestFetchLastHundredRepositories will test function FetchLastHundredRepositories
func TestFetchLastHundredRepositories(t *testing.T) {
	tests := []struct {
		name                     string
		searchQuery              model.SearchQuery
		mockResponseRepositories github.RepositoriesSearchResult
		mockResponseLanguages    map[string]int
		rateLimit                int
		expectedRepos            []model.GithubRepository
		expectError              bool
		expectedErrMsg           string
	}{
		{
			name:        "Single repository without search",
			rateLimit:   60,
			searchQuery: model.SearchQuery{},
			mockResponseRepositories: github.RepositoriesSearchResult{
				Repositories: []*github.Repository{
					{
						ID:       github.Int64(1),
						FullName: github.String("test/repo1"),
						Owner:    &github.User{Login: github.String("test-owner")},
						Name:     github.String("repo1"),
						Language: github.String("Go"),
					},
				},
			},
			mockResponseLanguages: map[string]int{
				"Go": 10,
			},
			expectedRepos: []model.GithubRepository{
				{
					ID:               1,
					FullName:         "test/repo1",
					Owner:            "test-owner",
					Repository:       "repo1",
					MostUsedLanguage: github.String("Go"),
					Languages: map[string]int{
						"Go": 10,
					},
				},
			},
			expectError: false,
		},
		{
			name:      "Multiple repository search by language",
			rateLimit: 60,
			searchQuery: model.SearchQuery{
				Language: "Java",
			},
			mockResponseRepositories: github.RepositoriesSearchResult{
				Repositories: []*github.Repository{
					{
						ID:       github.Int64(2),
						FullName: github.String("Owner2/repo2"),
						Owner:    &github.User{Login: github.String("Owner2")},
						Name:     github.String("repo2"),
						Language: github.String("Java"),
					},
				},
			},
			mockResponseLanguages: map[string]int{
				"Java": 200,
			},
			expectedRepos: []model.GithubRepository{
				{
					ID:               2,
					FullName:         "Owner2/repo2",
					Owner:            "Owner2",
					Repository:       "repo2",
					MostUsedLanguage: github.String("Java"),
					Languages: map[string]int{
						"Java": 200,
					},
				},
			},
			expectError: false,
		},
		{
			name:        "Invalid data for specific repository",
			rateLimit:   60,
			searchQuery: model.SearchQuery{},
			mockResponseRepositories: github.RepositoriesSearchResult{
				Repositories: []*github.Repository{
					{
						ID:       github.Int64(2),
						FullName: github.String("Owner2/repo2"),
						Name:     github.String("repo2"),
						Language: github.String("Java"),
					},
					{
						ID:       github.Int64(2),
						FullName: github.String("Owner2/repo2"),
						Name:     github.String("repo2"),
						Language: github.String("Java"),
					},
				},
			},
			expectedRepos:  []model.GithubRepository{},
			expectError:    true,
			expectedErrMsg: "INVALID_DATA_FOUND",
		},
		{
			name:        "Two repositories with rate limit",
			rateLimit:   1,
			searchQuery: model.SearchQuery{},
			mockResponseRepositories: github.RepositoriesSearchResult{
				Repositories: []*github.Repository{
					{
						ID:       github.Int64(1),
						FullName: github.String("test/repo1"),
						Owner:    &github.User{Login: github.String("test-owner")},
						Name:     github.String("repo1"),
						Language: github.String("Go"),
					},
					{
						ID:       github.Int64(2),
						FullName: github.String("Owner2/repo2"),
						Owner:    &github.User{Login: github.String("Owner2")},
						Name:     github.String("repo2"),
						Language: github.String("Java"),
					},
				},
			},
			expectedRepos:  []model.GithubRepository{},
			expectError:    true,
			expectedErrMsg: "RATE_LIMIT_REACHED",
		},
	}

	// execute tests
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			mockedHTTPClient := githubMock.NewMockedHTTPClient(
				githubMock.WithRequestMatchHandler(
					githubMock.GetSearchRepositories,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						_, err := w.Write(githubMock.MustMarshal(tt.mockResponseRepositories))

						if err != nil {
							t.Error("unable to configure mock http client")
						}
					}),
				),
				githubMock.WithRequestMatchHandler(
					githubMock.GetReposLanguagesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						_, err := w.Write(githubMock.MustMarshal(tt.mockResponseLanguages))

						if err != nil {
							t.Error("unable to configure mock http client")
						}
					}),
				),
			)

			// setup github service using default config and mocked client
			mockedRateLimiter := rate.NewLimiter(rate.Every(time.Hour), tt.rateLimit)
			mockedGithubClient := github.NewClient(mockedHTTPClient)
			conf := config.GetDefault()
			svc := NewGithubService(*conf, mockedGithubClient, mockedRateLimiter)

			// Prepare the context and search query
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(nil)
			repos, err := svc.FetchLastHundredRepositories(ctx, tt.searchQuery)

			if tt.expectError {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedRepos, repos)
		})
	}
}

// TestFetchLanguagesForSingleRepository test the function called FetchLanguagesForSingleRepository
func TestFetchLanguagesForSingleRepository(t *testing.T) {
	tests := []struct {
		name           string
		repo           model.GithubRepository
		mockResponse   map[string]int
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "Fetch languages successfully",
			repo: model.GithubRepository{
				ID:         1,
				Owner:      "Owner1",
				Repository: "Repo1",
			},
			mockResponse: map[string]int{
				"Go":     10000,
				"Python": 5000,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHTTPClient := githubMock.NewMockedHTTPClient(
				githubMock.WithRequestMatchHandler(
					githubMock.GetReposLanguagesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						_, err := w.Write(githubMock.MustMarshal(tt.mockResponse))

						if err != nil {
							t.Error("unable to configure mock http client")
						}
					}),
				),
			)

			mockedRateLimiter := rate.NewLimiter(rate.Every(time.Hour), 60)
			mockedGithubClient := github.NewClient(mockedHTTPClient)
			conf := config.GetDefault()
			svc := NewGithubService(*conf, mockedGithubClient, mockedRateLimiter)

			// Prepare wait group and channel
			swg := sizedwaitgroup.New(1)
			ch := make(chan model.GithubRepositoryLanguages, 1)

			// execute the function
			swg.Add()
			err := svc.FetchLanguagesForSingleRepository(tt.repo, &swg, ch)

			if tt.expectError {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)

				// check that the expected result was sent to the channel
				langResult := <-ch
				assert.Equal(t, tt.repo.ID, langResult.RepositoryID)
				assert.Equal(t, tt.mockResponse, langResult.Languages)
			}
		})
	}
}

// TestGetRepositoriesLanguages test function called GetRepositoriesLanguages
func TestGetRepositoriesLanguages(t *testing.T) {
	tests := []struct {
		name                        string
		repos                       []model.GithubRepository
		mockGithubResponseLanguages map[string]int
		mockResponses               map[int64]map[string]int
		expectedLanguages           map[int64]map[string]int
	}{
		{
			name: "Fetch languages successfully for multiple repositories",
			repos: []model.GithubRepository{
				{ID: 1, Owner: "owner1", Repository: "repo1", MostUsedLanguage: github.String("Go")},
			},
			mockGithubResponseLanguages: map[string]int{
				"Go":   10000,
				"HTML": 500,
			},
			mockResponses: map[int64]map[string]int{
				1: {"Go": 10000, "HTML": 500},
			},
			expectedLanguages: map[int64]map[string]int{
				1: {"Go": 10000, "HTML": 500},
			},
		},
		{
			name: "Some repositories don't have a most used language",
			repos: []model.GithubRepository{
				{ID: 1, Owner: "owner1", Repository: "repo1", MostUsedLanguage: github.String("Go")},
				{ID: 2, Owner: "owner2", Repository: "repo2", MostUsedLanguage: nil},
			},
			mockGithubResponseLanguages: map[string]int{
				"Go":   10000,
				"HTML": 500,
			},
			mockResponses: map[int64]map[string]int{
				1: {"Go": 10000, "HTML": 500},
			},
			expectedLanguages: map[int64]map[string]int{
				1: {"Go": 10000, "HTML": 500},
				2: {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockedHTTPClient := githubMock.NewMockedHTTPClient(
				githubMock.WithRequestMatchHandler(
					githubMock.GetReposLanguagesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						_, err := w.Write(githubMock.MustMarshal(tt.mockGithubResponseLanguages))

						if err != nil {
							t.Error("unable to configure mock http client")
						}
					}),
				),
			)

			mockedRateLimiter := rate.NewLimiter(rate.Every(time.Hour), 60)
			mockedGithubClient := github.NewClient(mockedHTTPClient)
			conf := config.GetDefault()
			svc := NewGithubService(*conf, mockedGithubClient, mockedRateLimiter)

			// Call the GetRepositoriesLanguages function
			repos, err := svc.GetRepositoriesLanguages(tt.repos)

			assert.NoError(t, err)

			// validate that the expected languages were correctly assigned to each repository
			for _, repo := range repos {
				expectedLanguages, ok := tt.expectedLanguages[repo.ID]
				if ok {
					assert.Equal(t, expectedLanguages, repo.Languages)
				}
			}
		})
	}
}
