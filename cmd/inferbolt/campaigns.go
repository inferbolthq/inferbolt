package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/inferbolthq/inferbolt/internal/agent"
	"github.com/inferbolthq/inferbolt/internal/campaigns"
)

// followPollInterval is how often a follower asks for new campaign steps. A
// campaign emits a step every few minutes at most, so this is about how quickly
// output appears, not about keeping up.
const followPollInterval = 2 * time.Second

func newCampaignsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "campaigns",
		Aliases: []string{"campaign"},
		Short:   "Inspect optimization campaigns",
	}
	cmd.AddCommand(newCampaignsListCmd())
	cmd.AddCommand(newCampaignsGetCmd())
	cmd.AddCommand(newCampaignsFollowCmd())
	cmd.AddCommand(newCampaignsCancelCmd())
	return cmd
}

func newCampaignsListCmd() *cobra.Command {
	var (
		state string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List campaigns",
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := apiClient.ListCampaigns(cmd.Context(), state, limit)
			if err != nil {
				return err
			}
			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(list)
			}
			if len(list) == 0 {
				fmt.Println("No campaigns yet.")
				return nil
			}

			tbl := tablewriter.NewWriter(os.Stdout)
			tbl.SetHeader([]string{"ID", "STATE", "MODEL", "GPU", "GOAL", "RECOMMENDED", "CREATED"})
			tbl.SetBorder(false)
			for _, c := range list {
				recommended := "—"
				if c.Recommendation != nil {
					recommended = c.Recommendation.Engine + " " + configSummary(c.Recommendation.EngineConfig)
				}
				tbl.Append([]string{
					c.ID, string(c.State), c.Model, c.GPUProfile,
					truncate(c.Goal, 40), recommended,
					c.CreatedAt.Format(time.RFC3339),
				})
			}
			tbl.Render()
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Filter by state: pending, running, completed, failed, cancelled")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum campaigns to list")
	return cmd
}

func newCampaignsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <campaignID>",
		Short: "Show a campaign and its recommendation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := apiClient.GetCampaign(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if isJSON() {
				return json.NewEncoder(os.Stdout).Encode(c)
			}
			printCampaignSummary(*c)
			printReport(c.ToReport())
			return nil
		},
	}
}

func newCampaignsFollowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "follow <campaignID>",
		Short: "Stream a running campaign's progress",
		Long: `Stream a running campaign's progress.

Replays everything that has happened so far, then follows until the campaign
finishes. Safe to attach, detach and reattach — the campaign runs on the server
either way.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := apiClient.GetCampaign(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			printCampaignSummary(*c)
			return followCampaign(cmd.Context(), args[0], 0)
		},
	}
}

func newCampaignsCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <campaignID>",
		Short: "Cancel a pending or running campaign",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := apiClient.CancelCampaign(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Printf("Campaign %s cancelled. A benchmark already in flight will finish.\n", args[0])
			return nil
		},
	}
}

// followCampaign renders steps as they are recorded, resuming from afterSeq,
// and returns once the campaign reaches a terminal state. Interrupting it does
// not stop the campaign — it is running on the server.
func followCampaign(ctx context.Context, campaignID string, afterSeq int) error {
	ticker := time.NewTicker(followPollInterval)
	defer ticker.Stop()

	for {
		page, err := apiClient.CampaignEventsAfter(ctx, campaignID, afterSeq)
		if err != nil {
			// A transient read failure should not end a follow that could
			// otherwise continue; the next tick retries.
			if ctx.Err() != nil {
				return nil
			}
			fmt.Fprintf(os.Stderr, "warning: could not read campaign progress: %v\n", err)
		} else {
			for _, e := range page.Events {
				renderEvent(toAgentEvent(e))
			}
			afterSeq = page.LastSeq
			if page.Done {
				return printFinalCampaign(ctx, campaignID)
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func printFinalCampaign(ctx context.Context, campaignID string) error {
	c, err := apiClient.GetCampaign(ctx, campaignID)
	if err != nil {
		return err
	}
	if c.State == campaigns.StateFailed && c.ErrorMsg != "" {
		fmt.Fprintf(os.Stderr, "\nCampaign failed: %s\n", c.ErrorMsg)
	}
	if isJSON() {
		return json.NewEncoder(os.Stdout).Encode(c)
	}
	printReport(c.ToReport())
	return nil
}

// toAgentEvent adapts a stored event back to the shape the renderer expects, so
// live and replayed output are identical.
func toAgentEvent(e campaigns.Event) agent.Event {
	return agent.Event{
		Kind:    agent.EventKind(e.Kind),
		Text:    e.Text,
		Trial:   e.Trial,
		Result:  e.Result,
		Elapsed: time.Duration(e.ElapsedMs) * time.Millisecond,
	}
}

func printCampaignSummary(c campaigns.Campaign) {
	fmt.Printf("InferBolt campaign %s — %s\n", c.ID, string(c.State))
	fmt.Printf("Goal:     %s\n", c.Goal)
	fmt.Printf("Model:    %s on %s\n", c.Model, c.GPUProfile)
	fmt.Printf("Engines:  %s\n", strings.Join(c.Engines, ", "))
	fmt.Printf("Workload: %d concurrent, %d/%d tokens, %d requests\n",
		c.Workload.Concurrency, c.Workload.PromptTokens, c.Workload.OutputTokens, c.Workload.NumRequests)
	fmt.Printf("Budget:   %d trials, %s (planner: %s)\n\n",
		c.Budget.MaxTrials, c.Budget.MaxDuration, c.PlannerModel)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
