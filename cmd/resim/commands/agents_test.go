package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/viper"
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

func (s *CommandsSuite) TestActualListAgents() {
	s.mockClient.On("ListAgentsWithResponse", matchContext).Return(
		&api.ListAgentsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListAgentsOutput{
				Agents:             []api.Agent{sampleAgent("agent-1", false), sampleAgent("agent-2", true), sampleAgent("agent-3", false)},
				LatestKnownVersion: "1.2.3",
			},
		}, nil)

	output := actualListAgents()
	s.Len(output.Agents, 3)
	s.Equal("1.2.3", output.LatestKnownVersion)
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

func (s *CommandsSuite) TestFormatAgentRowOutOfDateSuffix() {
	row := formatAgentRow(sampleAgent("agent-1", true), "1.2.3")
	s.Contains(row, "(out of date; latest 1.2.3)")
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

func (s *CommandsSuite) TestActualGetAgentAndDetailView() {
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

	got := actualGetAgent("agent-1")
	detail := formatAgentDetail(*got)
	s.Contains(detail, "Agent ID:        agent-1")
	s.Contains(detail, "[project-alpha] batch batch-one")
	s.Contains(detail, "branch=main")
	s.Contains(detail, "[project-beta] batch batch-two")
	// The second card has no branch: no dangling branch= marker on its line.
	s.NotContains(detail, "batch-two [RUNNING]: test test-two [RUNNING]  branch=")
}

func (s *CommandsSuite) TestFormatAgentDetailNoRecentActivity() {
	detail := formatAgentDetail(sampleAgent("agent-1", false))
	s.Contains(detail, "Recent activity: (none)")
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

func (s *CommandsSuite) TestActualPoolLabelQueuePassesWindow() {
	s.mockClient.On("ListAgentPoolLabelQueueWithResponse", matchContext,
		&api.ListAgentPoolLabelQueueParams{CompletedSinceDays: Ptr(14)}).Return(
		&api.ListAgentPoolLabelQueueResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListPoolLabelQueueOutput{Items: []api.PoolLabelQueueItem{samplePoolLabelQueueItem()}},
		}, nil)

	output := actualPoolLabelQueue(14)
	s.Len(output.Items, 1)
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroup() {
	out := formatPoolLabelQueueGroup(samplePoolLabelQueueItem(), 7)

	s.Contains(out, "=== RackHiLConfig (2 agents) ===")
	s.Contains(out, "agents: agent-1, agent-2")
	// Both active batches render, the elevated one with a High pill.
	s.Contains(out, "ACTIVE   active-one [RUNNING]")
	s.Contains(out, "ACTIVE   active-two [RUNNING] (High)")
	// Queued batches carry their 1-based positions; deprioritised gets Low.
	s.Contains(out, "Queued 1   queued-one [SUBMITTED]")
	s.Contains(out, "Queued 2   queued-two [SUBMITTED] (Low)")
	// Completed batches collapse into a footnote interpolating the window.
	s.Contains(out, "+ 3 completed in last 7 days")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroupWindowInterpolation() {
	out := formatPoolLabelQueueGroup(samplePoolLabelQueueItem(), 14)
	s.Contains(out, "+ 3 completed in last 14 days")
}

func (s *CommandsSuite) TestFormatPoolLabelQueueGroupNilQueuePosition() {
	item := samplePoolLabelQueueItem()
	item.QueuedBatches[0].QueuePosition = nil
	out := formatPoolLabelQueueGroup(item, 7)
	s.Contains(out, "QUEUED   queued-one [SUBMITTED]")
}

func (s *CommandsSuite) TestPriorityLabel() {
	s.Equal(" (High)", priorityLabel(500))
	s.Equal("", priorityLabel(1000))
	s.Equal(" (Low)", priorityLabel(2000))
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
