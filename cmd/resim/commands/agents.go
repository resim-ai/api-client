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
	agentResultsCmd = &cobra.Command{
		Use:   "results",
		Short: "results - Lists a HiL Agent's full results history",
		Long: `results - Lists every test result a HiL Agent has run for the caller's org.

This is the full, paginated history behind the agent detail page's Results tab;
'agents get' shows only the most recent slice. Filter with --text (case-insensitive
substring on the test/experience name) and --created-after (RFC3339 lower bound).
All matching results are fetched and printed; the Total line counts them.`,
		Run: agentResults,
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
	listPoolLabelsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists the distinct HiL pool labels visible to the caller's org",
		Long: `list - Lists the distinct pool labels visible to your org.

Labels are ordered by most recent agent check-in (the default) or, with
--order-by rank, by trigram similarity to --name. Use --name to filter to
labels matching a search term; --order-by rank is recommended when --name is
set so the closest matches come first.`,
		Run: listPoolLabels,
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
	poolLabelsAllKey           = "all"
	poolLabelsNameKey          = "name"
	poolLabelsOrderByKey       = "order-by"

	agentResultsTextKey         = "text"
	agentResultsCreatedAfterKey = "created-after"

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

	agentResultsCmd.Flags().String(agentIDKey, "", "Agent ID (as supplied at check-in)")
	agentResultsCmd.MarkFlagRequired(agentIDKey)
	agentResultsCmd.Flags().String(agentResultsTextKey, "", "Filter to results whose test (experience) name contains this substring (case-insensitive)")
	agentResultsCmd.Flags().String(agentResultsCreatedAfterKey, "", "Only results at or after this instant (RFC3339, e.g. 2026-06-04T00:00:00Z)")
	agentResultsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a table")

	queuePoolLabelsCmd.Flags().Int(poolLabelsCompletedDaysKey, 7, "Window for completed batches, in days (1-30)")
	queuePoolLabelsCmd.Flags().Bool(poolLabelsAllKey, false, "Show every pool label, including idle ones with no agents and no runs (hidden by default)")
	queuePoolLabelsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of grouped output")

	listPoolLabelsCmd.Flags().String(poolLabelsNameKey, "", "Filter pool labels by name (substring/trigram match). Pair with --order-by rank for the closest matches first")
	listPoolLabelsCmd.Flags().String(poolLabelsOrderByKey, "", "Order results: timestamp (most recent check-in; default) or rank (similarity to --name)")
	listPoolLabelsCmd.Flags().Bool(agentJSONKey, false, "Output raw JSON instead of a list")

	agentsCmd.AddCommand(listAgentsCmd)
	agentsCmd.AddCommand(getAgentCmd)
	agentsCmd.AddCommand(archiveAgentCmd)
	agentsCmd.AddCommand(agentUtilizationCmd)
	agentsCmd.AddCommand(agentResultsCmd)
	rootCmd.AddCommand(agentsCmd)

	poolLabelsCmd.AddCommand(queuePoolLabelsCmd)
	poolLabelsCmd.AddCommand(listPoolLabelsCmd)
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

// parseAgentResultsParams validates the raw flag values client-side so a
// malformed request never leaves the machine. Empty values stay unset so the
// server returns the agent's full (unfiltered) history.
func parseAgentResultsParams(text, createdAfterRaw string) (api.ListAgentResultsParams, error) {
	params := api.ListAgentResultsParams{PageSize: Ptr(100)}
	if text != "" {
		params.Text = Ptr(text)
	}
	if createdAfterRaw != "" {
		t, err := time.Parse(time.RFC3339, createdAfterRaw)
		if err != nil {
			return params, fmt.Errorf("invalid --%s value %q: expected RFC3339, e.g. 2026-06-04T00:00:00Z", agentResultsCreatedAfterKey, createdAfterRaw)
		}
		params.CreatedAfter = &t
	}
	return params, nil
}

func agentResults(cmd *cobra.Command, args []string) {
	agentID := viper.GetString(agentIDKey)
	params, err := parseAgentResultsParams(
		viper.GetString(agentResultsTextKey),
		viper.GetString(agentResultsCreatedAfterKey),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Auto-paginate the full (filtered) history. Validate and nil-check the
	// body before reading the token so a null page can't panic. The CLI fetches
	// every page, so the accumulated count is the exact, filter-correct total —
	// the server's per-page `Total` is not needed to render the header.
	var items []api.AgentRecentActivity
	for {
		response, err := Client.ListAgentResultsWithResponse(context.Background(), agentID, &params)
		if err != nil {
			log.Fatal("failed to list agent results:", err)
		}
		if response.HTTPResponse.StatusCode == http.StatusNotFound {
			log.Fatalf("agent %q not found", agentID)
		}
		ValidateResponse(http.StatusOK, "failed to list agent results", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response from listAgentResults")
		}
		items = append(items, response.JSON200.Items...)
		if response.JSON200.NextPageToken == nil || *response.JSON200.NextPageToken == "" {
			break
		}
		params.PageToken = response.JSON200.NextPageToken
	}

	if viper.GetBool(agentJSONKey) {
		OutputJson(items)
		return
	}
	fmt.Print(formatAgentResults(agentID, items))
}

// formatAgentResults renders the agent's results history: a header naming the
// agent and the count of results returned, then the shared activity table.
// Total is the number of rows actually fetched — because the CLI exhausts
// pagination, that equals the full set matching the query.
func formatAgentResults(agentID string, items []api.AgentRecentActivity) string {
	var b strings.Builder
	if len(items) == 0 {
		fmt.Fprintf(&b, "No results for agent %q.\n", agentID)
		return b.String()
	}
	fmt.Fprintf(&b, "Agent:  %s\n", agentID)
	fmt.Fprintf(&b, "Total:  %d\n", len(items))
	writeAgentActivityTable(&b, items, "")
	return b.String()
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
	fmt.Print(formatPoolLabelQueue(output.Items, days, viper.GetBool(poolLabelsAllKey), time.Now()))
}

// validatePoolLabelsOrderBy checks the raw --order-by value client-side so a
// malformed request never leaves the machine. An empty value stays unset so the
// server applies its documented default (timestamp).
func validatePoolLabelsOrderBy(orderBy string) error {
	switch api.ListPoolLabelsParamsOrderBy(orderBy) {
	case "", api.ListPoolLabelsParamsOrderByRank, api.ListPoolLabelsParamsOrderByTimestamp:
		return nil
	default:
		return fmt.Errorf("--%s must be %s or %s, got %q",
			poolLabelsOrderByKey,
			api.ListPoolLabelsParamsOrderByTimestamp, api.ListPoolLabelsParamsOrderByRank, orderBy)
	}
}

func listPoolLabels(cmd *cobra.Command, args []string) {
	orderBy := viper.GetString(poolLabelsOrderByKey)
	if err := validatePoolLabelsOrderBy(orderBy); err != nil {
		log.Fatal(err)
	}

	params := api.ListPoolLabelsParams{PageSize: Ptr(100)}
	if name := viper.GetString(poolLabelsNameKey); name != "" {
		params.Name = Ptr(name)
	}
	if orderBy != "" {
		ob := api.ListPoolLabelsParamsOrderBy(orderBy)
		params.OrderBy = &ob
	}

	// Auto-paginate the full set: an org's distinct pool labels are bounded, and
	// the rest of the CLI hides pagination behind a single command. Validate and
	// nil-check the body before reading the token so a null page can't panic.
	var labels []api.PoolLabel
	for {
		response, err := Client.ListPoolLabelsWithResponse(context.Background(), &params)
		if err != nil {
			log.Fatal("failed to list pool labels:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list pool labels", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response from listPoolLabels")
		}
		if response.JSON200.PoolLabels != nil {
			labels = append(labels, *response.JSON200.PoolLabels...)
		}
		if response.JSON200.NextPageToken == nil || *response.JSON200.NextPageToken == "" {
			break
		}
		params.PageToken = response.JSON200.NextPageToken
	}

	if viper.GetBool(agentJSONKey) {
		OutputJson(labels)
		return
	}

	if len(labels) == 0 {
		fmt.Println("No pool labels found in this org.")
		return
	}
	fmt.Printf("%d pool label(s):\n", len(labels))
	for _, label := range labels {
		fmt.Printf("  %s\n", label)
	}
}

// agentListHeader labels the columns emitted by formatAgentRow. It is routed
// through the same tabwriter as the rows so the labels align over their
// values, matching the uppercase header style of the utilization tables.
const agentListHeader = "NAME\tSTATUS\tVERSION\tPOOL LABELS\tLAST CHECK-IN\n"

// agentActivityColumns labels the agent-activity table shared by the
// recent-activity block of `agents get` and the full history of
// `agents results`. It carries no indent; writeAgentActivityTable prepends a
// caller-supplied indent to the header and every row so each caller controls
// nesting.
const agentActivityColumns = "PROJECT\tBATCH\tBATCH STATUS\tTEST\tTEST STATUS\tBRANCH\tTIMESTAMP\n"

// writeAgentActivityTable renders agent activity rows as a column-aligned
// table on b. indent is prepended to the header and every row: `agents get`
// nests its recent-activity table under the "Recent activity:" line with a
// two-space indent, while `agents results` renders a flush top-level table
// with no indent. A row with no branch leaves that cell empty so the
// timestamp still aligns.
func writeAgentActivityTable(b *strings.Builder, activity []api.AgentRecentActivity, indent string) {
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	fmt.Fprint(tw, indent+agentActivityColumns)
	for _, r := range activity {
		branch := ""
		if r.BranchName != nil {
			branch = *r.BranchName
		}
		fmt.Fprintf(tw, "%s%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			indent,
			r.ProjectName,
			r.BatchName, r.BatchConflatedStatus,
			r.JobName, r.JobConflatedStatus,
			branch,
			r.Timestamp.Format("2006-01-02 15:04:05"),
		)
	}
	tw.Flush()
}

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
	writeAgentActivityTable(&b, a.RecentActivity, "  ")
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

// staleActiveAge is how long a batch may sit in an active (non-terminal) status
// before the queue view flags it "⚠ stale". A HiL batch still running after a
// full day is far more often a stuck/zombie run than a genuinely long job, and
// this is the cheapest place to catch one before it pins utilization at 100%.
const staleActiveAge = 24 * time.Hour

// queueNameWidth caps the batch-name column so a pathologically long name can't
// shove the age/pill columns off screen; longer names are truncated with an
// ellipsis, matching the top-experiences table.
const queueNameWidth = 40

// formatPoolLabelQueue renders the whole queue view. Labels with no agents and
// no batches of any kind are noise, so they collapse into a single footer line
// unless --all is set; the labels that actually carry signal render as cards.
// Any label that looks like a mis-quoted flag is called out by name, since it
// almost certainly got into the org's label set by accident.
func formatPoolLabelQueue(items []api.PoolLabelQueueItem, completedSinceDays int, showAll bool, now time.Time) string {
	var b strings.Builder
	hidden := 0
	var suspicious []string
	for _, item := range items {
		if looksLikeFlag(item.PoolLabel) {
			suspicious = append(suspicious, item.PoolLabel)
		}
		if !showAll && isIdleLabel(item) {
			hidden++
			continue
		}
		b.WriteString(formatPoolLabelQueueGroup(item, completedSinceDays, now))
	}
	if hidden > 0 {
		fmt.Fprintf(&b, "\n%s hidden (no agents, no runs) · --all to show\n",
			pluralize(hidden, "idle label"))
	}
	if len(suspicious) > 0 {
		b.WriteByte('\n')
		for _, label := range suspicious {
			fmt.Fprintf(&b, "⚠ suspicious label %q looks like a mis-quoted flag, not a real pool label\n", label)
		}
	}
	return b.String()
}

// isIdleLabel reports whether a label carries no signal at all: no agents, no
// active/queued runs, and nothing completed in the window.
func isIdleLabel(item api.PoolLabelQueueItem) bool {
	return len(item.AssociatedAgentIDs) == 0 &&
		len(item.ActiveBatches) == 0 &&
		len(item.QueuedBatches) == 0 &&
		len(item.CompletedBatches) == 0
}

// looksLikeFlag reports whether a pool label looks like a shell-quoting mistake
// — it starts with a dash or contains whitespace, i.e. almost certainly a flag
// (or flag plus value) captured verbatim as a label rather than a real one.
func looksLikeFlag(label string) bool {
	return strings.HasPrefix(label, "-") || strings.ContainsAny(label, " \t")
}

func formatPoolLabelQueueGroup(item api.PoolLabelQueueItem, completedSinceDays int, now time.Time) string {
	var b strings.Builder
	agentCount := len(item.AssociatedAgentIDs)
	fmt.Fprintf(&b, "\n%s · %s", item.PoolLabel, pluralize(agentCount, "agent"))
	if agentCount > 0 {
		fmt.Fprintf(&b, " · %s", strings.Join(item.AssociatedAgentIDs, ", "))
	}
	b.WriteByte('\n')

	// Size the status/position and name columns across every rendered batch so
	// active and queued rows line their ages up within the card.
	tagW, nameW := 0, 0
	consider := func(tag, name string) {
		if w := utf8.RuneCountInString(tag); w > tagW {
			tagW = w
		}
		if w := utf8.RuneCountInString(truncate(name, queueNameWidth)); w > nameW {
			nameW = w
		}
	}
	for _, batch := range item.ActiveBatches {
		consider(string(batch.ConflatedStatus), batch.BatchName)
	}
	for _, batch := range item.QueuedBatches {
		consider(queuePositionTag(batch), batch.BatchName)
	}

	for _, batch := range item.ActiveBatches {
		trailer := priorityLabel(batch.Priority)
		if now.Sub(batch.Timestamp) >= staleActiveAge {
			trailer = "  ⚠ stale" + trailer
		}
		writeQueueBatchLine(&b, "●", string(batch.ConflatedStatus), batch.BatchName,
			tagW, nameW, formatAge(batch.Timestamp, now), trailer)
	}
	for _, batch := range item.QueuedBatches {
		writeQueueBatchLine(&b, "○", queuePositionTag(batch), batch.BatchName,
			tagW, nameW, formatAge(batch.Timestamp, now), priorityLabel(batch.Priority))
	}
	if len(item.ActiveBatches) == 0 && len(item.QueuedBatches) == 0 {
		b.WriteString("  idle — no active or queued runs\n")
	}
	if len(item.CompletedBatches) > 0 {
		fmt.Fprintf(&b, "  + %d completed in the last %s\n",
			len(item.CompletedBatches), pluralize(completedSinceDays, "day"))
	}
	return b.String()
}

// writeQueueBatchLine renders one batch row: a status dot, the status/position
// tag, the (padded, possibly truncated) batch name, its age, and any trailing
// markers (stale flag, priority pill).
func writeQueueBatchLine(b *strings.Builder, marker, tag, name string, tagW, nameW int, age, trailer string) {
	fmt.Fprintf(b, "  %s %-*s  %-*s  %s%s\n",
		marker, tagW, tag, nameW, truncate(name, queueNameWidth), age, trailer)
}

// queuePositionTag renders a queued batch's 1-based slot ("Queued #3"), falling
// back to a bare "QUEUED" when the server did not report a position.
func queuePositionTag(batch api.PoolLabelQueueBatch) string {
	if batch.QueuePosition != nil {
		return fmt.Sprintf("Queued #%d", *batch.QueuePosition)
	}
	return "QUEUED"
}

// formatAge renders how long ago t was, relative to now, as a compact
// right-hand annotation ("just now", "45m ago", "3h ago", "12d ago"). It is
// deliberately coarse: the queue view cares about minutes-vs-days, not seconds.
func formatAge(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d/time.Hour))
	default:
		return fmt.Sprintf("%dd ago", int(d/(24*time.Hour)))
	}
}

// pluralize renders a count and noun with the noun pluralized for everything but
// exactly one: "1 agent", "0 agents", "4 agents".
func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
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
