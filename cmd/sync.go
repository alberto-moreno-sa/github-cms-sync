package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	servicekit "github.com/alberto-moreno-sa/go-service-kit/contentful"
	githubapi "github.com/alberto-moreno-sa/go-service-kit/github"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/config"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/contentful"
	"github.com/alberto-moreno-sa/github-cms-sync/internal/syncer"
	"github.com/spf13/cobra"
)

var forceFlag bool

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync GitHub projects to Contentful",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}

		if forceFlag {
			cfg.ForceUpdate = true
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Initialize clients
		ghClient := githubapi.NewClient(cfg.GitHubToken)
		cmaClient := contentful.NewClient(cfg.SpaceID, cfg.CMAToken)

		// Run sync
		s := syncer.New(cfg, ghClient, cmaClient)
		stats, err := s.Run(ctx)
		if err != nil {
			return fmt.Errorf("sync: %w", err)
		}

		log.Printf("Sync complete: %d projects (%d new)", stats.Total, stats.NewAdded)

		// Record build log (non-fatal)
		recordBuildLog(ctx, cmaClient, cfg, stats)

		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&forceFlag, "force", false, "Force update all projects")
	rootCmd.AddCommand(syncCmd)
}

func recordBuildLog(ctx context.Context, cmaClient *contentful.Client, cfg *config.Config, stats *syncer.SyncStats) {
	log.Println("Recording build log...")

	const serviceName = "github-cms-sync"
	triggeredBy := "local"
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		triggeredBy = "github-actions"
	}

	logEntry := servicekit.BuildLogEntry{
		Service:         serviceName,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		TriggeredBy:     triggeredBy,
		ForceUpdate:     cfg.ForceUpdate,
		TranslationUsed: false,
		NewAdded:        stats.NewAdded,
		TotalAfterSync:  stats.Total,
		Status:          stats.Status,
	}

	buildLogResult, err := cmaClient.GetBuildLog(ctx)
	if err != nil {
		log.Printf("WARNING: failed to fetch build log: %v", err)
		return
	}

	var ownEntries, otherEntries []servicekit.BuildLogEntry
	for _, e := range buildLogResult.Entries {
		if e.Service == serviceName {
			ownEntries = append(ownEntries, e)
		} else {
			otherEntries = append(otherEntries, e)
		}
	}
	if len(ownEntries) >= 3 {
		ownEntries = ownEntries[len(ownEntries)-2:]
	}
	allLogEntries := append(otherEntries, append(ownEntries, logEntry)...)

	var buildLogEntryID string
	var buildLogVersion int

	if buildLogResult.EntryID == "" {
		buildLogEntryID, buildLogVersion, err = cmaClient.CreateBuildLog(ctx, allLogEntries)
		if err != nil {
			log.Printf("WARNING: failed to create build log: %v", err)
			return
		}
	} else {
		buildLogEntryID = buildLogResult.EntryID
		buildLogVersion, err = cmaClient.UpdateBuildLog(ctx, buildLogResult, allLogEntries)
		if err != nil {
			log.Printf("WARNING: failed to update build log: %v", err)
			return
		}
	}

	if err := cmaClient.PublishEntry(ctx, buildLogEntryID, buildLogVersion); err != nil {
		log.Printf("WARNING: failed to publish build log: %v", err)
		return
	}

	log.Printf("Build log updated (%d total entries)", len(allLogEntries))
}
