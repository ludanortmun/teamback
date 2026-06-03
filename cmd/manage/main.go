package main

import (
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/ludanortmun/teamback/internal/ai"
	"github.com/ludanortmun/teamback/internal/config"
	"github.com/ludanortmun/teamback/internal/core"
	"github.com/ludanortmun/teamback/internal/database"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "Teamback management CLI"
	app.Usage = "Management tool for Teamback app"
	app.UsageText = "Teamback management CLI [global options]"

	app.Commands = []cli.Command{
		{
			Name:      "users",
			Aliases:   []string{"u"},
			Usage:     "Manage users",
			UsageText: `manage users`,
			Subcommands: []cli.Command{
				{
					Name:  "add",
					Usage: "add a new user",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:     "name, n",
							Usage:    "user name",
							Required: true,
						},
						cli.StringFlag{
							Name:     "email, e",
							Usage:    "user email",
							Required: true,
						},
						cli.BoolFlag{
							Name:  "teacher, t",
							Usage: "add a teacher user",
						},
					},
					Action: runAddUser,
				},
			},
		},
		{
			Name:    "summaries",
			Aliases: []string{"s"},
			Usage:   "Manage AI summaries",
			Subcommands: []cli.Command{
				{
					Name:  "retrigger",
					Usage: "re-trigger AI summary generation for an assignment",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:     "assignment-id, a",
							Usage:    "assignment ID to re-generate summary for",
							Required: true,
						},
					},
					Action: runRetriggerSummary,
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func runAddUser(c *cli.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	teambackDb := database.NewTeambackDatabase(db)

	id := uuid.NewString()
	name := c.String("name")
	email := c.String("email")
	teacher := c.Bool("teacher")
	role := core.RoleStudent

	if teacher {
		log.Printf("adding teacher user %s (%s)", name, email)
		role = core.RoleTeacher
	} else {
		log.Printf("adding user %s (%s)", name, email)
	}

	user := core.User{
		ID:    id,
		Name:  name,
		Email: email,
		Role:  role,
	}

	return teambackDb.WriteUser(user)
}

func runRetriggerSummary(c *cli.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if !cfg.AIEnabled {
		return fmt.Errorf("AI features are not enabled (set OPENAI_API_KEY)")
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := database.Migrate(db, "migrations"); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	teambackDb := database.NewTeambackDatabase(db)
	summarizer := ai.NewOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, cfg.OpenAIModel)

	client := core.NewClient(teambackDb, core.WithSummarizer(summarizer, teambackDb, nil))

	assignmentID := c.String("assignment-id")
	log.Printf("Re-triggering AI summary for assignment %s...", assignmentID)

	summary, err := client.TriggerSummarySync(assignmentID)
	if err != nil {
		return fmt.Errorf("triggering summary: %w", err)
	}

	if summary.Status == "completed" {
		log.Printf("Summary completed successfully (attention_required=%v)", summary.AttentionRequired)
		fmt.Println(summary.Summary)
	} else {
		log.Printf("Summary status: %s", summary.Status)
	}

	return nil
}
