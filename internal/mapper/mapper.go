package mapper

import (
	"sort"
	"strings"
	"time"

	"github.com/alberto-moreno-sa/go-service-kit/github"
)

// RawProject holds the raw data from GitHub before AI enrichment.
type RawProject struct {
	Name      string
	Slug      string
	GitHubURL string
	LiveURL   string
	Languages []string
	ReadmeRaw string
	RepoSize  int
	PushedAt  time.Time
}

// ToRawProject converts a GitHub repo with its languages and README into a RawProject.
func ToRawProject(repo github.Repo, languages map[string]int, readme string) RawProject {
	var liveURL string
	if repo.Homepage != nil {
		liveURL = *repo.Homepage
	}

	return RawProject{
		Name:      repo.Name,
		Slug:      repo.Name,
		GitHubURL: repo.HTMLURL,
		LiveURL:   liveURL,
		Languages: sortedLanguages(languages),
		ReadmeRaw: readme,
		RepoSize:  repo.Size,
		PushedAt:  repo.PushedAt,
	}
}

// FilterRepos removes forks, archived repos, and the profile README repo.
func FilterRepos(repos []github.Repo, username string) []github.Repo {
	profileRepo := strings.ToLower(username)
	var filtered []github.Repo
	for _, r := range repos {
		if r.Fork || r.Archived {
			continue
		}
		if strings.ToLower(r.Name) == profileRepo {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// sortedLanguages returns language names sorted by byte count descending.
func sortedLanguages(languages map[string]int) []string {
	type langCount struct {
		name  string
		bytes int
	}
	var langs []langCount
	for name, bytes := range languages {
		langs = append(langs, langCount{name, bytes})
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].bytes > langs[j].bytes
	})
	result := make([]string, len(langs))
	for i, l := range langs {
		result[i] = l.name
	}
	return result
}
