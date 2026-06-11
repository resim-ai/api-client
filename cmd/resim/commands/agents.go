package commands

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	agentsCmd = &cobra.Command{
		Use:     "agents",
		Short:   "agents contains commands for managing and inspecting ReSim HiL Agents",
		Long:    ``,
		Aliases: []string{"agent"},
	}
	listAgentsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists all HiL Agents in the caller's org",
		Run:   listAgents,
	}
	getAgentCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Returns a single HiL Agent",
		Run:   getAgent,
	}
	archiveAgentCmd = &cobra.Command{
		Use:     "archive",
		Short:   "archive - Archives (soft-deletes) a HiL Agent; it reappears if the host checks in again",
		Aliases: []string{"remove", "hide"},
		Run:     archiveAgent,
	}

	poolLabelsCmd = &cobra.Command{
		Use:     "pool-labels",
		Short:   "pool-labels contains commands for inspecting HiL pool labels and their batch queues",
		Long:    ``,
		Aliases: []string{"pool-label"},
	}
	queuePoolLabelsCmd = &cobra.Command{
		Use:   "queue",
		Short: "queue - Lists the per-pool-label batch queue across the caller's org",
		Run:   queuePoolLabels,
	}
)

const (
	agentIDKey                 = "agent-id"
	agentYesKey                = "yes"
	agentJSONKey               = "json"
	poolLabelsCompletedDaysKey = "completed-since-days"

	completedSinceDaysMin = 1
	completedSinceDaysMax = 30
)

func init() {
	listAgentsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a table")

	getAgentCmd.Flags().String(agentIDKey, "", "Agent ID (as supplied at check-in)")
	getAgentCmd.MarkFlagRequired(agentIDKey)
	getAgentCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a detail view")

	archiveAgentCmd.Flags().String(agentIDKey, "", "Agent ID to archive (soft-delete)")
	archiveAgentCmd.MarkFlagRequired(agentIDKey)
	archiveAgentCmd.Flags().Bool(agentYesKey, false, "Skip the confirmation prompt")

	queuePoolLabelsCmd.Flags().Int(poolLabelsCompletedDaysKey, 7, "Window for completed batches, in days (1-30)")
	queuePoolLabelsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of grouped output")

	agentsCmd.AddCommand(listAgentsCmd)
	agentsCmd.AddCommand(getAgentCmd)
	agentsCmd.AddCommand(archiveAgentCmd)
	rootCmd.AddCommand(agentsCmd)

	poolLabelsCmd.AddCommand(queuePoolLabelsCmd)
	rootCmd.AddCommand(poolLabelsCmd)
}

func actualListAgents() *api.ListAgentsOutput {
	response, err := Client.ListAgentsWithResponse(context.Background())
	if err != nil {
		log.Fatal("failed to list agents:", err)
	}
	ValidateResponse(http.StatusOK, "failed to list agents", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from listAgents")
	}
	return response.JSON200
}

func listAgents(cmd *cobra.Command, args []string) {
	output := actualListAgents()

	if viper.GetBool(agentJSONKey) {
		OutputJson(*output)
		return
	}

	if len(output.Agents) == 0 {
		fmt.Println("No agents found in this org.")
		return
	}

	for _, a := range output.Agents {
		fmt.Print(formatAgentRow(a, output.LatestKnownVersion))
	}
}

func actualGetAgent(agentID string) *api.Agent {
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
	return response.JSON200
}

func getAgent(cmd *cobra.Command, args []string) {
	agent := actualGetAgent(viper.GetString(agentIDKey))

	if viper.GetBool(agentJSONKey) {
		OutputJson(*agent)
		return
	}
	fmt.Print(formatAgentDetail(*agent))
}

// confirmArchiveAgent reads a yes/no answer from in. Anything other than an
// explicit yes aborts.
func confirmArchiveAgent(in io.Reader, agentID string) bool {
	fmt.Fprintf(os.Stderr,
		"Archive agent %q? It will reappear in `resim agents list` if the host checks in again.\n[y/N]: ",
		agentID,
	)
	var resp string
	fmt.Fscanln(in, &resp)
	resp = strings.TrimSpace(resp)
	return strings.EqualFold(resp, "y") || strings.EqualFold(resp, "yes")
}

func actualArchiveAgent(agentID string) *api.ArchiveAgentOutput {
	response, err := Client.ArchiveAgentWithResponse(context.Background(), agentID)
	if err != nil {
		log.Fatal("failed to archive agent:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatalf("agent %q not found", agentID)
	}
	ValidateResponse(http.StatusOK, "failed to archive agent", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from archiveAgent")
	}
	return response.JSON200
}

func archiveAgent(cmd *cobra.Command, args []string) {
	agentID := viper.GetString(agentIDKey)

	if !viper.GetBool(agentYesKey) && !confirmArchiveAgent(os.Stdin, agentID) {
		fmt.Println("Aborted.")
		return
	}

	output := actualArchiveAgent(agentID)
	fmt.Printf("Archived agent %q at %s.\n", output.AgentID, output.ArchivedAt.Format("2006-01-02 15:04:05 MST"))
}

// validateCompletedSinceDays enforces the server's accepted range client-side
// so the user gets a clear message instead of a raw 400.
func validateCompletedSinceDays(days int) error {
	if days < completedSinceDaysMin || days > completedSinceDaysMax {
		return fmt.Errorf("--%s must be between %d and %d, got %d",
			poolLabelsCompletedDaysKey, completedSinceDaysMin, completedSinceDaysMax, days)
	}
	return nil
}

func actualPoolLabelQueue(completedSinceDays int) *api.ListPoolLabelQueueOutput {
	response, err := Client.ListAgentPoolLabelQueueWithResponse(
		context.Background(),
		&api.ListAgentPoolLabelQueueParams{
			CompletedSinceDays: Ptr(completedSinceDays),
		},
	)
	if err != nil {
		log.Fatal("failed to list pool-label queue:", err)
	}
	ValidateResponse(http.StatusOK, "failed to list pool-label queue", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from listAgentPoolLabelQueue")
	}
	return response.JSON200
}

func queuePoolLabels(cmd *cobra.Command, args []string) {
	days := viper.GetInt(poolLabelsCompletedDaysKey)
	if err := validateCompletedSinceDays(days); err != nil {
		log.Fatal(err)
	}

	output := actualPoolLabelQueue(days)

	if viper.GetBool(agentJSONKey) {
		OutputJson(*output)
		return
	}

	if len(output.Items) == 0 {
		fmt.Println("No pool labels in the queue right now.")
		return
	}
	for _, item := range output.Items {
		fmt.Print(formatPoolLabelQueueGroup(item, days))
	}
}

// formatAgentRow renders a one-line summary suitable for `agents list`. The
// version trailer carries an explicit "(out of date)" suffix so the CLI
// surfaces the same signal as the UI; when the server reports no canonical
// latest version the indicator is suppressed entirely.
func formatAgentRow(a api.Agent, latestKnownVersion string) string {
	verSuffix := ""
	if a.IsOutOfDate && latestKnownVersion != "" {
		verSuffix = fmt.Sprintf(" (out of date; latest %s)", latestKnownVersion)
	}
	return fmt.Sprintf("%s\t%s\tv%s%s\t%s\tlast check-in %s\n",
		a.AgentID,
		a.Activity,
		a.Version,
		verSuffix,
		strings.Join(a.PoolLabels, ", "),
		a.LastCheckin.Format("2006-01-02 15:04:05"),
	)
}

func formatAgentDetail(a api.Agent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Agent ID:        %s\n", a.AgentID)
	fmt.Fprintf(&b, "Activity:        %s\n", a.Activity)
	fmt.Fprintf(&b, "Version:         v%s\n", a.Version)
	if a.IsOutOfDate {
		fmt.Fprintf(&b, "                 (out of date — visit https://docs.resim.ai for the latest agent version)\n")
	}
	fmt.Fprintf(&b, "Pool labels:     %s\n", strings.Join(a.PoolLabels, ", "))
	fmt.Fprintf(&b, "First check-in:  %s\n", a.FirstCheckin.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(&b, "Last check-in:   %s\n", a.LastCheckin.Format("2006-01-02 15:04:05 MST"))
	if len(a.RecentActivity) == 0 {
		fmt.Fprintln(&b, "Recent activity: (none)")
		return b.String()
	}
	fmt.Fprintln(&b, "Recent activity:")
	for _, r := range a.RecentActivity {
		branch := ""
		if r.BranchName != nil && *r.BranchName != "" {
			branch = fmt.Sprintf("  branch=%s", *r.BranchName)
		}
		fmt.Fprintf(&b, "  - [%s] batch %s [%s]: test %s [%s]%s  %s\n",
			r.ProjectName,
			r.BatchName, r.BatchConflatedStatus,
			r.JobName, r.JobConflatedStatus,
			branch,
			r.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	return b.String()
}

func formatPoolLabelQueueGroup(item api.PoolLabelQueueItem, completedSinceDays int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n=== %s (%d agents) ===\n", item.PoolLabel, len(item.AssociatedAgentIDs))
	if len(item.AssociatedAgentIDs) > 0 {
		fmt.Fprintf(&b, "    agents: %s\n", strings.Join(item.AssociatedAgentIDs, ", "))
	}
	for _, batch := range item.ActiveBatches {
		fmt.Fprintf(&b, "  ACTIVE   %s [%s]%s  %s\n",
			batch.BatchName,
			batch.ConflatedStatus,
			priorityLabel(batch.Priority),
			batch.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	for _, batch := range item.QueuedBatches {
		pos := "QUEUED"
		if batch.QueuePosition != nil {
			pos = fmt.Sprintf("Queued %d", *batch.QueuePosition)
		}
		fmt.Fprintf(&b, "  %s   %s [%s]%s  %s\n",
			pos, batch.BatchName, batch.ConflatedStatus, priorityLabel(batch.Priority),
			batch.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	if len(item.CompletedBatches) > 0 {
		fmt.Fprintf(&b, "  + %d completed in last %d days\n", len(item.CompletedBatches), completedSinceDays)
	}
	return b.String()
}

// priorityLabel maps the raw scheduler priority (ascending sort, default
// requestPriorityDefault) to the same High/Low pills the UI renders. Default
// priority gets no label.
func priorityLabel(priority int) string {
	switch {
	case priority < requestPriorityDefault:
		return " (High)"
	case priority > requestPriorityDefault:
		return " (Low)"
	default:
		return ""
	}
}
