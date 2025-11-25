package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"

	"github.com/resim-ai/api-client/api"
	"github.com/slack-go/slack"
	"github.com/spf13/viper"
)

func getBatchStatusText(batch *api.Batch) string {
	if batch.Status != nil {
		switch *batch.Status {
		case api.BatchStatusSUCCEEDED:
			return "ran successfully"
		case api.BatchStatusERROR:
			return "completed with errors"
		case api.BatchStatusCANCELLED:
			return "was cancelled"
		case api.BatchStatusSUBMITTED, api.BatchStatusEXPERIENCESRUNNING, api.BatchStatusBATCHMETRICSQUEUED, api.BatchStatusBATCHMETRICSRUNNING:
			return "is running"
		}
	}
	return "had an unknown status"
}

// BatchMetadata contains suite, system, and URL information for a batch
type BatchMetadata struct {
	Suite     api.TestSuite
	System    api.System
	SuiteUrl  string
	BatchUrl  string
	SystemUrl string
}

func getBatchMetadata(batch *api.Batch) *BatchMetadata {
	baseUrl, err := url.Parse(strings.Replace(viper.GetString(urlKey), "api", "app", 1))
	if err != nil {
		log.Fatal("unable to parse url:", err)
	}
	baseUrl.Path, err = url.JoinPath("projects", batch.ProjectID.String())
	if err != nil {
		log.Fatal("unable to build base url:", err)
	}

	// Get the suite object
	suiteResponse, err := Client.GetTestSuiteWithResponse(context.Background(), *batch.ProjectID, *batch.TestSuiteID)
	if err != nil {
		log.Fatal("unable to retrieve suite for batch:", err)
	}
	ValidateResponse(http.StatusOK, "unable to retrieve suite for batch", suiteResponse.HTTPResponse, suiteResponse.Body)
	suite := *suiteResponse.JSON200

	// Get the system object
	systemResponse, err := Client.GetSystemWithResponse(context.Background(), *batch.ProjectID, *batch.SystemID)
	if err != nil {
		log.Fatal("unable to retrieve system for batch:", err)
	}
	ValidateResponse(http.StatusOK, "unable to retrieve system for batch", systemResponse.HTTPResponse, systemResponse.Body)
	system := *systemResponse.JSON200

	// Build URLs
	suiteUrl := baseUrl.JoinPath("test-suites", batch.TestSuiteID.String(), "revisions", strconv.Itoa(int(*batch.TestSuiteRevision))).String()
	batchUrl := baseUrl.JoinPath("batches", batch.BatchID.String()).String()
	systemUrl := baseUrl.JoinPath("systems", batch.SystemID.String()).String()

	return &BatchMetadata{
		Suite:     suite,
		System:    system,
		SuiteUrl:  suiteUrl,
		BatchUrl:  batchUrl,
		SystemUrl: systemUrl,
	}
}

// BatchStatusCounts contains all status count information for a batch
type BatchStatusCounts struct {
	TotalJobs int
	Passed    int
	FailBlock int
	FailWarn  int
	Error     int
	Running   int
	Cancelled int
}

func getBatchStatusCounts(batch *api.Batch) *BatchStatusCounts {
	var totalJobs int
	if batch.TotalJobs != nil {
		totalJobs = int(*batch.TotalJobs)
	}

	var succeeded, errorCount, running, cancelled int
	if batch.JobStatusCounts != nil {
		succeeded = batch.JobStatusCounts.Succeeded
		errorCount = batch.JobStatusCounts.Error
		// make sure that totalJobs is the sum of all status counts shown
		running = batch.JobStatusCounts.Submitted + batch.JobStatusCounts.Running + batch.JobStatusCounts.MetricsQueued + batch.JobStatusCounts.MetricsRunning
		cancelled = batch.JobStatusCounts.Cancelled
	}

	var failBlock, failWarn int
	if batch.JobMetricsStatusCounts != nil {
		failBlock = batch.JobMetricsStatusCounts.FailBlock
		failWarn = batch.JobMetricsStatusCounts.FailWarn
	}

	// Calculate passed count (same as bff: https://github.com/resim-ai/rerun/blob/ebf0cde9472f555ae099e08e512ed4a7dfdf01f4/bff/lib/bff/batches/conflated_status_counts.ex#L49)
	passed := succeeded - (failBlock + failWarn)

	return &BatchStatusCounts{
		TotalJobs: totalJobs,
		Passed:    passed,
		FailBlock: failBlock,
		FailWarn:  failWarn,
		Error:     errorCount,
		Running:   running,
		Cancelled: cancelled,
	}
}

func batchToSlackWebhookPayload(batch *api.Batch) *slack.WebhookMessage {
	blocks := &slack.Blocks{BlockSet: make([]slack.Block, 2)}

	metadata := getBatchMetadata(batch)
	statusCounts := getBatchStatusCounts(batch)
	statusText := getBatchStatusText(batch)

	// Intro text
	introText := fmt.Sprintf("The <%s|%s> *<%s|run>* for <%s|%s> %s with the following breakdown:", metadata.SuiteUrl, metadata.Suite.Name, metadata.BatchUrl, metadata.SystemUrl, metadata.System.Name, statusText)
	introTextBlock := slack.NewTextBlockObject("mrkdwn", introText, false, false)
	blocks.BlockSet[0] = slack.NewSectionBlock(introTextBlock, nil, nil)

	// List section - only include non-zero counts
	boldStyle := slack.RichTextSectionTextStyle{Bold: true}
	buildListElement := func(count int, label string, filter string) *slack.RichTextSection {
		return slack.NewRichTextSection(
			slack.NewRichTextSectionTextElement(fmt.Sprintf("%d ", count), nil),
			slack.NewRichTextSectionLinkElement(metadata.BatchUrl+filter, label, &boldStyle),
		)
	}

	listElements := []slack.RichTextElement{
		slack.NewRichTextSection(slack.NewRichTextSectionTextElement(fmt.Sprintf("%d total tests", statusCounts.TotalJobs), nil)),
	}
	// Always show Passed and Blocking regardless of count
	listElements = append(listElements, buildListElement(statusCounts.Passed, "Passed", "?TEST_STATUS_MULTI=Passed"))
	listElements = append(listElements, buildListElement(statusCounts.FailBlock, "Blocking", "?TEST_STATUS_MULTI=Blocker"))
	// Only show Warning, Erroring, and Running if count > 0
	if statusCounts.FailWarn > 0 {
		listElements = append(listElements, buildListElement(statusCounts.FailWarn, "Warning", "?TEST_STATUS_MULTI=Warning"))
	}
	if statusCounts.Error > 0 {
		listElements = append(listElements, buildListElement(statusCounts.Error, "Erroring", "?TEST_STATUS_MULTI=Error"))
	}
	if statusCounts.Running > 0 {
		listElements = append(listElements, buildListElement(statusCounts.Running, "Running", "?TEST_STATUS_MULTI=Running&TEST_STATUS_MULTI=Queued"))
	}
	if statusCounts.Cancelled > 0 {
		listElements = append(listElements, buildListElement(statusCounts.Cancelled, "Cancelled", "?TEST_STATUS_MULTI=Cancelled"))
	}

	listBlock := slack.NewRichTextList("bullet", 0, listElements...)
	blocks.BlockSet[1] = slack.NewRichTextBlock("list", listBlock)

	webhookPayload := slack.WebhookMessage{
		Blocks: blocks,
	}
	return &webhookPayload
}

func batchToMarkdown(batch *api.Batch) string {
	metadata := getBatchMetadata(batch)
	statusCounts := getBatchStatusCounts(batch)
	statusText := getBatchStatusText(batch)

	// Build markdown output
	var markdown strings.Builder
	markdown.WriteString(fmt.Sprintf("The [%s](%s) **[run](%s)** for [%s](%s) %s with the following breakdown:\n\n", metadata.Suite.Name, metadata.SuiteUrl, metadata.BatchUrl, metadata.System.Name, metadata.SystemUrl, statusText))
	markdown.WriteString(fmt.Sprintf("- %d total tests\n", statusCounts.TotalJobs))
	// Always show Passed and Blocking regardless of count
	markdown.WriteString(fmt.Sprintf("- %d **[Passed](%s?TEST_STATUS_MULTI=Passed)**\n", statusCounts.Passed, metadata.BatchUrl))
	markdown.WriteString(fmt.Sprintf("- %d **[Blocking](%s?TEST_STATUS_MULTI=Blocker)**\n", statusCounts.FailBlock, metadata.BatchUrl))
	// Only show Warning, Erroring, and Running if count > 0
	if statusCounts.FailWarn > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Warning](%s?TEST_STATUS_MULTI=Warning)**\n", statusCounts.FailWarn, metadata.BatchUrl))
	}
	if statusCounts.Error > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Erroring](%s?TEST_STATUS_MULTI=Error)**\n", statusCounts.Error, metadata.BatchUrl))
	}
	if statusCounts.Running > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Running](%s?TEST_STATUS_MULTI=Running&TEST_STATUS_MULTI=Queued)**\n", statusCounts.Running, metadata.BatchUrl))
	}
	if statusCounts.Cancelled > 0 {
		markdown.WriteString(fmt.Sprintf("- %d **[Cancelled](%s?TEST_STATUS_MULTI=Cancelled)**\n", statusCounts.Cancelled, metadata.BatchUrl))
	}
	return markdown.String()
}