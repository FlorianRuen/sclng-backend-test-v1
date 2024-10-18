package model

type GithubRepository struct {
	ID               int64          `json:"-"` // ignored from json only used to fetch languages easily
	FullName         string         `json:"fullName"`
	Owner            string         `json:"owner"`
	Repository       string         `json:"repository"`
	Licence          *string        `json:"licence,omitempty"` // licence can be nil for some repositories without licence
	MostUsedLanguage *string         `json:"-"`
	Languages        map[string]int `json:"languages"`
}

type GithubRepositoryLanguages struct {
	RepositoryID int64
	Languages    map[string]int
}
