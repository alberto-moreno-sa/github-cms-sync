package syncer

import (
	"context"
	"fmt"
	"log"
	"sync"

	githubapi "github.com/alberto-moreno-sa/go-service-kit/github"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/config"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/contentful"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/enricher"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/heuristic"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/mapper"
)

// SyncStats holds the results of a sync run.
type SyncStats struct {
	NewAdded int
	Total    int
	Status   string
}

// Syncer orchestrates the GitHub → CMS sync pipeline.
type Syncer struct {
	cfg    *config.Config
	github *githubapi.Client
	cma    *contentful.Client
}

// New creates a new Syncer.
func New(cfg *config.Config, gh *githubapi.Client, cma *contentful.Client) *Syncer {
	return &Syncer{
		cfg:    cfg,
		github: gh,
		cma:    cma,
	}
}

// Run executes the full sync pipeline.
func (s *Syncer) Run(ctx context.Context) (*SyncStats, error) {
	// 1. Fetch repos
	log.Println("Fetching GitHub repositories...")
	repos, err := s.github.ListRepos(ctx, s.cfg.GitHubUsername)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	log.Printf("Found %d public repos", len(repos))

	// 2. Filter
	filtered := mapper.FilterRepos(repos, s.cfg.GitHubUsername)
	log.Printf("After filtering: %d repos", len(filtered))

	if len(filtered) == 0 {
		return &SyncStats{Status: "success"}, nil
	}

	// 3. Fetch details concurrently
	log.Println("Fetching repo details (languages, READMEs)...")
	rawProjects, err := s.fetchDetails(ctx, filtered)
	if err != nil {
		return nil, fmt.Errorf("fetch details: %w", err)
	}

	// 4. Enrich with Gemini
	log.Println("Enriching projects with Gemini AI...")
	enriched, err := enricher.Enrich(ctx, s.cfg.GeminiAPIKey, rawProjects)
	if err != nil {
		return nil, fmt.Errorf("enrich: %w", err)
	}
	log.Printf("Enriched %d projects", len(enriched))

	// 5. Apply featured heuristic
	projects := heuristic.ApplyFeatured(enriched, s.cfg.MaxFeatured, s.cfg.MaxProjects)
	log.Printf("Final selection: %d projects (%d featured)", len(projects), s.cfg.MaxFeatured)

	// 6. Fetch current state from Contentful
	log.Println("Fetching current projects from Contentful...")
	result, err := s.cma.GetProjects(ctx, s.cfg.EntryID)
	if err != nil {
		return nil, fmt.Errorf("get projects: %w", err)
	}

	// 7. Update Contentful
	log.Println("Updating projects in Contentful...")
	newVersion, err := s.cma.UpdateProjects(ctx, result, projects)
	if err != nil {
		return nil, fmt.Errorf("update projects: %w", err)
	}

	// 8. Publish (use the real entry ID from Contentful, not the config value)
	if err := s.cma.PublishEntry(ctx, result.EntryID, newVersion); err != nil {
		return nil, fmt.Errorf("publish: %w", err)
	}

	log.Println("Successfully synced and published.")

	return &SyncStats{
		NewAdded: len(projects) - len(result.Projects),
		Total:    len(projects),
		Status:   "success",
	}, nil
}

func (s *Syncer) fetchDetails(ctx context.Context, repos []githubapi.Repo) ([]mapper.RawProject, error) {
	var (
		mu          sync.Mutex
		wg          sync.WaitGroup
		sem         = make(chan struct{}, 5)
		rawProjects []mapper.RawProject
		firstErr    error
	)

	for _, repo := range repos {
		wg.Add(1)
		go func(r githubapi.Repo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			languages, err := s.github.GetRepoLanguages(ctx, s.cfg.GitHubUsername, r.Name)
			if err != nil {
				log.Printf("WARNING: languages failed for %s: %v", r.Name, err)
				languages = map[string]int{}
			}

			readme, err := s.github.GetRepoREADME(ctx, s.cfg.GitHubUsername, r.Name)
			if err != nil {
				log.Printf("WARNING: readme failed for %s: %v", r.Name, err)
			}

			raw := mapper.ToRawProject(r, languages, readme)

			mu.Lock()
			rawProjects = append(rawProjects, raw)
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	return rawProjects, nil
}
