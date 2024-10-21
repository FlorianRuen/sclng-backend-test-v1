package model

import "strings"

type SearchQuery struct {
	Owner    string `form:"owner"`
	License  string `form:"license"`
	Language string `form:"language"`
}

func (params SearchQuery) ToGithubQuery(filterPublicRepositories bool) string {
	var githubQuery strings.Builder

	if filterPublicRepositories {
		githubQuery.WriteString("is:public ")
	}

	if params.Owner != "" {
		githubQuery.WriteString("owner:" + params.Owner + " ")
	}

	if params.License != "" {
		githubQuery.WriteString("license:" + params.License + " ")
	}

	if params.Language != "" {
		githubQuery.WriteString("language:" + params.Language + " ")
	}

	return strings.TrimSpace(githubQuery.String())
}
