package commands

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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
	agentUtilizationCmd = &cobra.Command{
		Use:   "utilization",
		Short: "utilization - Returns a bucketed utilization time-series for one or all HiL Agents",
		Long: `utilization - Returns a dense, time-ordered series of utilization buckets.

With --agent-id, returns the series for that single HiL Agent. Without it,
returns a series for every non-removed agent in the org in one request;
agents with no activity in the window appear with all-zero buckets.

Each bucket reports:
  utilization      fraction of wall-clock time the agent was running at least
                   one experience (union of running intervals, 0.0-1.0)
  idle / offline   split of the non-running remainder: offline is the fraction
                   of wall-clock during which the agent's heartbeat had been
                   silent for over five minutes, idle the residual
                   1 - utilization - offline (floored at 0)
  tests            number of job runs that started in the bucket; bucket
                   counts sum to the window's total
  avgConcurrency   running job-seconds divided by bucket wall-clock seconds
                   (>= 0.0; exceeds 1.0 when experiences run concurrently)

The window summary also reports the total tests run, the average and median
queue wait (submission to execution start) of runs started in the window, and
the top experiences by running time (--top-experiences controls how many).

Only experience-running time counts; metrics phases run on metrics workers,
not the agent. The denominator is always full wall-clock, so buckets in which
the agent was offline read 0.0. Note: a job that never recorded a terminal
transition (e.g. the agent died mid-run) counts as running until query time,
so a sustained 100% utilization can indicate a stuck run rather than a busy
rack. Offline intervals are recorded from this feature's deployment onward;
in earlier windows offline reads 0 and all non-running time appears as idle.`,
		Run: agentUtilization,
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
	agentStartTimeKey          = "start-time"
	agentEndTimeKey            = "end-time"
	agentIntervalKey           = "interval"
	agentTopExperiencesKey     = "top-experiences"
	poolLabelsCompletedDaysKey = "completed-since-days"

	completedSinceDaysMin = 1
	completedSinceDaysMax = 30

	topExperiencesDefault = 10
	topExperiencesMin     = 0
	topExperiencesMax     = 50
)

func init() {
	listAgentsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a table")

	getAgentCmd.Flags().String(agentIDKey, "", "Agent ID (as supplied at check-in)")
	getAgentCmd.MarkFlagRequired(agentIDKey)
	getAgentCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a detail view")

	archiveAgentCmd.Flags().String(agentIDKey, "", "Agent ID to archive (soft-delete)")
	archiveAgentCmd.MarkFlagRequired(agentIDKey)
	archiveAgentCmd.Flags().Bool(agentYesKey, false, "Skip the confirmation prompt")

	agentUtilizationCmd.Flags().String(agentIDKey, "", "Agent ID (as supplied at check-in). Omit to fetch utilization for all agents in the org")
	agentUtilizationCmd.Flags().String(agentStartTimeKey, "", "Inclusive window start (RFC3339, e.g. 2026-06-04T00:00:00Z). Defaults to end time minus 7 days")
	agentUtilizationCmd.Flags().String(agentEndTimeKey, "", "Exclusive window end (RFC3339). Defaults to now")
	agentUtilizationCmd.Flags().String(agentIntervalKey, "", "Bucket width: hour or day. Buckets are UTC-aligned. Defaults to day")
	agentUtilizationCmd.Flags().Int(agentTopExperiencesKey, topExperiencesDefault, "How many top experiences (ranked by running time in the window) to include (0-50). 0 omits the list")
	agentUtilizationCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a table")

	queuePoolLabelsCmd.Flags().Int(poolLabelsCompletedDaysKey, 7, "Window for completed batches, in days (1-30)")
	queuePoolLabelsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of grouped output")

	agentsCmd.AddCommand(listAgentsCmd)
	agentsCmd.AddCommand(getAgentCmd)
	agentsCmd.AddCommand(archiveAgentCmd)
	agentsCmd.AddCommand(agentUtilizationCmd)
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

// parseAgentUtilizationParams validates the raw flag values client-side so a
// malformed request never leaves the machine. Empty values stay unset so the
// server applies its documented defaults (end = now, start = end minus 7 days,
// interval = day).
func parseAgentUtilizationParams(startRaw, endRaw, intervalRaw string, topExperiences int) (api.GetAgentUtilizationParams, error) {
	params := api.GetAgentUtilizationParams{}
	if topExperiences < topExperiencesMin || topExperiences > topExperiencesMax {
		return params, fmt.Errorf("--%s must be between %d and %d, got %d",
			agentTopExperiencesKey, topExperiencesMin, topExperiencesMax, topExperiences)
	}
	params.TopExperiences = Ptr(topExperiences)
	if startRaw != "" {
		t, err := time.Parse(time.RFC3339, startRaw)
		if err != nil {
			return params, fmt.Errorf("invalid --%s value %q: expected RFC3339, e.g. 2026-06-04T00:00:00Z", agentStartTimeKey, startRaw)
		}
		params.StartTime = &t
	}
	if endRaw != "" {
		t, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return params, fmt.Errorf("invalid --%s value %q: expected RFC3339, e.g. 2026-06-11T00:00:00Z", agentEndTimeKey, endRaw)
		}
		params.EndTime = &t
	}
	if intervalRaw != "" {
		interval := api.GetAgentUtilizationParamsInterval(intervalRaw)
		if interval != api.GetAgentUtilizationParamsIntervalHour && interval != api.GetAgentUtilizationParamsIntervalDay {
			return params, fmt.Errorf("invalid --%s value %q: must be hour or day", agentIntervalKey, intervalRaw)
		}
		params.Interval = &interval
	}
	if params.StartTime != nil && params.EndTime != nil && !params.StartTime.Before(*params.EndTime) {
		return params, fmt.Errorf("--%s must be strictly before --%s", agentStartTimeKey, agentEndTimeKey)
	}
	return params, nil
}

func actualAgentUtilization(agentID string, params api.GetAgentUtilizationParams) *api.AgentUtilizationOutput {
	response, err := Client.GetAgentUtilizationWithResponse(context.Background(), agentID, &params)
	if err != nil {
		log.Fatal("failed to get agent utilization:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatalf("agent %q not found", agentID)
	}
	ValidateResponse(http.StatusOK, "failed to get agent utilization", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from getAgentUtilization")
	}
	return response.JSON200
}

// listAgentUtilizationParams converts the single-agent params to the
// all-agents operation's identical-but-distinct generated type.
func listAgentUtilizationParams(params api.GetAgentUtilizationParams) api.ListAgentUtilizationParams {
	listParams := api.ListAgentUtilizationParams{
		StartTime:      params.StartTime,
		EndTime:        params.EndTime,
		TopExperiences: params.TopExperiences,
	}
	if params.Interval != nil {
		interval := api.ListAgentUtilizationParamsInterval(*params.Interval)
		listParams.Interval = &interval
	}
	return listParams
}

func actualListAgentUtilization(params api.ListAgentUtilizationParams) *api.ListAgentUtilizationOutput {
	response, err := Client.ListAgentUtilizationWithResponse(context.Background(), &params)
	if err != nil {
		log.Fatal("failed to list agent utilization:", err)
	}
	ValidateResponse(http.StatusOK, "failed to list agent utilization", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from listAgentUtilization")
	}
	return response.JSON200
}

func agentUtilization(cmd *cobra.Command, args []string) {
	params, err := parseAgentUtilizationParams(
		viper.GetString(agentStartTimeKey),
		viper.GetString(agentEndTimeKey),
		viper.GetString(agentIntervalKey),
		viper.GetInt(agentTopExperiencesKey),
	)
	if err != nil {
		log.Fatal(err)
	}

	// No --agent-id means the org-wide series: one request for all agents.
	if viper.GetString(agentIDKey) == "" {
		output := actualListAgentUtilization(listAgentUtilizationParams(params))
		if viper.GetBool(agentJSONKey) {
			OutputJson(*output)
			return
		}
		fmt.Print(formatListAgentUtilization(*output))
		return
	}

	output := actualAgentUtilization(viper.GetString(agentIDKey), params)

	if viper.GetBool(agentJSONKey) {
		OutputJson(*output)
		return
	}
	fmt.Print(formatAgentUtilization(*output))
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

// writeUtilizationBuckets renders the shared bucket table: one row per
// bucket, the utilization/idle/offline split as percentages, tests started in
// the bucket as a count, and average concurrency as a plain ratio.
func writeUtilizationBuckets(b *strings.Builder, buckets []api.AgentUtilizationBucket) {
	fmt.Fprintf(b, "\n%-22s%-14s%-9s%-10s%-7s%s\n", "BUCKET START", "UTILIZATION", "IDLE", "OFFLINE", "TESTS", "AVG CONCURRENCY")
	for _, bucket := range buckets {
		fmt.Fprintf(b, "%-22s%-14s%-9s%-10s%-7d%.2f\n",
			bucket.BucketStart.Format("2006-01-02 15:04"),
			fmt.Sprintf("%.1f%%", bucket.Utilization*100),
			fmt.Sprintf("%.1f%%", bucket.Idle*100),
			fmt.Sprintf("%.1f%%", bucket.Offline*100),
			bucket.TestsRun,
			bucket.AvgConcurrency,
		)
	}
}

// formatSeconds renders a seconds count as a compact duration, dropping the
// zero trailers Duration.String() produces (2h0m0s -> 2h, 5m0s -> 5m).
func formatSeconds(seconds float64) string {
	d := time.Duration(seconds * float64(time.Second)).Round(time.Second)
	out := d.String()
	if strings.HasSuffix(out, "m0s") {
		out = strings.TrimSuffix(out, "0s")
	}
	if strings.HasSuffix(out, "h0m") {
		out = strings.TrimSuffix(out, "0m")
	}
	return out
}

// writeUtilizationSummary renders the window-level counters shared by the
// single-agent and org-wide outputs. The queue-wait line is omitted when the
// server reported no started run with a measurable wait.
func writeUtilizationSummary(b *strings.Builder, totalTestsRun int, avgQueueSeconds, medianQueueSeconds *float64) {
	fmt.Fprintf(b, "Tests run: %d\n", totalTestsRun)
	if avgQueueSeconds != nil && medianQueueSeconds != nil {
		fmt.Fprintf(b, "Queue wait: avg %s, median %s\n",
			formatSeconds(*avgQueueSeconds), formatSeconds(*medianQueueSeconds))
	}
}

// writeTopExperiences renders the experiences ranked by running time in the
// window. share is a fraction of busy time, not of wall-clock.
func writeTopExperiences(b *strings.Builder, tops []api.AgentUtilizationTopExperience) {
	if len(tops) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%-40s%-7s%-11s%s\n", "TOP EXPERIENCES", "RUNS", "RUN TIME", "SHARE OF BUSY TIME")
	for _, e := range tops {
		fmt.Fprintf(b, "%-40s%-7d%-11s%.1f%%\n",
			e.ExperienceName, e.RunCount, formatSeconds(e.TotalRunSeconds), e.Share*100)
	}
}

// formatAgentUtilization renders the resolved window header followed by the
// bucket table.
func formatAgentUtilization(out api.AgentUtilizationOutput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Agent:    %s\n", out.AgentID)
	fmt.Fprintf(&b, "Interval: %s\n", out.Interval)
	fmt.Fprintf(&b, "Window:   %s — %s\n",
		out.WindowStart.Format("2006-01-02 15:04:05 MST"),
		out.WindowEnd.Format("2006-01-02 15:04:05 MST"),
	)
	writeUtilizationSummary(&b, out.TotalTestsRun, out.AvgQueueSeconds, out.MedianQueueSeconds)
	if len(out.Buckets) == 0 {
		fmt.Fprintln(&b, "No buckets in the window.")
		return b.String()
	}
	writeUtilizationBuckets(&b, out.Buckets)
	writeTopExperiences(&b, out.TopExperiences)
	return b.String()
}

// formatListAgentUtilization renders the shared window header once, then one
// bucket table per agent. The window and interval are uniform across agents,
// so they are not repeated per section.
func formatListAgentUtilization(out api.ListAgentUtilizationOutput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Interval: %s\n", out.Interval)
	fmt.Fprintf(&b, "Window:   %s — %s\n",
		out.WindowStart.Format("2006-01-02 15:04:05 MST"),
		out.WindowEnd.Format("2006-01-02 15:04:05 MST"),
	)
	writeUtilizationSummary(&b, out.TotalTestsRun, out.AvgQueueSeconds, out.MedianQueueSeconds)
	if len(out.Agents) == 0 {
		fmt.Fprintln(&b, "No agents found in this org.")
		return b.String()
	}
	writeTopExperiences(&b, out.TopExperiences)
	for _, series := range out.Agents {
		fmt.Fprintf(&b, "\n=== %s ===\n", series.AgentID)
		if len(series.Buckets) == 0 {
			fmt.Fprintln(&b, "No buckets in the window.")
			continue
		}
		writeUtilizationBuckets(&b, series.Buckets)
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
