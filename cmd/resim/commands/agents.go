package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	agentsCmd = &cobra.Command{
		Use:     "agents",
		Short:   "agents manages and inspects ReSim Agents (HiL Agent Status surface)",
		Long:    ``,
		Aliases: []string{"agent"},
	}
	listAgentsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists HiL Agents in the caller's org",
		Run:   listAgents,
	}
	getAgentCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Returns a single HiL Agent",
		Run:   getAgent,
	}
	removeAgentCmd = &cobra.Command{
		Use:   "remove",
		Short: "remove - Soft-deletes a HiL Agent (it will reappear if the host checks in again)",
		Run:   removeAgent,
	}

	poolLabelsCmd = &cobra.Command{
		Use:     "pool-labels",
		Short:   "pool-labels inspects HiL pool labels and their batch queues",
		Long:    ``,
		Aliases: []string{"pool-label"},
	}
	queuePoolLabelsCmd = &cobra.Command{
		Use:   "queue",
		Short: "queue - Lists the per-pool-label batch queue for the org",
		Run:   queuePoolLabels,
	}
)

const (
	agentIDKey                 = "agent-id"
	agentYesKey                = "yes"
	agentJSONKey               = "json"
	agentAllKey                = "all"
	poolLabelsProjectIDKey     = "project-id"
	poolLabelsCompletedDaysKey = "completed-since-days"
)

func init() {
	listAgentsCmd.Flags().Bool(agentAllKey, false, "Follow next-page tokens until the full list is fetched")
	listAgentsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a table")

	getAgentCmd.Flags().String(agentIDKey, "", "Agent ID (as supplied at check-in)")
	getAgentCmd.MarkFlagRequired(agentIDKey)
	getAgentCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a detail view")

	removeAgentCmd.Flags().String(agentIDKey, "", "Agent ID to remove (soft-delete)")
	removeAgentCmd.MarkFlagRequired(agentIDKey)
	removeAgentCmd.Flags().Bool(agentYesKey, false, "Skip the confirmation prompt")

	queuePoolLabelsCmd.Flags().String(poolLabelsProjectIDKey, "", "Optional: scope the queue to a project (UUID)")
	queuePoolLabelsCmd.Flags().Int(poolLabelsCompletedDaysKey, 7, "Window for completed batches (days)")
	queuePoolLabelsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of grouped output")

	agentsCmd.AddCommand(listAgentsCmd)
	agentsCmd.AddCommand(getAgentCmd)
	agentsCmd.AddCommand(removeAgentCmd)
	rootCmd.AddCommand(agentsCmd)

	poolLabelsCmd.AddCommand(queuePoolLabelsCmd)
	rootCmd.AddCommand(poolLabelsCmd)
}

func listAgents(cmd *cobra.Command, args []string) {
	follow := viper.GetBool(agentAllKey)
	asJSON := viper.GetBool(agentJSONKey)

	var pageToken *string
	var allAgents []api.Agent
	var latestKnownVersion string

	for {
		response, err := Client.ListAgentsWithResponse(
			context.Background(),
			&api.ListAgentsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			},
		)
		if err != nil {
			log.Fatal("failed to list agents:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list agents", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response from listAgents")
		}

		allAgents = append(allAgents, response.JSON200.Agents...)
		latestKnownVersion = response.JSON200.LatestKnownVersion

		next := response.JSON200.NextPageToken
		if next == nil || *next == "" || !follow {
			break
		}
		pageToken = next
	}

	if asJSON {
		OutputJson(map[string]any{
			"agents":             allAgents,
			"latestKnownVersion": latestKnownVersion,
		})
		return
	}

	if len(allAgents) == 0 {
		fmt.Println("No agents found in this org.")
		return
	}

	for _, a := range allAgents {
		printAgentRow(a, latestKnownVersion)
	}
}

func getAgent(cmd *cobra.Command, args []string) {
	agentID := viper.GetString(agentIDKey)
	asJSON := viper.GetBool(agentJSONKey)

	response, err := Client.GetAgentWithResponse(context.Background(), agentID)
	if err != nil {
		log.Fatal("failed to get agent:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatalf("agent %q not found", agentID)
	}
	ValidateResponse(http.StatusOK, "failed to get agent", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from getAgent")
	}

	if asJSON {
		OutputJson(*response.JSON200)
		return
	}
	printAgentDetail(*response.JSON200)
}

func removeAgent(cmd *cobra.Command, args []string) {
	agentID := viper.GetString(agentIDKey)
	skipConfirm := viper.GetBool(agentYesKey)

	if !skipConfirm {
		fmt.Fprintf(os.Stderr,
			"Remove agent %q? It will reappear in `resim agents list` if the host checks in again.\n[y/N]: ",
			agentID,
		)
		var resp string
		fmt.Scanln(&resp)
		if !strings.EqualFold(strings.TrimSpace(resp), "y") &&
			!strings.EqualFold(strings.TrimSpace(resp), "yes") {
			fmt.Println("Aborted.")
			return
		}
	}

	response, err := Client.RemoveAgentWithResponse(context.Background(), agentID)
	if err != nil {
		log.Fatal("failed to remove agent:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatalf("agent %q not found", agentID)
	}
	ValidateResponse(http.StatusNoContent, "failed to remove agent", response.HTTPResponse, response.Body)
	fmt.Printf("Removed agent %q.\n", agentID)
}

func queuePoolLabels(cmd *cobra.Command, args []string) {
	asJSON := viper.GetBool(agentJSONKey)

	params := &api.ListAgentPoolLabelQueueParams{}
	if pid := viper.GetString(poolLabelsProjectIDKey); pid != "" {
		uuidVal, err := uuid.Parse(pid)
		if err != nil {
			log.Fatalf("invalid --project-id value: %v", err)
		}
		params.ProjectID = &uuidVal
	}
	if days := viper.GetInt(poolLabelsCompletedDaysKey); days != 0 {
		params.CompletedSinceDays = &days
	}

	response, err := Client.ListAgentPoolLabelQueueWithResponse(context.Background(), params)
	if err != nil {
		log.Fatal("failed to list pool-label queue:", err)
	}
	ValidateResponse(http.StatusOK, "failed to list pool-label queue", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from listAgentPoolLabelQueue")
	}

	if asJSON {
		OutputJson(*response.JSON200)
		return
	}

	if len(response.JSON200.Items) == 0 {
		fmt.Println("No pool labels in the queue right now.")
		return
	}
	for _, item := range response.JSON200.Items {
		printPoolLabelQueueGroup(item)
	}
}

// printAgentRow renders a one-line summary suitable for `agents list`.
// The version trailer carries an explicit "(out of date)" suffix when
// applicable so the CLI surfaces the same signal as the UI.
func printAgentRow(a api.Agent, latestKnownVersion string) {
	verSuffix := ""
	if a.IsOutOfDate {
		if latestKnownVersion != "" {
			verSuffix = fmt.Sprintf(" (out of date; latest %s)", latestKnownVersion)
		} else {
			verSuffix = " (out of date)"
		}
	}
	fmt.Printf("%s\t%s\tv%s%s\t%s\tlast check-in %s\n",
		a.AgentID,
		a.Activity,
		a.Version,
		verSuffix,
		strings.Join(a.PoolLabels, ", "),
		a.LastCheckin.Format("2006-01-02 15:04:05"),
	)
}

func printAgentDetail(a api.Agent) {
	fmt.Printf("Agent ID:        %s\n", a.AgentID)
	fmt.Printf("Activity:        %s\n", a.Activity)
	fmt.Printf("Version:         v%s\n", a.Version)
	if a.IsOutOfDate {
		fmt.Printf("                 (out of date — visit https://docs.resim.ai for the latest agent version)\n")
	}
	fmt.Printf("Pool labels:     %s\n", strings.Join(a.PoolLabels, ", "))
	fmt.Printf("First check-in:  %s\n", a.FirstCheckin.Format("2006-01-02 15:04:05 MST"))
	fmt.Printf("Last check-in:   %s\n", a.LastCheckin.Format("2006-01-02 15:04:05 MST"))
	if len(a.RecentActivity) == 0 {
		fmt.Println("Recent activity: (none)")
		return
	}
	fmt.Println("Recent activity:")
	for _, r := range a.RecentActivity {
		branch := ""
		if r.BranchName != nil && *r.BranchName != "" {
			branch = fmt.Sprintf("  branch=%s", *r.BranchName)
		}
		fmt.Printf("  - batch %s [%s]: test %s [%s]%s  %s\n",
			r.BatchName, r.BatchConflatedStatus,
			r.JobName, r.JobConflatedStatus,
			branch,
			r.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
}

func printPoolLabelQueueGroup(item api.PoolLabelQueueItem) {
	fmt.Printf("\n=== %s (%d agents) ===\n", item.PoolLabel, len(item.AssociatedAgentIDs))
	if len(item.AssociatedAgentIDs) > 0 {
		fmt.Printf("    agents: %s\n", strings.Join(item.AssociatedAgentIDs, ", "))
	}
	if item.ActiveBatch != nil {
		fmt.Printf("  ACTIVE   %s [%s]%s  %s\n",
			item.ActiveBatch.BatchName,
			item.ActiveBatch.ConflatedStatus,
			priorityFlag(item.ActiveBatch.Priority),
			item.ActiveBatch.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	for _, b := range item.QueuedBatches {
		pos := ""
		if b.QueuePosition != nil {
			pos = fmt.Sprintf("Queued %d", *b.QueuePosition)
		} else {
			pos = "QUEUED"
		}
		fmt.Printf("  %s   %s [%s]%s  %s\n",
			pos, b.BatchName, b.ConflatedStatus, priorityFlag(b.Priority),
			b.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	if len(item.CompletedBatches) > 0 {
		fmt.Printf("  + %d completed in last 7 days\n", len(item.CompletedBatches))
	}
}

func priorityFlag(p bool) string {
	if p {
		return " (Priority)"
	}
	return ""
}
