package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
)

func sampleAgent(agentID string, outOfDate bool) api.Agent {
	checkin := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	return api.Agent{
		AgentID:      agentID,
		Activity:     "ACTIVE",
		Version:      "1.0.0",
		IsOutOfDate:  outOfDate,
		PoolLabels:   api.PoolLabels{"RackHiLConfig"},
		FirstCheckin: checkin.Add(-24 * time.Hour),
		LastCheckin:  checkin,
	}
}

// captureStdout runs f while capturing everything written to os.Stdout.
func captureStdout(s *CommandsSuite, f func()) string {
	orig := os.Stdout
	r, w, err := os.Pipe()
	s.Require().NoError(err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	f()

	w.Close()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, readErr := r.Read(buf)
		b.Write(buf[:n])
		if readErr != nil {
			break
		}
	}
	return b.String()
}

func (s *CommandsSuite) TestListAgentsParsesResponse() {
	viper.Reset()
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	s.mockClient.On("ListAgentsWithResponse", matchContext).Return(
		&api.ListAgentsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentsOutput{
				Agents:             []api.Agent{sampleAgent("agent-1", false), sampleAgent("agent-2", true), sampleAgent("agent-3", false)},
				LatestKnownVersion: "1.2.3",
			},
		}, nil)

	out := captureStdout(s, func() { listAgents(nil, nil) })
	var parsed api.ListAgentsOutput
	s.Require().NoError(json.Unmarshal([]byte(out), &parsed))
	s.Len(parsed.Agents, 3)
	s.Equal("1.2.3", parsed.LatestKnownVersion)
}

func (s *CommandsSuite) TestListAgentsEmptyState() {
	viper.Reset()
	s.mockClient.On("ListAgentsWithResponse", matchContext).Return(
		&api.ListAgentsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListAgentsOutput{Agents: []api.Agent{}, LatestKnownVersion: ""},
		}, nil)

	out := captureStdout(s, func() { listAgents(nil, nil) })
	s.Contains(out, "No agents found in this org.")
}

func (s *CommandsSuite) TestListAgentsJSONRoundTrips() {
	viper.Reset()
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	s.mockClient.On("ListAgentsWithResponse", matchContext).Return(
		&api.ListAgentsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentsOutput{
				Agents:             []api.Agent{sampleAgent("agent-1", false)},
				LatestKnownVersion: "1.2.3",
			},
		}, nil)

	out := captureStdout(s, func() { listAgents(nil, nil) })
	var parsed api.ListAgentsOutput
	s.Require().NoError(json.Unmarshal([]byte(out), &parsed))
	s.Len(parsed.Agents, 1)
	s.Equal("agent-1", parsed.Agents[0].AgentID)
	s.Equal("1.2.3", parsed.LatestKnownVersion)
}

func (s *CommandsSuite) TestListAgentsTableOutput() {
	viper.Reset()
	s.mockClient.On("ListAgentsWithResponse", matchContext).Return(
		&api.ListAgentsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentsOutput{
				Agents:             []api.Agent{sampleAgent("agent-1", false), sampleAgent("agent-2", true)},
				LatestKnownVersion: "1.2.3",
			},
		}, nil)

	out := captureStdout(s, func() { listAgents(nil, nil) })
	s.Contains(out, "NAME")
	s.Contains(out, "STATUS")
	s.Contains(out, "VERSION")
	s.Contains(out, "POOL LABELS")
	s.Contains(out, "LAST CHECK-IN")
	s.Contains(out, "agent-1")
	s.Contains(out, "agent-2")
	s.Contains(out, "(out of date; latest v1.2.3)")
	// The header carries the label, so rows no longer repeat it.
	s.NotContains(out, "last check-in")
}

func (s *CommandsSuite) TestFormatAgentRowOutOfDateSuffix() {
	row := formatAgentRow(sampleAgent("agent-1", true), "1.2.3")
	s.Contains(row, "(out of date; latest v1.2.3)")
}

// TestFormatAgentVersionSinglePrefix guards against double-prefixing: the CLI
// renders exactly one leading "v" whether or not the server already supplied
// the prefix on the agent version or the latest-known version.
func (s *CommandsSuite) TestFormatAgentVersionSinglePrefix() {
	agent := sampleAgent("agent-1", true)
	agent.Version = "v1.0.0"
	row := formatAgentRow(agent, "v1.2.3")
	s.Contains(row, "v1.0.0")
	s.NotContains(row, "vv1.0.0")
	s.Contains(row, "(out of date; latest v1.2.3)")
	s.NotContains(row, "vv1.2.3")

	detail := formatAgentDetail(agent)
	s.Contains(detail, "Version:         v1.0.0")
	s.NotContains(detail, "vv1.0.0")
}

func (s *CommandsSuite) TestFormatAgentRowSuppressesSuffixWithoutLatestVersion() {
	// The server reports no canonical latest version: the indicator is
	// suppressed even for agents flagged out of date.
	row := formatAgentRow(sampleAgent("agent-1", true), "")
	s.NotContains(row, "out of date")
}

func (s *CommandsSuite) TestFormatAgentRowUpToDate() {
	row := formatAgentRow(sampleAgent("agent-1", false), "1.2.3")
	s.NotContains(row, "out of date")
	s.Contains(row, "agent-1")
	s.Contains(row, "RackHiLConfig")
}

func (s *CommandsSuite) TestGetAgentDetailViewWithRecentActivity() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	defer viper.Reset()
	agent := sampleAgent("agent-1", false)
	agent.RecentActivity = []api.AgentRecentActivity{
		{
			BatchID:              uuid.New(),
			BatchName:            "batch-one",
			BatchConflatedStatus: api.ConflatedBatchStatusCOMPLETE,
			JobID:                uuid.New(),
			JobName:              "test-one",
			JobConflatedStatus:   api.ConflatedJobStatusPASSED,
			ProjectName:          "project-alpha",
			BranchName:           Ptr("main"),
			Timestamp:            time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
		},
		{
			BatchID:              uuid.New(),
			BatchName:            "batch-two",
			BatchConflatedStatus: api.ConflatedBatchStatusRUNNING,
			JobID:                uuid.New(),
			JobName:              "test-two",
			JobConflatedStatus:   api.ConflatedJobStatusRUNNING,
			ProjectName:          "project-beta",
			Timestamp:            time.Date(2026, 6, 1, 11, 30, 0, 0, time.UTC),
		},
	}
	s.mockClient.On("GetAgentWithResponse", matchContext, "agent-1").Return(
		&api.GetAgentResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &agent,
		}, nil)

	detail := captureStdout(s, func() { getAgent(nil, nil) })
	s.Contains(detail, "Agent ID:        agent-1")
	// Recent activity is a labeled, column-aligned table: assert on the header
	// labels and the bare cell values (cells are padded apart, and the inline
	// "batch"/"test"/"branch=" prefixes are gone now that the header labels them).
	s.Contains(detail, "PROJECT")
	s.Contains(detail, "BATCH STATUS")
	s.Contains(detail, "TEST STATUS")
	s.Contains(detail, "BRANCH")
	s.Contains(detail, "TIMESTAMP")
	s.Contains(detail, "project-alpha")
	s.Contains(detail, "batch-one")
	s.Contains(detail, "test-one")
	s.Contains(detail, "main")
	s.Contains(detail, "project-beta")
	s.Contains(detail, "batch-two")
	// The second card has no branch: its row carries no branch value (the only
	// branch in the fixture, "main", belongs to the first row).
	for _, line := range strings.Split(detail, "\n") {
		if strings.Contains(line, "batch-two") {
			s.NotContains(line, "main")
		}
	}
}

func (s *CommandsSuite) TestFormatAgentDetailNoRecentActivity() {
	detail := formatAgentDetail(sampleAgent("agent-1", false))
	s.Contains(detail, "Recent activity: (none)")
}

func (s *CommandsSuite) TestFormatAgentDetailOutOfDateIndicator() {
	detail := formatAgentDetail(sampleAgent("agent-1", true))
	s.Contains(detail, "out of date")
	s.Contains(detail, "docs.resim.ai")
}

func (s *CommandsSuite) TestGetAgentDetailOutput() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	defer viper.Reset()
	agent := sampleAgent("agent-1", false)
	s.mockClient.On("GetAgentWithResponse", matchContext, "agent-1").Return(
		&api.GetAgentResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &agent,
		}, nil)

	out := captureStdout(s, func() { getAgent(nil, nil) })
	s.Contains(out, "Agent ID:        agent-1")
	s.Contains(out, "Pool labels:     RackHiLConfig")
}

func (s *CommandsSuite) TestGetAgentJSONRoundTrips() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	agent := sampleAgent("agent-1", true)
	s.mockClient.On("GetAgentWithResponse", matchContext, "agent-1").Return(
		&api.GetAgentResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &agent,
		}, nil)

	out := captureStdout(s, func() { getAgent(nil, nil) })
	var parsed api.Agent
	s.Require().NoError(json.Unmarshal([]byte(out), &parsed))
	s.Equal("agent-1", parsed.AgentID)
	s.True(parsed.IsOutOfDate)
}

func (s *CommandsSuite) TestConfirmArchiveAgent() {
	s.True(confirmArchiveAgent(strings.NewReader("y\n"), "agent-1"))
	s.True(confirmArchiveAgent(strings.NewReader("YES\n"), "agent-1"))
	s.False(confirmArchiveAgent(strings.NewReader("n\n"), "agent-1"))
	s.False(confirmArchiveAgent(strings.NewReader("\n"), "agent-1"))
}

func (s *CommandsSuite) TestArchiveAgentWithYesFlag() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	viper.Set(agentYesKey, true)
	defer viper.Reset()

	archivedAt := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	s.mockClient.On("ArchiveAgentWithResponse", matchContext, "agent-1").Return(
		&api.ArchiveAgentResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ArchiveAgentOutput{AgentID: "agent-1", ArchivedAt: archivedAt},
		}, nil)

	out := captureStdout(s, func() { archiveAgent(nil, nil) })
	s.Contains(out, `Archived agent "agent-1"`)
	s.Contains(out, "2026-06-10")
}

func (s *CommandsSuite) TestArchiveAgentDeclinedMakesNoClientCall() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	viper.Set(agentYesKey, false)
	defer viper.Reset()

	// Feed "n" to the confirmation prompt via a real pipe standing in for stdin.
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	s.Require().NoError(err)
	_, err = w.WriteString("n\n")
	s.Require().NoError(err)
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	// No expectation is registered on the mock: any client call would fail the test.
	out := captureStdout(s, func() { archiveAgent(nil, nil) })
	s.Contains(out, "Aborted.")
}

func (s *CommandsSuite) TestValidateCompletedSinceDays() {
	s.NoError(validateCompletedSinceDays(1))
	s.NoError(validateCompletedSinceDays(7))
	s.NoError(validateCompletedSinceDays(30))
	s.ErrorContains(validateCompletedSinceDays(0), "between 1 and 30")
	s.ErrorContains(validateCompletedSinceDays(31), "between 1 and 30")
}

// poolLabelQueueNow is a fixed "now" for the queue-format tests. The sample
// batches are timestamped two hours earlier so they render fresh (not stale).
var poolLabelQueueNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func samplePoolLabelQueueItem() api.PoolLabelQueueItem {
	ts := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	batch := func(name string, status api.ConflatedBatchStatus, priority int, queuePos *int) api.PoolLabelQueueBatch {
		return api.PoolLabelQueueBatch{
			BatchID:         uuid.New(),
			BatchName:       name,
			ConflatedStatus: status,
			Priority:        priority,
			QueuePosition:   queuePos,
			Timestamp:       ts,
		}
	}
	return api.PoolLabelQueueItem{
		PoolLabel: "RackHiLConfig",
		ActiveBatches: []api.PoolLabelQueueBatch{
			batch("active-one", api.ConflatedBatchStatusRUNNING, 1000, nil),
			batch("active-two", api.ConflatedBatchStatusRUNNING, 500, nil),
		},
		QueuedBatches: []api.PoolLabelQueueBatch{
			batch("queued-one", api.ConflatedBatchStatusSUBMITTED, 1000, Ptr(1)),
			batch("queued-two", api.ConflatedBatchStatusSUBMITTED, 2000, Ptr(2)),
		},
		CompletedBatches: []api.PoolLabelQueueBatch{
			batch("done-one", api.ConflatedBatchStatusCOMPLETE, 1000, nil),
			batch("done-two", api.ConflatedBatchStatusERROR, 1000, nil),
			batch("done-three", api.ConflatedBatchStatusCANCELLED, 1000, nil),
		},
		AssociatedAgentIDs: []string{"agent-1", "agent-2"},
	}
}

// lineWith returns the first line of out that contains needle, or "" if none
// does. Used to tie a trailing marker (pill, stale flag) to its batch row.
func lineWith(out, needle string) string {
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, needle) {
			return ln
		}
	}
	return ""
}

func (s *CommandsSuite) TestQueuePoolLabelsPassesWindow() {
	viper.Reset()
	viper.Set(poolLabelsCompletedDaysKey, 14)
	defer viper.Reset()
	s.mockClient.On("ListAgentPoolLabelQueueWithResponse", matchContext,
		&api.ListAgentPoolLabelQueueParams{CompletedSinceDays: Ptr(14)}).Return(
		&api.ListAgentPoolLabelQueueResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelQueueOutput{Items: []api.PoolLabelQueueItem{samplePoolLabelQueueItem()}},
		}, nil)

	out := captureStdout(s, func() { queuePoolLabels(nil, nil) })
	s.Contains(out, "RackHiLConfig · 2 agents")
	s.Contains(out, "+ 3 completed in the last 14 days")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroup() {
	out := formatPoolLabelQueueGroup(samplePoolLabelQueueItem(), 7, poolLabelQueueNow)

	// Header folds the agent count and IDs onto one line, count pluralized.
	s.Contains(out, "RackHiLConfig · 2 agents · agent-1, agent-2")
	// Both active batches render with a status dot; the elevated one gets a
	// High pill, and the age is relative to now (two hours after the batch).
	s.Contains(lineWith(out, "active-one"), "● RUNNING")
	s.Contains(lineWith(out, "active-one"), "2h ago")
	s.Contains(lineWith(out, "active-two"), "● RUNNING")
	s.Contains(lineWith(out, "active-two"), "(High)")
	// Queued batches carry their 1-based positions; deprioritised gets Low.
	s.Contains(lineWith(out, "queued-one"), "○ Queued #1")
	s.Contains(lineWith(out, "queued-two"), "○ Queued #2")
	s.Contains(lineWith(out, "queued-two"), "(Low)")
	// Completed batches collapse into a footnote interpolating the window.
	s.Contains(out, "+ 3 completed in the last 7 days")
	// Fresh batches are not flagged stale.
	s.NotContains(out, "⚠ stale")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroupWindowInterpolation() {
	out := formatPoolLabelQueueGroup(samplePoolLabelQueueItem(), 14, poolLabelQueueNow)
	s.Contains(out, "+ 3 completed in the last 14 days")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroupNilQueuePosition() {
	item := samplePoolLabelQueueItem()
	item.QueuedBatches[0].QueuePosition = nil
	out := formatPoolLabelQueueGroup(item, 7, poolLabelQueueNow)
	s.Contains(lineWith(out, "queued-one"), "○ QUEUED")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroupStale() {
	item := samplePoolLabelQueueItem()
	// Age one active batch past the 24h stale threshold; the other stays fresh.
	item.ActiveBatches[1].Timestamp = poolLabelQueueNow.Add(-12 * 24 * time.Hour)
	out := formatPoolLabelQueueGroup(item, 7, poolLabelQueueNow)
	s.Contains(lineWith(out, "active-two"), "12d ago")
	s.Contains(lineWith(out, "active-two"), "⚠ stale")
	s.NotContains(lineWith(out, "active-one"), "⚠ stale")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroupIdle() {
	item := api.PoolLabelQueueItem{
		PoolLabel:          "hil-idle-with-agent",
		AssociatedAgentIDs: []string{"hil-4"},
	}
	out := formatPoolLabelQueueGroup(item, 7, poolLabelQueueNow)
	s.Contains(out, "hil-idle-with-agent · 1 agent · hil-4")
	s.Contains(out, "idle — no active or queued runs")
	s.NotContains(out, "completed in the last")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueHidesIdleLabels() {
	items := []api.PoolLabelQueueItem{
		samplePoolLabelQueueItem(),
		{PoolLabel: "empty-a"},
		{PoolLabel: "empty-b"},
	}
	// Default: idle labels collapse into a footer, names suppressed.
	out := formatPoolLabelQueue(items, 7, false, poolLabelQueueNow)
	s.Contains(out, "RackHiLConfig · 2 agents")
	s.Contains(out, "2 idle labels hidden (no agents, no runs) · --all to show")
	s.NotContains(out, "empty-a")
	s.NotContains(out, "empty-b")

	// --all renders every label as its own card, no hidden footer.
	all := formatPoolLabelQueue(items, 7, true, poolLabelQueueNow)
	s.Contains(all, "empty-a · 0 agents")
	s.Contains(all, "empty-b · 0 agents")
	s.NotContains(all, "idle labels hidden")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueFlagsSuspiciousLabel() {
	items := []api.PoolLabelQueueItem{
		{PoolLabel: "--pool-labels hil-test-system"},
	}
	out := formatPoolLabelQueue(items, 7, false, poolLabelQueueNow)
	s.Contains(out, `⚠ suspicious label "--pool-labels hil-test-system"`)
}

func (s *CommandsSuite) TestFormatAge() {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	s.Equal("just now", formatAge(base, base))
	s.Equal("just now", formatAge(base.Add(time.Minute), base)) // future clamps
	s.Equal("45m ago", formatAge(base.Add(-45*time.Minute), base))
	s.Equal("3h ago", formatAge(base.Add(-3*time.Hour), base))
	s.Equal("12d ago", formatAge(base.Add(-12*24*time.Hour), base))
}

func (s *CommandsSuite) TestPluralize() {
	s.Equal("0 agents", pluralize(0, "agent"))
	s.Equal("1 agent", pluralize(1, "agent"))
	s.Equal("4 agents", pluralize(4, "agent"))
	s.Equal("1 day", pluralize(1, "day"))
}

func (s *CommandsSuite) TestLooksLikeFlag() {
	s.True(looksLikeFlag("--pool-labels hil-test-system"))
	s.True(looksLikeFlag("-x"))
	s.True(looksLikeFlag("has space"))
	s.False(looksLikeFlag("hil-test-system"))
	s.False(looksLikeFlag("RackHiLConfig"))
}

func (s *CommandsSuite) TestPriorityLabel() {
	s.Equal(" (High)", priorityLabel(500))
	s.Equal("", priorityLabel(1000))
	s.Equal(" (Low)", priorityLabel(2000))
}

func (s *CommandsSuite) TestQueuePoolLabelsRendersGroups() {
	viper.Reset()
	viper.Set(poolLabelsCompletedDaysKey, 7)
	defer viper.Reset()
	s.mockClient.On("ListAgentPoolLabelQueueWithResponse", matchContext,
		&api.ListAgentPoolLabelQueueParams{CompletedSinceDays: Ptr(7)}).Return(
		&api.ListAgentPoolLabelQueueResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelQueueOutput{Items: []api.PoolLabelQueueItem{samplePoolLabelQueueItem()}},
		}, nil)

	out := captureStdout(s, func() { queuePoolLabels(nil, nil) })
	s.Contains(out, "RackHiLConfig · 2 agents")
	s.Contains(out, "+ 3 completed in the last 7 days")
}

func (s *CommandsSuite) TestQueuePoolLabelsJSONRoundTrips() {
	viper.Reset()
	viper.Set(poolLabelsCompletedDaysKey, 7)
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	s.mockClient.On("ListAgentPoolLabelQueueWithResponse", matchContext,
		&api.ListAgentPoolLabelQueueParams{CompletedSinceDays: Ptr(7)}).Return(
		&api.ListAgentPoolLabelQueueResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelQueueOutput{Items: []api.PoolLabelQueueItem{samplePoolLabelQueueItem()}},
		}, nil)

	out := captureStdout(s, func() { queuePoolLabels(nil, nil) })
	var parsed api.ListPoolLabelQueueOutput
	s.Require().NoError(json.Unmarshal([]byte(out), &parsed))
	s.Require().Len(parsed.Items, 1)
	s.Equal("RackHiLConfig", parsed.Items[0].PoolLabel)
}

func (s *CommandsSuite) TestQueuePoolLabelsEmptyState() {
	viper.Reset()
	viper.Set(poolLabelsCompletedDaysKey, 7)
	defer viper.Reset()
	s.mockClient.On("ListAgentPoolLabelQueueWithResponse", matchContext,
		&api.ListAgentPoolLabelQueueParams{CompletedSinceDays: Ptr(7)}).Return(
		&api.ListAgentPoolLabelQueueResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelQueueOutput{Items: []api.PoolLabelQueueItem{}},
		}, nil)

	out := captureStdout(s, func() { queuePoolLabels(nil, nil) })
	s.Contains(out, "No pool labels in the queue right now.")
}

func (s *CommandsSuite) TestValidatePoolLabelsOrderBy() {
	// Empty stays unset (server default); rank and timestamp are accepted.
	s.NoError(validatePoolLabelsOrderBy(""))
	s.NoError(validatePoolLabelsOrderBy("rank"))
	s.NoError(validatePoolLabelsOrderBy("timestamp"))
	// Anything else is rejected client-side with the accepted values named.
	err := validatePoolLabelsOrderBy("bogus")
	s.Require().Error(err)
	s.Contains(err.Error(), "rank")
	s.Contains(err.Error(), "timestamp")
}

func (s *CommandsSuite) TestListPoolLabelsMultiPageAccumulates() {
	viper.Reset()
	defer viper.Reset()
	// Page 1 carries a next-page token; page 2 (sent with that token) closes
	// the series. Labels from both pages must appear, proving accumulation.
	s.mockClient.On("ListPoolLabelsWithResponse", matchContext,
		&api.ListPoolLabelsParams{PageSize: Ptr(100)}).Return(
		&api.ListPoolLabelsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListPoolLabelsOutput{
				PoolLabels:    &[]api.PoolLabel{"alpha", "beta"},
				NextPageToken: Ptr("page-2"),
			},
		}, nil)
	s.mockClient.On("ListPoolLabelsWithResponse", matchContext,
		&api.ListPoolLabelsParams{PageSize: Ptr(100), PageToken: Ptr("page-2")}).Return(
		&api.ListPoolLabelsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListPoolLabelsOutput{
				PoolLabels: &[]api.PoolLabel{"gamma"},
			},
		}, nil)

	out := captureStdout(s, func() { listPoolLabels(nil, nil) })
	s.Contains(out, "3 pool label(s):")
	s.Contains(out, "alpha")
	s.Contains(out, "beta")
	s.Contains(out, "gamma")
}

func (s *CommandsSuite) TestListPoolLabelsPassesNameAndOrderBy() {
	viper.Reset()
	viper.Set(poolLabelsNameKey, "nav")
	viper.Set(poolLabelsOrderByKey, "rank")
	defer viper.Reset()
	rank := api.ListPoolLabelsParamsOrderByRank
	s.mockClient.On("ListPoolLabelsWithResponse", matchContext,
		&api.ListPoolLabelsParams{PageSize: Ptr(100), Name: Ptr("nav"), OrderBy: &rank}).Return(
		&api.ListPoolLabelsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelsOutput{PoolLabels: &[]api.PoolLabel{"nav-rig"}},
		}, nil)

	out := captureStdout(s, func() { listPoolLabels(nil, nil) })
	s.Contains(out, "nav-rig")
}

func (s *CommandsSuite) TestListPoolLabelsEmptyState() {
	viper.Reset()
	defer viper.Reset()
	s.mockClient.On("ListPoolLabelsWithResponse", matchContext,
		&api.ListPoolLabelsParams{PageSize: Ptr(100)}).Return(
		&api.ListPoolLabelsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelsOutput{PoolLabels: &[]api.PoolLabel{}},
		}, nil)

	out := captureStdout(s, func() { listPoolLabels(nil, nil) })
	s.Contains(out, "No pool labels found in this org.")
}

func (s *CommandsSuite) TestListPoolLabelsJSONRoundTrips() {
	viper.Reset()
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	s.mockClient.On("ListPoolLabelsWithResponse", matchContext,
		&api.ListPoolLabelsParams{PageSize: Ptr(100)}).Return(
		&api.ListPoolLabelsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelsOutput{PoolLabels: &[]api.PoolLabel{"alpha", "beta"}},
		}, nil)

	out := captureStdout(s, func() { listPoolLabels(nil, nil) })
	var parsed []string
	s.Require().NoError(json.Unmarshal([]byte(out), &parsed))
	s.Equal([]string{"alpha", "beta"}, parsed)
}

// sampleAgentResult builds an AgentRecentActivity row for the results tests.
// branch is optional (nil leaves the branch cell empty).
func sampleAgentResult(batchName, jobName, projectName string, branch *string) api.AgentRecentActivity {
	return api.AgentRecentActivity{
		BatchID:              uuid.New(),
		BatchName:            batchName,
		BatchConflatedStatus: api.ConflatedBatchStatusCOMPLETE,
		JobID:                uuid.New(),
		JobName:              jobName,
		JobConflatedStatus:   api.ConflatedJobStatusPASSED,
		ProjectName:          projectName,
		BranchName:           branch,
		Timestamp:            time.Date(2026, 6, 1, 11, 0, 0, 0, time.UTC),
	}
}

func (s *CommandsSuite) TestParseAgentResultsParams() {
	// Empty flags stay unset so the server returns the full history.
	params, err := parseAgentResultsParams("", "")
	s.NoError(err)
	s.Equal(Ptr(100), params.PageSize)
	s.Nil(params.Text)
	s.Nil(params.CreatedAfter)

	// Text and a valid RFC3339 lower bound are forwarded.
	params, err = parseAgentResultsParams("merge", "2026-06-01T00:00:00Z")
	s.NoError(err)
	s.Equal(Ptr("merge"), params.Text)
	s.Require().NotNil(params.CreatedAfter)
	s.True(params.CreatedAfter.Equal(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)))

	// A malformed created-after fails before any request is built.
	_, err = parseAgentResultsParams("", "notatime")
	s.Require().Error(err)
	s.Contains(err.Error(), agentResultsCreatedAfterKey)
}

func (s *CommandsSuite) TestAgentResultsMultiPageAccumulates() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	defer viper.Reset()
	// Page 1 carries a next-page token; page 2 (sent with it) closes the series.
	s.mockClient.On("ListAgentResultsWithResponse", matchContext, "agent-1",
		&api.ListAgentResultsParams{PageSize: Ptr(100)}).Return(
		&api.ListAgentResultsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentResultsOutput{
				Items: []api.AgentRecentActivity{
					sampleAgentResult("batch-a", "test-a", "proj-a", Ptr("main")),
					sampleAgentResult("batch-b", "test-b", "proj-b", nil),
				},
				NextPageToken: Ptr("p2"),
				Total:         3,
			},
		}, nil)
	s.mockClient.On("ListAgentResultsWithResponse", matchContext, "agent-1",
		&api.ListAgentResultsParams{PageSize: Ptr(100), PageToken: Ptr("p2")}).Return(
		&api.ListAgentResultsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentResultsOutput{
				Items: []api.AgentRecentActivity{
					sampleAgentResult("batch-c", "test-c", "proj-c", nil),
				},
			},
		}, nil)

	out := captureStdout(s, func() { agentResults(nil, nil) })
	s.Contains(out, "Agent:  agent-1")
	s.Contains(out, "Total:  3")
	s.Contains(out, "PROJECT")
	s.Contains(out, "proj-a")
	s.Contains(out, "batch-a")
	s.Contains(out, "proj-b")
	s.Contains(out, "proj-c")
	s.Contains(out, "main")
	// The branchless rows must not borrow the first row's branch.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "batch-b") || strings.Contains(line, "batch-c") {
			s.NotContains(line, "main")
		}
	}
}

func (s *CommandsSuite) TestAgentResultsTotalReflectsFetchedRows() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	defer viper.Reset()
	// Server Total is deliberately larger than the rows returned. Because the
	// CLI fetches every page, the header counts the rows actually shown so it
	// never overstates (the count is filter-correct regardless of server Total).
	s.mockClient.On("ListAgentResultsWithResponse", matchContext, "agent-1",
		&api.ListAgentResultsParams{PageSize: Ptr(100)}).Return(
		&api.ListAgentResultsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentResultsOutput{
				Items: []api.AgentRecentActivity{
					sampleAgentResult("batch-a", "test-a", "proj-a", nil),
					sampleAgentResult("batch-b", "test-b", "proj-b", nil),
				},
				Total: 99,
			},
		}, nil)

	out := captureStdout(s, func() { agentResults(nil, nil) })
	s.Contains(out, "Total:  2")
	s.NotContains(out, "Total:  99")
}

func (s *CommandsSuite) TestAgentResultsEmptyState() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	defer viper.Reset()
	s.mockClient.On("ListAgentResultsWithResponse", matchContext, "agent-1",
		&api.ListAgentResultsParams{PageSize: Ptr(100)}).Return(
		&api.ListAgentResultsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListAgentResultsOutput{Items: []api.AgentRecentActivity{}},
		}, nil)

	out := captureStdout(s, func() { agentResults(nil, nil) })
	s.Contains(out, `No results for agent "agent-1".`)
}

func (s *CommandsSuite) TestAgentResultsJSONRoundTrips() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	s.mockClient.On("ListAgentResultsWithResponse", matchContext, "agent-1",
		&api.ListAgentResultsParams{PageSize: Ptr(100)}).Return(
		&api.ListAgentResultsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentResultsOutput{
				Items: []api.AgentRecentActivity{sampleAgentResult("batch-a", "test-a", "proj-a", nil)},
				Total: 1,
			},
		}, nil)

	out := captureStdout(s, func() { agentResults(nil, nil) })
	var parsed []api.AgentRecentActivity
	s.Require().NoError(json.Unmarshal([]byte(out), &parsed))
	s.Require().Len(parsed, 1)
	s.Equal("batch-a", parsed[0].BatchName)
}

func (s *CommandsSuite) TestAgentResultsPassesFilters() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	viper.Set(agentResultsTextKey, "merge")
	viper.Set(agentResultsCreatedAfterKey, "2026-06-01T00:00:00Z")
	defer viper.Reset()
	after := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	s.mockClient.On("ListAgentResultsWithResponse", matchContext, "agent-1",
		mock.MatchedBy(func(p *api.ListAgentResultsParams) bool {
			return p.Text != nil && *p.Text == "merge" &&
				p.CreatedAfter != nil && p.CreatedAfter.Equal(after)
		})).Return(
		&api.ListAgentResultsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentResultsOutput{
				Items: []api.AgentRecentActivity{sampleAgentResult("batch-a", "merge-test", "proj-a", nil)},
				Total: 1,
			},
		}, nil)

	out := captureStdout(s, func() { agentResults(nil, nil) })
	s.Contains(out, "merge-test")
}

func (s *CommandsSuite) TestParseAgentUtilizationParams() {
	// All flags empty: the time/interval params stay unset so the server
	// defaults apply; topExperiences is always sent explicitly.
	params, err := parseAgentUtilizationParams("", "", "", topExperiencesDefault)
	s.NoError(err)
	s.Nil(params.StartTime)
	s.Nil(params.EndTime)
	s.Nil(params.Interval)
	s.Equal(topExperiencesDefault, *params.TopExperiences)

	// Valid values parse through.
	params, err = parseAgentUtilizationParams("2026-06-04T00:00:00Z", "2026-06-11T00:00:00Z", "hour", 0)
	s.NoError(err)
	s.Equal(time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC), params.StartTime.UTC())
	s.Equal(time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC), params.EndTime.UTC())
	s.Equal(api.GetAgentUtilizationParamsIntervalHour, *params.Interval)
	s.Equal(0, *params.TopExperiences)

	// Malformed times and unknown intervals fail before any request is made.
	_, err = parseAgentUtilizationParams("yesterday", "", "", topExperiencesDefault)
	s.ErrorContains(err, agentStartTimeKey)
	_, err = parseAgentUtilizationParams("", "2026-06-11", "", topExperiencesDefault)
	s.ErrorContains(err, agentEndTimeKey)
	_, err = parseAgentUtilizationParams("", "", "week", topExperiencesDefault)
	s.ErrorContains(err, "must be hour or day")

	// startTime >= endTime is rejected client-side, mirroring the server's 400.
	_, err = parseAgentUtilizationParams("2026-06-11T00:00:00Z", "2026-06-04T00:00:00Z", "", topExperiencesDefault)
	s.ErrorContains(err, "strictly before")

	// topExperiences outside the server's accepted range is rejected
	// client-side, mirroring the server's 400.
	_, err = parseAgentUtilizationParams("", "", "", -1)
	s.ErrorContains(err, agentTopExperiencesKey)
	_, err = parseAgentUtilizationParams("", "", "", topExperiencesMax+1)
	s.ErrorContains(err, agentTopExperiencesKey)
}

func sampleUtilizationOutput() api.AgentUtilizationOutput {
	start := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	return api.AgentUtilizationOutput{
		AgentID:            "agent-1",
		Interval:           api.AgentUtilizationOutputIntervalDay,
		WindowStart:        start,
		WindowEnd:          start.AddDate(0, 0, 2),
		TotalTestsRun:      12,
		AvgQueueSeconds:    Ptr(95.0),
		MedianQueueSeconds: Ptr(34.0),
		Buckets: []api.AgentUtilizationBucket{
			{
				BucketStart: start,
				BucketEnd:   start.AddDate(0, 0, 1),
				Utilization: 0.425,
				Offline:     0.125,
				TestsRun:    12,
			},
			{
				BucketStart: start.AddDate(0, 0, 1),
				BucketEnd:   start.AddDate(0, 0, 2),
				Utilization: 0,
				Offline:     1,
				TestsRun:    0,
			},
		},
		TopExperiences: []api.AgentUtilizationTopExperience{
			{
				ExperienceID:    uuid.MustParse("d30e0003-0000-0000-0000-000000000001"),
				ExperienceName:  "Highway merge",
				RunCount:        8,
				TotalRunSeconds: 11520,
				Share:           0.62,
			},
		},
	}
}

func (s *CommandsSuite) mockAgentUtilization(out *api.AgentUtilizationOutput) {
	s.mockClient.On("GetAgentUtilizationWithResponse", matchContext, "agent-1",
		mock.AnythingOfType("*api.GetAgentUtilizationParams")).Return(
		&api.GetAgentUtilizationResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      out,
		}, nil)
}

func (s *CommandsSuite) TestActualAgentUtilization() {
	out := sampleUtilizationOutput()
	s.mockAgentUtilization(&out)

	got := actualAgentUtilization("agent-1", api.GetAgentUtilizationParams{})
	s.Equal("agent-1", got.AgentID)
	s.Len(got.Buckets, 2)
}

func (s *CommandsSuite) TestFormatAgentUtilization() {
	out := formatAgentUtilization(sampleUtilizationOutput())
	s.Contains(out, "Agent:      agent-1")
	s.Contains(out, "Interval:   day")
	s.Contains(out, "BUCKET START")
	// Utilization renders as a percentage; the empty bucket carries explicit
	// zeros. There is no concurrency column — an agent runs one test at a time.
	s.Contains(out, "42.5%")
	s.Contains(out, "0.0%")
	s.NotContains(out, "CONCURRENCY")
	// Offline and per-bucket test counts render alongside utilization; idle is
	// not surfaced (it is inferable as 1 − utilization − offline).
	s.NotContains(out, "IDLE")
	s.Contains(out, "OFFLINE")
	s.Contains(out, "12.5%")
	// Window-level summary: total tests, queue wait, top experiences.
	s.Contains(out, "Tests run:  12")
	s.Contains(out, "Queue wait: avg 1m35s, median 34s")
	s.Contains(out, "TOP EXPERIENCES")
	s.Contains(out, "Highway merge")
	s.Contains(out, "3h12m")
	s.Contains(out, "62.0%")
}

func (s *CommandsSuite) TestFormatAgentUtilizationOmitsQueueWaitAndExperiences() {
	out := sampleUtilizationOutput()
	out.AvgQueueSeconds = nil
	out.MedianQueueSeconds = nil
	out.TopExperiences = nil
	formatted := formatAgentUtilization(out)
	s.NotContains(formatted, "Queue wait")
	s.NotContains(formatted, "TOP EXPERIENCES")
}

func (s *CommandsSuite) TestFormatSeconds() {
	s.Equal("0s", formatSeconds(0))
	s.Equal("40s", formatSeconds(40))
	s.Equal("1m35s", formatSeconds(95))
	s.Equal("5m", formatSeconds(300))
	s.Equal("2h", formatSeconds(7200))
	s.Equal("3h12m", formatSeconds(11520))
	s.Equal("1h0m5s", formatSeconds(3605))
}

func (s *CommandsSuite) TestTruncate() {
	// Short strings pass through unchanged.
	s.Equal("Highway merge", truncate("Highway merge", 44))
	// A string exactly at the limit is not truncated.
	s.Equal("abcde", truncate("abcde", 5))
	// Overflow is clipped to maxRunes display columns, ending in an ellipsis.
	got := truncate("vat_37_state_impartial_tests--vadc-starts-in-active_moving-full", 43)
	s.Equal(43, utf8.RuneCountInString(got))
	s.True(strings.HasSuffix(got, "…"))
}

func (s *CommandsSuite) TestFormatAgentUtilizationTruncatesLongExperienceName() {
	out := sampleUtilizationOutput()
	longName := "vat_37_state_impartial_tests--vadc-starts-in-active_moving-full"
	out.TopExperiences[0].ExperienceName = longName
	formatted := formatAgentUtilization(out)
	// The full name overflows the column, so it must not appear verbatim; an
	// ellipsis-clipped prefix appears instead.
	s.NotContains(formatted, longName)
	s.Contains(formatted, "vat_37_state_impartial_tests--vadc-starts-…")
}

func (s *CommandsSuite) TestFormatAgentUtilizationEmptyBuckets() {
	out := sampleUtilizationOutput()
	out.Buckets = nil
	formatted := formatAgentUtilization(out)
	s.Contains(formatted, "No buckets in the window.")
	s.NotContains(formatted, "BUCKET START")
}

func (s *CommandsSuite) TestAgentUtilizationJSONRoundTrips() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	out := sampleUtilizationOutput()
	s.mockAgentUtilization(&out)

	stdout := captureStdout(s, func() { agentUtilization(nil, nil) })
	var parsed api.AgentUtilizationOutput
	s.Require().NoError(json.Unmarshal([]byte(stdout), &parsed))
	s.Equal("agent-1", parsed.AgentID)
	s.Equal(out.Interval, parsed.Interval)
	s.Require().Len(parsed.Buckets, 2)
	s.Equal(out.Buckets[0].Utilization, parsed.Buckets[0].Utilization)
}

func (s *CommandsSuite) TestAgentUtilizationTableOutput() {
	viper.Reset()
	viper.Set(agentIDKey, "agent-1")
	defer viper.Reset()
	out := sampleUtilizationOutput()
	s.mockAgentUtilization(&out)

	stdout := captureStdout(s, func() { agentUtilization(nil, nil) })
	s.Contains(stdout, "Agent:      agent-1")
	s.Contains(stdout, "42.5%")
}

func sampleListUtilizationOutput() api.ListAgentUtilizationOutput {
	single := sampleUtilizationOutput()
	return api.ListAgentUtilizationOutput{
		Interval:           api.ListAgentUtilizationOutputIntervalDay,
		WindowStart:        single.WindowStart,
		WindowEnd:          single.WindowEnd,
		TotalTestsRun:      single.TotalTestsRun,
		AvgQueueSeconds:    single.AvgQueueSeconds,
		MedianQueueSeconds: single.MedianQueueSeconds,
		TopExperiences:     single.TopExperiences,
		Agents: []api.AgentUtilizationSeries{
			{AgentID: "agent-1", Buckets: single.Buckets},
			{AgentID: "agent-idle", Buckets: []api.AgentUtilizationBucket{
				{
					BucketStart: single.WindowStart,
					BucketEnd:   single.WindowEnd,
					Utilization: 0,
				},
			}},
		},
	}
}

func (s *CommandsSuite) mockListAgentUtilization(out *api.ListAgentUtilizationOutput) {
	s.mockClient.On("ListAgentUtilizationWithResponse", matchContext,
		mock.AnythingOfType("*api.ListAgentUtilizationParams")).Return(
		&api.ListAgentUtilizationResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      out,
		}, nil)
}

func (s *CommandsSuite) TestListAgentUtilizationParamsConversion() {
	// Unset fields stay unset so the server defaults still apply.
	converted := listAgentUtilizationParams(api.GetAgentUtilizationParams{})
	s.Nil(converted.StartTime)
	s.Nil(converted.EndTime)
	s.Nil(converted.Interval)
	s.Nil(converted.TopExperiences)

	start := time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)
	interval := api.GetAgentUtilizationParamsIntervalHour
	converted = listAgentUtilizationParams(api.GetAgentUtilizationParams{
		StartTime:      &start,
		EndTime:        &end,
		Interval:       &interval,
		TopExperiences: Ptr(25),
	})
	s.Equal(&start, converted.StartTime)
	s.Equal(&end, converted.EndTime)
	s.Equal(api.ListAgentUtilizationParamsIntervalHour, *converted.Interval)
	s.Equal(25, *converted.TopExperiences)
}

func (s *CommandsSuite) TestActualListAgentUtilization() {
	out := sampleListUtilizationOutput()
	s.mockListAgentUtilization(&out)

	got := actualListAgentUtilization(api.ListAgentUtilizationParams{})
	s.Require().Len(got.Agents, 2)
	s.Equal("agent-1", got.Agents[0].AgentID)
	s.Equal("agent-idle", got.Agents[1].AgentID)
}

func (s *CommandsSuite) TestFormatListAgentUtilization() {
	out := formatListAgentUtilization(sampleListUtilizationOutput())
	// The shared window header renders once; each agent gets its own section.
	s.Contains(out, "Interval:   day")
	s.Equal(1, strings.Count(out, "Window:"))
	s.Contains(out, "=== agent-1 ===")
	s.Contains(out, "=== agent-idle ===")
	s.Contains(out, "42.5%")
	// The idle agent's explicit zero bucket renders rather than being elided.
	s.Equal(2, strings.Count(out, "BUCKET START"))
	// The fleet-wide summary renders once before the per-agent sections; the
	// top-experiences table renders once at the very bottom, after them.
	s.Contains(out, "Tests run:  12")
	s.Contains(out, "Queue wait: avg 1m35s, median 34s")
	s.Equal(1, strings.Count(out, "TOP EXPERIENCES"))
	s.Contains(out, "Highway merge")
	s.Greater(strings.Index(out, "TOP EXPERIENCES"), strings.LastIndex(out, "=== "),
		"TOP EXPERIENCES should render after the per-agent sections")
	// Each agent section also carries its own absolute tests-run total. The
	// fleet line (12), agent-1's sum (12), and the idle agent's (0) make three
	// "Tests run:" lines; the idle agent's 0 is distinct from the fleet total.
	s.Contains(out, "Tests run:  0")
	s.Equal(3, strings.Count(out, "Tests run:"))
}

func (s *CommandsSuite) TestFormatListAgentUtilizationNoAgents() {
	out := sampleListUtilizationOutput()
	out.Agents = []api.AgentUtilizationSeries{}
	formatted := formatListAgentUtilization(out)
	s.Contains(formatted, "No agents found in this org.")
	s.NotContains(formatted, "BUCKET START")
}

// TestAgentUtilizationWithoutAgentID verifies the dispatch: no --agent-id
// means the single org-wide request, never the per-agent endpoint.
func (s *CommandsSuite) TestAgentUtilizationWithoutAgentID() {
	viper.Reset()
	defer viper.Reset()
	out := sampleListUtilizationOutput()
	s.mockListAgentUtilization(&out)

	stdout := captureStdout(s, func() { agentUtilization(nil, nil) })
	s.Contains(stdout, "=== agent-1 ===")
	s.Contains(stdout, "=== agent-idle ===")
	s.mockClient.AssertNotCalled(s.T(), "GetAgentUtilizationWithResponse",
		mock.Anything, mock.Anything, mock.Anything)
}

func (s *CommandsSuite) TestAgentUtilizationWithoutAgentIDJSONRoundTrips() {
	viper.Reset()
	viper.Set(agentJSONKey, true)
	defer viper.Reset()
	out := sampleListUtilizationOutput()
	s.mockListAgentUtilization(&out)

	stdout := captureStdout(s, func() { agentUtilization(nil, nil) })
	var parsed api.ListAgentUtilizationOutput
	s.Require().NoError(json.Unmarshal([]byte(stdout), &parsed))
	s.Require().Len(parsed.Agents, 2)
	s.Equal(out.Interval, parsed.Interval)
	s.Equal(out.Agents[0].Buckets[0].Utilization, parsed.Agents[0].Buckets[0].Utilization)
}
