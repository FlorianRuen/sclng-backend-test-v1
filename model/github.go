package model

type GithubRepository struct {
	ID               int64          `json:"-"` // ignored from json only used to fetch languages easily
	FullName         string         `json:"fullName"`
	Owner            string         `json:"owner"`
	Repository       string         `json:"repository"`
	License          string         `json:"license"` // license can be nil, will contains empty string
	MostUsedLanguage *string        `json:"-"`
	Languages        map[string]int `json:"languages"`
}

type GithubRepositoryLanguages struct {
	RepositoryID int64
	Languages    map[string]int
}
