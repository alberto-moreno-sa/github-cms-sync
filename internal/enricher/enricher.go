package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alberto-moreno-sa/go-service-kit/gemini"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/contentful"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/mapper"
)

const maxReadmeChars = 1500

const systemPrompt = `You are a technical content writer for a software engineer's portfolio website.
You will receive a JSON array of GitHub repositories with their metadata.
For EACH repository, generate a JSON object with:

1. "name": a human-readable project name derived from the repo name (e.g. "financial-dashboard" → "Financial Dashboard", "go-service-kit" → "Go Service Kit", "alberthiggs.com" → "alberthiggs.com")
2. "shortDescription": 1 brief phrase, max 200 chars. A concise summary of what the project is.
3. "description": 1 sentence, max 120 chars. What the project does.
4. "longDescription": 2-3 sentences. What it does, key technical decisions, and impact.
4. "technologies": array of specific technologies (frameworks, libraries, databases).
   Use the languages list AND the README to identify: React, FastAPI, PostgreSQL, Docker, etc.
   Do NOT list generic terms like "JavaScript" if a framework like "React" is more specific.
6. "highlights": array of 3-5 bullet points. Focus on technical achievements, not features.
   Each highlight should be concise (under 60 chars).
7. "category": one of ["Web", "Backend", "Full-Stack", "Libraries", "DevOps", "Game Dev", "Mobile"]
8. "gradient": a Tailwind CSS gradient string (from-{color}-500 to-{color}-600).
   Choose colors that match the project's domain:
   - Finance/money → emerald/teal
   - Infrastructure/DevOps → cyan/blue
   - Frontend/UI → purple/indigo
   - Data/Analytics → amber/orange
   - Games → red/rose
   - Libraries/Tools → slate/gray

Return ONLY a valid JSON array with one object per repository. No markdown, no explanation.`

type enrichedData struct {
	Name             string   `json:"name"`
	ShortDescription string   `json:"shortDescription"`
	Description      string   `json:"description"`
	LongDescription  string   `json:"longDescription"`
	Technologies    []string `json:"technologies"`
	Highlights      []string `json:"highlights"`
	Category        string   `json:"category"`
	Gradient        string   `json:"gradient"`
}

const (
	maxRetries = 3
	retryDelay = 20 * time.Second
)

// Enrich sends all projects to Gemini in a single request and returns enriched projects.
func Enrich(ctx context.Context, apiKey string, projects []mapper.RawProject) ([]contentful.Project, error) {
	log.Printf("  Sending %d projects to Gemini in a single batch...", len(projects))

	userPrompt := buildBatchPrompt(projects)

	var response string
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := retryDelay * time.Duration(attempt)
			log.Printf("  Retry %d/%d (waiting %s)...", attempt, maxRetries, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		var err error
		response, err = gemini.GenerateContent(ctx, apiKey, systemPrompt, userPrompt)
		if err == nil {
			break
		}

		lastErr = err
		if !strings.Contains(err.Error(), "429") && !strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
			return nil, fmt.Errorf("gemini: %w", err)
		}
		log.Printf("  Rate limited, will retry...")
	}

	if response == "" && lastErr != nil {
		return nil, fmt.Errorf("gemini after %d retries: %w", maxRetries, lastErr)
	}

	response = stripMarkdownFences(response)

	var dataList []enrichedData
	if err := json.Unmarshal([]byte(response), &dataList); err != nil {
		return nil, fmt.Errorf("parse gemini response: %w", err)
	}

	// Match by position: Gemini returns items in the same order as the input
	var result []contentful.Project
	for i, raw := range projects {
		if i >= len(dataList) {
			log.Printf("WARNING: Gemini did not return data for %s, skipping", raw.Name)
			continue
		}
		data := dataList[i]
		result = append(result, contentful.Project{
			Name:             data.Name,
			Slug:             raw.Slug,
			ShortDescription: data.ShortDescription,
			Description:      data.Description,
			LongDescription:  data.LongDescription,
			GithubURL:       raw.GitHubURL,
			Technologies:    data.Technologies,
			Highlights:      data.Highlights,
			Featured:        false,
			Gradient:        data.Gradient,
			Category:        data.Category,
			PushedAt:        raw.PushedAt,
		})
	}

	log.Printf("  Gemini returned data for %d/%d projects", len(result), len(projects))
	return result, nil
}

func buildBatchPrompt(projects []mapper.RawProject) string {
	type repoEntry struct {
		Name      string `json:"name"`
		Languages string `json:"languages"`
		Readme    string `json:"readme"`
	}

	entries := make([]repoEntry, len(projects))
	for i, p := range projects {
		readme := p.ReadmeRaw
		if len(readme) > maxReadmeChars {
			readme = readme[:maxReadmeChars]
		}
		entries[i] = repoEntry{
			Name:      p.Name,
			Languages: strings.Join(p.Languages, ", "),
			Readme:    readme,
		}
	}

	jsonBytes, err := json.Marshal(entries)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
