package contentful

import "time"

// Project represents a project entry for the CMS.
type Project struct {
	Name            string   `json:"name"`
	Slug            string   `json:"slug"`
	Description     string   `json:"description"`
	LongDescription string   `json:"longDescription"`
	GithubURL       string   `json:"githubUrl"`
	Technologies    []string `json:"technologies"`
	Highlights      []string `json:"highlights"`
	Featured        bool     `json:"featured"`
	Gradient        string   `json:"gradient"`
	Category        string   `json:"category"`
	PushedAt        time.Time `json:"-"`
}

// ProjectsResult holds the fetched projects along with entry metadata
// needed for the fetch-mutate-put update pattern.
type ProjectsResult struct {
	Projects  []Project
	EntryID   string
	Version   int
	RawFields map[string]interface{}
}
