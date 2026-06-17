package commands

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

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
  offline          fraction of wall-clock during which the agent's heartbeat
                   had been silent for over five minutes. Idle time is not
                   shown; infer it as 1 - utilization - offline
  tests            number of job runs that started in the bucket; bucket
                   counts sum to the window's total

An agent runs one experience at a time, so there is no concurrency metric.

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
	agentUtilizationCmd.Flags().String(agentIntervalKey, "", "Bucket width: hour or day. Buckets step back from the window end one interval at a time. Defaults to day")
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

func listAgents(cmd *cobra.Command, args []string) {
	response, err := Client.ListAgentsWithResponse(context.Background())
	if err != nil {
		log.Fatal("failed to list agents:", err)
	}
	ValidateResponse(http.StatusOK, "failed to list agents", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from listAgents")
	}
	output := response.JSON200

	if viper.GetBool(agentJSONKey) {
		OutputJson(*output)
		return
	}

	if len(output.Agents) == 0 {
		fmt.Println("No agents found in this org.")
		return
	}

	// Route the header and every row through a tabwriter so the tab-separated
	// columns are padded to a consistent width; printing the raw tabs straight
	// to the terminal would snap to fixed 8-column tab stops and the columns
	// drift. The header carries the column labels so the rows can stay bare.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprint(w, agentListHeader)
	for _, a := range output.Agents {
		fmt.Fprint(w, formatAgentRow(a, output.LatestKnownVersion))
	}
	w.Flush()
}

func getAgent(cmd *cobra.Command, args []string) {
	agentID := viper.GetString(agentIDKey)
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
	agent := response.JSON200

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

func archiveAgent(cmd *cobra.Command, args []string) {
	agentID := viper.GetString(agentIDKey)

	if !viper.GetBool(agentYesKey) && !confirmArchiveAgent(os.Stdin, agentID) {
		fmt.Println("Aborted.")
		return
	}

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
	output := response.JSON200
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

func queuePoolLabels(cmd *cobra.Command, args []string) {
	days := viper.GetInt(poolLabelsCompletedDaysKey)
	if err := validateCompletedSinceDays(days); err != nil {
		log.Fatal(err)
	}

	response, err := Client.ListAgentPoolLabelQueueWithResponse(
		context.Background(),
		&api.ListAgentPoolLabelQueueParams{
			CompletedSinceDays: Ptr(days),
		},
	)
	if err != nil {
		log.Fatal("failed to list pool-label queue:", err)
	}
	ValidateResponse(http.StatusOK, "failed to list pool-label queue", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response from listAgentPoolLabelQueue")
	}
	output := response.JSON200

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

// agentListHeader labels the columns emitted by formatAgentRow. It is routed
// through the same tabwriter as the rows so the labels align over their
// values, matching the uppercase header style of the utilization tables.
const agentListHeader = "NAME\tSTATUS\tVERSION\tPOOL LABELS\tLAST CHECK-IN\n"

// agentActivityHeader labels the recent-activity table emitted by
// formatAgentDetail. Like agentListHeader it is routed through the rows'
// tabwriter so the labels align over their values; the leading indent nests
// the table under the "Recent activity:" line.
const agentActivityHeader = "  PROJECT\tBATCH\tBATCH STATUS\tTEST\tTEST STATUS\tBRANCH\tTIMESTAMP\n"

// displayVersion renders a version string with exactly one leading "v",
// regardless of whether the server already prefixed it. Without this the CLI
// double-prefixes server-supplied "v1.2.3" values into "vv1.2.3".
func displayVersion(version string) string {
	return "v" + strings.TrimPrefix(version, "v")
}

// formatAgentRow renders a one-line summary suitable for `agents list`,
// aligned under agentListHeader. The version trailer carries an explicit
// "(out of date)" suffix so the CLI surfaces the same signal as the UI; when
// the server reports no canonical latest version the indicator is suppressed
// entirely.
func formatAgentRow(a api.Agent, latestKnownVersion string) string {
	verSuffix := ""
	if a.IsOutOfDate && latestKnownVersion != "" {
		verSuffix = fmt.Sprintf(" (out of date; latest %s)", displayVersion(latestKnownVersion))
	}
	return fmt.Sprintf("%s\t%s\t%s%s\t%s\t%s\n",
		a.AgentID,
		a.Activity,
		displayVersion(a.Version),
		verSuffix,
		strings.Join(a.PoolLabels, ", "),
		a.LastCheckin.Format("2006-01-02 15:04:05"),
	)
}

func formatAgentDetail(a api.Agent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Agent ID:        %s\n", a.AgentID)
	fmt.Fprintf(&b, "Activity:        %s\n", a.Activity)
	fmt.Fprintf(&b, "Version:         %s", displayVersion(a.Version))
	if a.IsOutOfDate {
		fmt.Fprint(&b, " (out of date — visit https://docs.resim.ai for the latest agent version)")
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Pool labels:     %s\n", strings.Join(a.PoolLabels, ", "))
	fmt.Fprintf(&b, "First check-in:  %s\n", a.FirstCheckin.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(&b, "Last check-in:   %s\n", a.LastCheckin.Format("2006-01-02 15:04:05 MST"))
	if len(a.RecentActivity) == 0 {
		fmt.Fprintln(&b, "Recent activity: (none)")
		return b.String()
	}
	fmt.Fprintln(&b, "Recent activity:")
	// Render the activity as a labeled, column-aligned table: a header row of
	// column labels followed by bare values, all padded to a consistent width
	// on Flush. The header carries the field names, so the rows drop the inline
	// "batch"/"test"/"branch=" prefixes. Rows without a branch leave that cell
	// empty so the timestamp still aligns.
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprint(tw, agentActivityHeader)
	for _, r := range a.RecentActivity {
		branch := ""
		if r.BranchName != nil {
			branch = *r.BranchName
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ProjectName,
			r.BatchName, r.BatchConflatedStatus,
			r.JobName, r.JobConflatedStatus,
			branch,
			r.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	tw.Flush()
	return b.String()
}

// writeUtilizationBuckets renders the shared bucket table: one row per
// bucket, utilization and offline as percentages, and tests started in the
// bucket as a count. Idle time is not surfaced — it is inferable as
// 1 − utilization − offline.
func writeUtilizationBuckets(b *strings.Builder, buckets []api.AgentUtilizationBucket) {
	fmt.Fprintf(b, "\n%-22s%-14s%-10s%s\n", "BUCKET START", "UTILIZATION", "OFFLINE", "TESTS")
	for _, bucket := range buckets {
		fmt.Fprintf(b, "%-22s%-14s%-10s%d\n",
			bucket.BucketStart.Format("2006-01-02 15:04"),
			fmt.Sprintf("%.1f%%", bucket.Utilization*100),
			fmt.Sprintf("%.1f%%", bucket.Offline*100),
			bucket.TestsRun,
		)
	}
}

// sumBucketTestsRun totals one agent's per-bucket start counts. Each run
// counts once across the series — in the bucket it started in — so the sum
// is that agent's total tests run in the window.
func sumBucketTestsRun(buckets []api.AgentUtilizationBucket) int {
	total := 0
	for _, bucket := range buckets {
		total += bucket.TestsRun
	}
	return total
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

// utilLabelWidth is the column width of the summary header labels, sized to the
// widest label ("Queue wait:") so their values line up in a single column.
const utilLabelWidth = 11

// writeUtilizationSummary renders the window-level counters shared by the
// single-agent and org-wide outputs. The queue-wait line is omitted when the
// server reported no started run with a measurable wait.
func writeUtilizationSummary(b *strings.Builder, totalTestsRun int, avgQueueSeconds, medianQueueSeconds *float64) {
	fmt.Fprintf(b, "%-*s %d\n", utilLabelWidth, "Tests run:", totalTestsRun)
	if avgQueueSeconds != nil && medianQueueSeconds != nil {
		fmt.Fprintf(b, "%-*s avg %s, median %s\n", utilLabelWidth, "Queue wait:",
			formatSeconds(*avgQueueSeconds), formatSeconds(*medianQueueSeconds))
	}
}

// topExpNameWidth is the fixed width of the experience-name column. Names
// longer than this are truncated so they don't push the other columns out of
// alignment; the header line fits within an 80-column terminal.
const topExpNameWidth = 44

// truncate shortens s to at most maxRunes characters, replacing the tail with
// an ellipsis when it overflows. It counts runes (not bytes) so the result is
// maxRunes display columns wide, matching how fmt pads fixed-width fields.
func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes-1]) + "…"
}

// writeTopExperiences renders the experiences ranked by running time in the
// window. share is a fraction of busy time, not of wall-clock.
func writeTopExperiences(b *strings.Builder, tops []api.AgentUtilizationTopExperience) {
	if len(tops) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%-*s%-7s%-11s%s\n", topExpNameWidth, "TOP EXPERIENCES", "RUNS", "RUN TIME", "SHARE OF BUSY TIME")
	for _, e := range tops {
		// Truncate one rune short of the column width so a clipped name still
		// leaves at least one space before the RUNS column.
		fmt.Fprintf(b, "%-*s%-7d%-11s%.1f%%\n",
			topExpNameWidth, truncate(e.ExperienceName, topExpNameWidth-1),
			e.RunCount, formatSeconds(e.TotalRunSeconds), e.Share*100)
	}
}

// formatAgentUtilization renders the resolved window header followed by the
// bucket table.
func formatAgentUtilization(out api.AgentUtilizationOutput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-*s %s\n", utilLabelWidth, "Agent:", out.AgentID)
	fmt.Fprintf(&b, "%-*s %s\n", utilLabelWidth, "Interval:", out.Interval)
	fmt.Fprintf(&b, "%-*s %s — %s\n", utilLabelWidth, "Window:",
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
	fmt.Fprintf(&b, "%-*s %s\n", utilLabelWidth, "Interval:", out.Interval)
	fmt.Fprintf(&b, "%-*s %s — %s\n", utilLabelWidth, "Window:",
		out.WindowStart.Format("2006-01-02 15:04:05 MST"),
		out.WindowEnd.Format("2006-01-02 15:04:05 MST"),
	)
	writeUtilizationSummary(&b, out.TotalTestsRun, out.AvgQueueSeconds, out.MedianQueueSeconds)
	if len(out.Agents) == 0 {
		fmt.Fprintln(&b, "No agents found in this org.")
		return b.String()
	}
	for _, series := range out.Agents {
		fmt.Fprintf(&b, "\n=== %s ===\n", series.AgentID)
		if len(series.Buckets) == 0 {
			fmt.Fprintln(&b, "No buckets in the window.")
			continue
		}
		fmt.Fprintf(&b, "%-*s %d\n", utilLabelWidth, "Tests run:", sumBucketTestsRun(series.Buckets))
		writeUtilizationBuckets(&b, series.Buckets)
	}
	writeTopExperiences(&b, out.TopExperiences)
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
