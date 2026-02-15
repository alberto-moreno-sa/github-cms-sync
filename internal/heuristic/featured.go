package heuristic

import (
	"sort"

	"github.com/alberto-moreno-sa/github-cms-sync/internal/contentful"
)

// ApplyFeatured sorts projects by PushedAt descending, marks the top maxFeatured
// as featured, the next ones as not featured, and discards the rest beyond maxTotal.
func ApplyFeatured(projects []contentful.Project, maxFeatured, maxTotal int) []contentful.Project {
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].PushedAt.After(projects[j].PushedAt)
	})

	if len(projects) > maxTotal {
		projects = projects[:maxTotal]
	}

	for i := range projects {
		projects[i].Featured = i < maxFeatured
	}

	return projects
}
