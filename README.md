# github-cms-sync

Go service that syncs GitHub repositories to the Projects section of a CMS (currently Contentful). Uses Gemini AI to generate human-readable descriptions, highlights, and metadata for each project.

## How it works

1. **Fetch** public repos from GitHub (excludes forks, archived, and profile README repos)
2. **Collect** languages and READMEs concurrently for each repo
3. **Enrich** all projects in a single Gemini AI batch request — generates name, description, technologies, highlights, category, and gradient
4. **Rank** projects by recent activity, marking the top N as featured
5. **Sync** the enriched data to Contentful via the CMA (fetch-mutate-put pattern)
6. **Log** the build result as an audit entry in Contentful

## Requirements

- Go 1.25+
- [go-service-kit](https://github.com/alberto-moreno-sa/go-service-kit) (shared SDK)

## Configuration

Copy `.env.example` to `.env` and fill in the values:

| Variable | Required | Default | Description |
|---|---|---|---|
| `GITHUB_USERNAME` | No | `alberto-moreno-sa` | GitHub username to sync repos from |
| `GITHUB_TOKEN` | No | — | GitHub PAT (increases API rate limits) |
| `CONTENTFUL_SPACE_ID` | Yes | — | Contentful space ID |
| `CONTENTFUL_CMA_TOKEN` | Yes | — | Contentful Management API token |
| `CONTENTFUL_ENTRY_ID` | Yes | — | Entry ID or sectionId for the projects section |
| `GEMINI_API_KEY` | Yes | — | Google Gemini API key |
| `MAX_FEATURED` | No | `5` | Number of projects marked as featured |
| `MAX_PROJECTS` | No | `15` | Maximum number of projects to sync |
| `FORCE_UPDATE` | No | `false` | Force update all projects |

## Usage

```bash
# Build
make build

# Run sync
make run

# Or directly
go run . sync

# Force update all projects
go run . sync --force
```

## CI/CD

- **CI** — Runs on push/PR to `main`: build, lint, test
- **Sync** — Weekly cron (Mondays 6:00 UTC) + manual dispatch with optional force update

### GitHub Actions Secrets

Set these in the repository settings:

- `CONTENTFUL_SPACE_ID`
- `CONTENTFUL_CMA_TOKEN`
- `CONTENTFUL_ENTRY_ID`
- `GEMINI_API_KEY`

`GITHUB_TOKEN` is provided automatically by GitHub Actions.

## Project Structure

```
├── cmd/
│   ├── root.go          # Cobra root command
│   └── sync.go          # Sync command + build log
├── internal/
│   ├── config/          # Environment configuration
│   ├── contentful/      # CMS client (Contentful)
│   ├── enricher/        # Gemini AI enrichment
│   ├── heuristic/       # Featured project ranking
│   ├── mapper/          # GitHub repo → internal model
│   └── syncer/          # Pipeline orchestrator
├── main.go
└── .github/workflows/
    ├── ci.yml           # Build + lint + test
    └── sync.yml         # Scheduled sync
```

## License

Private repository.
