package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	workflowCmd = &cobra.Command{
		Use:     "workflows",
		Short:   "workflows contains commands for creating and managing workflows",
		Long:    ``,
		Aliases: []string{"workflow"},
	}

	listWorkflowsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - List all the workflows associated with this project and their suites",
		Long:  ``,
		Run:   listWorkflows,
	}

	getWorkflowCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Retrieves a workflow by name or ID and its suites",
		Long:  ``,
		Run:   getWorkflow,
	}

	createWorkflowCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new workflow",
		Long:  ``,
		Run:   createWorkflow,
	}

	updateWorkflowCmd = &cobra.Command{
		Use:   "update",
		Short: "update - Updates an existing workflow",
		Long:  ``,
		Run:   updateWorkflow,
	}

	workflowRunsCmd = &cobra.Command{
		Use:   "runs",
		Short: "runs - Manage workflow runs",
		Long:  ``,
	}

	createWorkflowRunCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Run a workflow",
		Long:  ``,
		Run:   createWorkflowRun,
	}

	listWorkflowRunsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - List runs for a workflow",
		Long:  ``,
		Run:   listWorkflowRuns,
	}

	getWorkflowRunCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Get a workflow run and list its suite runs (batchID <> test suite)",
		Long:  ``,
		Run:   getWorkflowRun,
	}
)

const (
	workflowProjectKey                 = "project"
	workflowKey                        = "workflow"
	workflowNameKey                    = "name"
	workflowDescriptionKey             = "description"
	workflowCILinkKey                  = "ci-link"
	workflowSuitesKey                  = "suites"
	workflowSuitesFileKey              = "suites-file"
	workflowBuildIDKey                 = "build-id"
	workflowParameterKey               = "parameter"
	workflowPoolLabelsKey              = "pool-labels"
	workflowAccountKey                 = "account"
	workflowAllowableFailurePercentKey = "allowable-failure-percent"
	workflowRunIDKey                   = "run-id"
)

func init() {
	// List Workflows
	listWorkflowsCmd.Flags().String(workflowProjectKey, "", "The name or ID of the project to list workflows for")
	listWorkflowsCmd.MarkFlagRequired(workflowProjectKey)
	workflowCmd.AddCommand(listWorkflowsCmd)

	// Get Workflow
	getWorkflowCmd.Flags().String(workflowProjectKey, "", "The name or ID of the project the workflow is associated with.")
	getWorkflowCmd.MarkFlagRequired(workflowProjectKey)
	getWorkflowCmd.Flags().String(workflowKey, "", "The name or ID of the workflow to retrieve.")
	getWorkflowCmd.MarkFlagRequired(workflowKey)
	workflowCmd.AddCommand(getWorkflowCmd)

	// Create Workflow
	createWorkflowCmd.Flags().String(workflowProjectKey, "", "The name or ID of the project to create the workflow in.")
	createWorkflowCmd.MarkFlagRequired(workflowProjectKey)
	createWorkflowCmd.Flags().String(workflowNameKey, "", "The name of the workflow.")
	createWorkflowCmd.MarkFlagRequired(workflowNameKey)
	createWorkflowCmd.Flags().String(workflowDescriptionKey, "", "The description of the workflow.")
	createWorkflowCmd.MarkFlagRequired(workflowDescriptionKey)
	createWorkflowCmd.Flags().String(workflowCILinkKey, "", "An optional link to the CI workflow.")
	createWorkflowCmd.Flags().String(workflowSuitesKey, "", "JSON array of objects {testSuite, enabled} specifying suites for this workflow. testSuite may be a UUID or a test suite name.")
	createWorkflowCmd.Flags().String(workflowSuitesFileKey, "", "Path to a JSON file containing an array of objects {testSuite, enabled} specifying suites for this workflow. testSuite may be a UUID or a test suite name.")
	createWorkflowCmd.MarkFlagsOneRequired(workflowSuitesKey, workflowSuitesFileKey)
	workflowCmd.AddCommand(createWorkflowCmd)

	// Update Workflow
	updateWorkflowCmd.Flags().String(workflowProjectKey, "", "The name or ID of the project the workflow is associated with.")
	updateWorkflowCmd.MarkFlagRequired(workflowProjectKey)
	updateWorkflowCmd.Flags().String(workflowKey, "", "The name or ID of the workflow to update.")
	updateWorkflowCmd.MarkFlagRequired(workflowKey)
	updateWorkflowCmd.Flags().String(workflowNameKey, "", "A new name for the workflow.")
	updateWorkflowCmd.Flags().String(workflowDescriptionKey, "", "A new description for the workflow.")
    updateWorkflowCmd.Flags().String(workflowCILinkKey, "", "A new CI workflow link.")
    updateWorkflowCmd.Flags().String(workflowSuitesKey, "", "JSON array of objects {testSuite, enabled}. The CLI will add/remove/update suites to match this full list. testSuite may be a UUID or a test suite name.")
    updateWorkflowCmd.Flags().String(workflowSuitesFileKey, "", "Path to a JSON file containing an array of objects {testSuite, enabled}. The CLI will add/remove/update suites to match this full list. testSuite may be a UUID or a test suite name.")
	updateWorkflowCmd.MarkFlagsMutuallyExclusive(workflowSuitesKey, workflowSuitesFileKey)
	workflowCmd.AddCommand(updateWorkflowCmd)

	// Workflow Runs: parent cmd
	// Common flags for runs subcommands are added on each subcommand
	workflowRunsCmd.AddCommand(createWorkflowRunCmd)
	workflowRunsCmd.AddCommand(listWorkflowRunsCmd)
	// Workflow Runs - Get
	getWorkflowRunCmd.Flags().String(workflowProjectKey, "", "The name or ID of the project.")
	getWorkflowRunCmd.MarkFlagRequired(workflowProjectKey)
	getWorkflowRunCmd.Flags().String(workflowKey, "", "The name or ID of the workflow.")
	getWorkflowRunCmd.MarkFlagRequired(workflowKey)
	getWorkflowRunCmd.Flags().String(workflowRunIDKey, "", "The ID of the workflow run to get.")
	getWorkflowRunCmd.MarkFlagRequired(workflowRunIDKey)
	workflowRunsCmd.AddCommand(getWorkflowRunCmd)
	workflowCmd.AddCommand(workflowRunsCmd)

	// Workflow Runs - Create
	createWorkflowRunCmd.Flags().String(workflowProjectKey, "", "The name or ID of the project.")
	createWorkflowRunCmd.MarkFlagRequired(workflowProjectKey)
	createWorkflowRunCmd.Flags().String(workflowKey, "", "The name or ID of the workflow to run.")
	createWorkflowRunCmd.MarkFlagRequired(workflowKey)
	createWorkflowRunCmd.Flags().String(workflowBuildIDKey, "", "The ID of the build to use in this workflow run.")
	createWorkflowRunCmd.MarkFlagRequired(workflowBuildIDKey)
	createWorkflowRunCmd.Flags().StringSlice(workflowParameterKey, []string{}, "(Optional) Parameter overrides to pass to the build. Format: <parameter-name>=<parameter-value> or <parameter-name>:<parameter-value>. The equals sign (=) is recommended, especially if parameter names contain colons. Accepts repeated parameters or comma-separated parameters.")
	createWorkflowRunCmd.Flags().StringSlice(workflowPoolLabelsKey, []string{}, "Pool labels to determine where to run this workflow. Pool labels are interpreted as a logical AND. Accepts repeated labels or comma-separated labels.")
	createWorkflowRunCmd.Flags().String(workflowAccountKey, "", "Specify a username for a CI/CD platform account to associate with this workflow run.")
	createWorkflowRunCmd.Flags().Int(workflowAllowableFailurePercentKey, 0, "An optional percentage (0-100) that determines the maximum percentage of tests that can have an execution error and have aggregate metrics be computed and consider the run successfully completed. Defaults to 0.")

	// Workflow Runs - List
	listWorkflowRunsCmd.Flags().String(workflowProjectKey, "", "The name or ID of the project.")
	listWorkflowRunsCmd.MarkFlagRequired(workflowProjectKey)
	listWorkflowRunsCmd.Flags().String(workflowKey, "", "The name or ID of the workflow to list runs for.")
	listWorkflowRunsCmd.MarkFlagRequired(workflowKey)

	rootCmd.AddCommand(workflowCmd)
}

type workflowSuiteSummary struct {
	TestSuiteID uuid.UUID `json:"testSuiteID"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
}

type workflowSummary struct {
	WorkflowID     uuid.UUID              `json:"workflowID"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	CiWorkflowLink *string                `json:"ciWorkflowLink,omitempty"`
	Suites         []workflowSuiteSummary `json:"suites"`
}

func listWorkflows(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(workflowProjectKey))

	var pageToken *string = nil
	workflows := []api.Workflow{}
	for {
		response, err := Client.ListWorkflowsWithResponse(
			context.Background(), projectID, &api.ListWorkflowsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list workflows:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list workflows", response.HTTPResponse, response.Body)

		if response.JSON200 == nil || response.JSON200.Workflows == nil {
			break
		}
		workflows = append(workflows, response.JSON200.Workflows...)
		if response.JSON200.NextPageToken != "" {
			pageToken = &response.JSON200.NextPageToken
		} else {
			break
		}
	}

	// For each workflow, list its suites and output summary
	out := []workflowSummary{}
	for _, wf := range workflows {
		suitesResp, err := Client.ListWorkflowSuitesWithResponse(context.Background(), projectID, wf.WorkflowID)
		if err != nil {
			log.Fatal("failed to list workflow suites:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list workflow suites", suitesResp.HTTPResponse, suitesResp.Body)
		if suitesResp.JSON200 == nil || suitesResp.JSON200.WorkflowSuites == nil {
			out = append(out, workflowSummary{WorkflowID: wf.WorkflowID, Name: wf.Name, Description: wf.Description, CiWorkflowLink: wf.CiWorkflowLink, Suites: []workflowSuiteSummary{}})
			continue
		}
		converted := []workflowSuiteSummary{}
		for _, ws := range suitesResp.JSON200.WorkflowSuites {
			converted = append(converted, workflowSuiteSummary{
				TestSuiteID: ws.TestSuite.TestSuiteID,
				Name:        ws.TestSuite.Name,
				Enabled:     bool(ws.Enabled),
			})
		}
		out = append(out, workflowSummary{WorkflowID: wf.WorkflowID, Name: wf.Name, Description: wf.Description, CiWorkflowLink: wf.CiWorkflowLink, Suites: converted})
	}

	if len(out) == 0 {
		fmt.Println("no workflows")
		return
	}
	OutputJson(out)
}

func getWorkflow(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(workflowProjectKey))
	wf := actualGetWorkflow(projectID, viper.GetString(workflowKey), false)

	suitesResp, err := Client.ListWorkflowSuitesWithResponse(context.Background(), projectID, wf.WorkflowID)
	if err != nil {
		log.Fatal("failed to list workflow suites:", err)
	}
	ValidateResponse(http.StatusOK, "failed to list workflow suites", suitesResp.HTTPResponse, suitesResp.Body)

	converted := []workflowSuiteSummary{}
	if suitesResp.JSON200 != nil && suitesResp.JSON200.WorkflowSuites != nil {
		for _, ws := range suitesResp.JSON200.WorkflowSuites {
			converted = append(converted, workflowSuiteSummary{
				TestSuiteID: ws.TestSuite.TestSuiteID,
				Name:        ws.TestSuite.Name,
				Enabled:     bool(ws.Enabled),
			})
		}
	}

	OutputJson(workflowSummary{WorkflowID: wf.WorkflowID, Name: wf.Name, Description: wf.Description, CiWorkflowLink: wf.CiWorkflowLink, Suites: converted})
}

func createWorkflow(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(workflowProjectKey))

	name := viper.GetString(workflowNameKey)
	if name == "" {
		log.Fatal("empty workflow name")
	}

	description := viper.GetString(workflowDescriptionKey)
	if description == "" {
		log.Fatal("empty workflow description")
	}

	var ciLink *string
	if viper.IsSet(workflowCILinkKey) {
		link := viper.GetString(workflowCILinkKey)
		if link != "" {
			ciLink = &link
		}
	}

	// Parse suites from JSON string or JSON file
	type suiteSpec struct {
		TestSuite string `json:"testSuite"`
		Enabled   bool   `json:"enabled"`
	}
	var specs []suiteSpec
	if viper.IsSet(workflowSuitesFileKey) {
		path := viper.GetString(workflowSuitesFileKey)
		f, err := os.Open(path)
		if err != nil {
			log.Fatal("failed to open suites file: ", err)
		}
		defer f.Close()
		dec := json.NewDecoder(f)
		if err := dec.Decode(&specs); err != nil {
			log.Fatal("failed to parse suites file as JSON: ", err)
		}
	} else if viper.IsSet(workflowSuitesKey) {
		raw := viper.GetString(workflowSuitesKey)
		if err := json.Unmarshal([]byte(raw), &specs); err != nil {
			log.Fatal("failed to parse suites JSON: ", err)
		}
	} else {
		log.Fatal("you must specify suites JSON via --", workflowSuitesKey, " or --", workflowSuitesFileKey)
	}

	if len(specs) == 0 {
		log.Fatal("no suites specified")
	}

	workflowSuites := make([]api.WorkflowSuiteInput, 0, len(specs))
	for _, s := range specs {
		if s.TestSuite == "" {
			log.Fatal("suite entry missing testSuite")
		}
		var id uuid.UUID
		if parsed, err := uuid.Parse(s.TestSuite); err == nil {
			id = parsed
		} else {
			ts := actualGetTestSuite(projectID, s.TestSuite, nil, false)
			if ts == nil {
				log.Fatal("unable to find test suite: ", s.TestSuite)
			}
			id = ts.TestSuiteID
		}
		workflowSuites = append(workflowSuites, api.WorkflowSuiteInput{Enabled: s.Enabled, TestSuiteID: id})
	}

	body := api.CreateWorkflowInput{
		Name:           name,
		Description:    description,
		WorkflowSuites: workflowSuites,
	}
	if ciLink != nil {
		body.CiWorkflowLink = ciLink
	}

	response, err := Client.CreateWorkflowWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create workflow:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create workflow", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	wf := *response.JSON201

	fmt.Println("Created workflow successfully!")
	if wf.WorkflowID == uuid.Nil {
		log.Fatal("empty ID")
	}
	fmt.Println("workflow ID:", wf.WorkflowID.String())
}

func updateWorkflow(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(workflowProjectKey))
	existing := actualGetWorkflow(projectID, viper.GetString(workflowKey), false)

	req := api.UpdateWorkflowInput{}

	if viper.IsSet(workflowNameKey) {
		req.Name = Ptr(viper.GetString(workflowNameKey))
	}
	if viper.IsSet(workflowDescriptionKey) {
		req.Description = Ptr(viper.GetString(workflowDescriptionKey))
	}
	if viper.IsSet(workflowCILinkKey) {
		ci := viper.GetString(workflowCILinkKey)
		req.CiWorkflowLink = &ci
	}

    suitesSpecified := viper.IsSet(workflowSuitesKey) || viper.IsSet(workflowSuitesFileKey)

    // If nothing set, error out
    if req.Name == nil && req.Description == nil && req.CiWorkflowLink == nil && !suitesSpecified {
        log.Fatal("nothing to update; provide at least one of --name, --description, --ci-link, --suites/--suites-file")
    }

    // First, update workflow metadata if provided
    if req.Name != nil || req.Description != nil || req.CiWorkflowLink != nil {
        resp, err := Client.UpdateWorkflowWithResponse(context.Background(), projectID, existing.WorkflowID, req)
        if err != nil {
            log.Fatal("failed to update workflow:", err)
        }
        ValidateResponse(http.StatusOK, "failed to update workflow", resp.HTTPResponse, resp.Body)
        if resp.JSON200 == nil {
            log.Fatal("empty response")
        }
    }

    // Then, reconcile suites if provided
    if suitesSpecified {
        type suiteSpec struct {
            TestSuite string `json:"testSuite"`
            Enabled   bool   `json:"enabled"`
        }
        var specs []suiteSpec
        if viper.IsSet(workflowSuitesFileKey) {
            path := viper.GetString(workflowSuitesFileKey)
            f, err := os.Open(path)
            if err != nil {
                log.Fatal("failed to open suites file: ", err)
            }
            defer f.Close()
            dec := json.NewDecoder(f)
            if err := dec.Decode(&specs); err != nil {
                log.Fatal("failed to parse suites file as JSON: ", err)
            }
        } else {
            raw := viper.GetString(workflowSuitesKey)
            if err := json.Unmarshal([]byte(raw), &specs); err != nil {
                log.Fatal("failed to parse suites JSON: ", err)
            }
        }

        // Build desired map[id]enabled
        desired := make(map[uuid.UUID]bool, len(specs))
        for _, s := range specs {
            if s.TestSuite == "" {
                log.Fatal("suite entry missing testSuite")
            }
            var id uuid.UUID
            if parsed, err := uuid.Parse(s.TestSuite); err == nil {
                id = parsed
            } else {
                ts := actualGetTestSuite(projectID, s.TestSuite, nil, false)
                if ts == nil {
                    log.Fatal("unable to find test suite: ", s.TestSuite)
                }
                id = ts.TestSuiteID
            }
            desired[id] = s.Enabled
        }

        // Fetch current suites
        suitesResp, err := Client.ListWorkflowSuitesWithResponse(context.Background(), projectID, existing.WorkflowID)
        if err != nil {
            log.Fatal("failed to list workflow suites:", err)
        }
        ValidateResponse(http.StatusOK, "failed to list workflow suites", suitesResp.HTTPResponse, suitesResp.Body)

        current := make(map[uuid.UUID]bool)
        if suitesResp.JSON200 != nil && suitesResp.JSON200.WorkflowSuites != nil {
            for _, ws := range suitesResp.JSON200.WorkflowSuites {
                current[ws.TestSuite.TestSuiteID] = bool(ws.Enabled)
            }
        }

        // Compute diffs
        creates := make([]api.CreateWorkflowSuiteInput, 0)
        updates := make([]api.UpdateWorkflowSuiteInput, 0)
        deletes := make([]api.TestSuiteID, 0)

        for id, enabled := range desired {
            if curEnabled, ok := current[id]; !ok {
                creates = append(creates, api.CreateWorkflowSuiteInput{TestSuiteID: api.TestSuiteID(id), Enabled: enabled})
            } else if curEnabled != enabled {
                updates = append(updates, api.UpdateWorkflowSuiteInput{TestSuiteID: api.TestSuiteID(id), Enabled: enabled})
            }
        }
        for id := range current {
            if _, ok := desired[id]; !ok {
                deletes = append(deletes, api.TestSuiteID(id))
            }
        }

        // Apply changes: create -> update -> delete
        if len(creates) > 0 {
            resp, err := Client.CreateWorkflowSuitesWithResponse(context.Background(), projectID, existing.WorkflowID, api.CreateWorkflowSuitesInput{WorkflowSuites: creates})
            if err != nil {
                log.Fatal("failed to add workflow suites:", err)
            }
            ValidateResponse(http.StatusCreated, "failed to add workflow suites", resp.HTTPResponse, resp.Body)
        }
        if len(updates) > 0 {
            resp, err := Client.UpdateWorkflowSuitesWithResponse(context.Background(), projectID, existing.WorkflowID, api.UpdateWorkflowSuitesInput{WorkflowSuites: updates})
            if err != nil {
                log.Fatal("failed to update workflow suites:", err)
            }
            ValidateResponse(http.StatusOK, "failed to update workflow suites", resp.HTTPResponse, resp.Body)
        }
        if len(deletes) > 0 {
            resp, err := Client.DeleteWorkflowSuitesWithResponse(context.Background(), projectID, existing.WorkflowID, api.DeleteWorkflowSuitesInput{TestSuiteIDs: deletes})
            if err != nil {
                log.Fatal("failed to remove workflow suites:", err)
            }
            // DELETE typically returns 204 No Content
            ValidateResponse(http.StatusNoContent, "failed to remove workflow suites", resp.HTTPResponse, resp.Body)
        }

        fmt.Println("Reconciled workflow suites successfully!")
    }

    fmt.Println("Updated workflow successfully!")
    fmt.Println("workflow ID:", existing.WorkflowID.String())
}

func createWorkflowRun(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(workflowProjectKey))
	wf := actualGetWorkflow(projectID, viper.GetString(workflowKey), false)

	buildID, err := uuid.Parse(viper.GetString(workflowBuildIDKey))
	if err != nil {
		log.Fatal("failed to parse build ID: ", err)
	}

	// Parse --parameter (if any provided)
	parameters := api.BatchParameters{}
	if viper.IsSet(workflowParameterKey) {
		parameterStrings := viper.GetStringSlice(workflowParameterKey)
		for _, parameterString := range parameterStrings {
			key, value, err := ParseParameterString(parameterString)
			if err != nil {
				log.Fatal(err)
			}
			parameters[key] = value
		}
	}

	poolLabels := getAndValidatePoolLabels(workflowPoolLabelsKey)

	// Process the associated account: by default, we try to get from CI/CD environment variables
	associatedAccount := GetCIEnvironmentVariableAccount()
	if viper.IsSet(workflowAccountKey) {
		associatedAccount = viper.GetString(workflowAccountKey)
	}

	body := api.CreateWorkflowRunInput{
		BuildID:           api.BuildID(buildID),
		AssociatedAccount: &associatedAccount,
	}

	if len(parameters) > 0 {
		body.Parameters = &parameters
	}
	if len(poolLabels) > 0 {
		body.PoolLabels = &poolLabels
	}
	if viper.IsSet(workflowAllowableFailurePercentKey) {
		allowableFailurePercent := viper.GetInt(workflowAllowableFailurePercentKey)
		if allowableFailurePercent < 0 || allowableFailurePercent > 100 {
			log.Fatal("allowable failure percent must be between 0 and 100")
		}
		body.AllowableFailurePercent = &allowableFailurePercent
	}

	resp, err := Client.CreateWorkflowRunWithResponse(context.Background(), projectID, wf.WorkflowID, body)
	if err != nil {
		log.Fatal("failed to run workflow:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to run workflow", resp.HTTPResponse, resp.Body)
	if resp.JSON201 == nil {
		log.Fatal("empty response")
	}
	run := *resp.JSON201
	fmt.Println("Created workflow run successfully!")
	fmt.Println("workflow run ID:", run.WorkflowRunID.String())
}

func listWorkflowRuns(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(workflowProjectKey))
	wf := actualGetWorkflow(projectID, viper.GetString(workflowKey), false)

	var pageToken *string = nil
	runs := []api.WorkflowRun{}
	for {
		response, err := Client.ListWorkflowRunsWithResponse(
			context.Background(), projectID, wf.WorkflowID, &api.ListWorkflowRunsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list workflow runs:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list workflow runs", response.HTTPResponse, response.Body)
		if response.JSON200 == nil || response.JSON200.WorkflowRuns == nil {
			break
		}
		runs = append(runs, response.JSON200.WorkflowRuns...)
		if response.JSON200.NextPageToken != "" {
			pageToken = &response.JSON200.NextPageToken
		} else {
			break
		}
	}

	if len(runs) == 0 {
		fmt.Println("no workflow runs")
		return
	}
	OutputJson(runs)
}

func getWorkflowRun(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(workflowProjectKey))
	wf := actualGetWorkflow(projectID, viper.GetString(workflowKey), false)

	runID, err := uuid.Parse(viper.GetString(workflowRunIDKey))
	if err != nil {
		log.Fatal("failed to parse run ID: ", err)
	}

	resp, err := Client.GetWorkflowRunWithResponse(context.Background(), projectID, wf.WorkflowID, api.WorkflowRunID(runID))
	if err != nil {
		log.Fatal("failed to get workflow run:", err)
	}
	ValidateResponse(http.StatusOK, "failed to get workflow run", resp.HTTPResponse, resp.Body)
	if resp.JSON200 == nil {
		log.Fatal("empty response")
	}
	if resp.JSON200.WorkflowRunTestSuites == nil {
		fmt.Println("no suite runs")
		return
	}
	OutputJson(resp.JSON200.WorkflowRunTestSuites)
}

func actualGetWorkflow(projectID uuid.UUID, workflowKeyRaw string, expectArchived bool) api.Workflow {
	if workflowKeyRaw == "" {
		log.Fatal("must specify the workflow name or ID")
	}

	if id, err := uuid.Parse(workflowKeyRaw); err == nil {
		resp, err := Client.GetWorkflowWithResponse(context.Background(), projectID, api.WorkflowID(id))
		if err != nil {
			log.Fatal("unable to retrieve workflow:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve workflow", resp.HTTPResponse, resp.Body)
		if resp.JSON200 == nil {
			log.Fatal("empty workflow response")
		}
		return *resp.JSON200
	}

	var pageToken *string = nil
	for {
		resp, err := Client.ListWorkflowsWithResponse(context.Background(), projectID, &api.ListWorkflowsParams{
			PageToken: pageToken,
			OrderBy:   Ptr("timestamp"),
			Archived:  Ptr(expectArchived),
		})
		if err != nil {
			log.Fatal("unable to list workflows:", err)
		}
		ValidateResponse(http.StatusOK, "unable to list workflows", resp.HTTPResponse, resp.Body)
		if resp.JSON200 == nil || resp.JSON200.Workflows == nil {
			break
		}
		for _, wf := range resp.JSON200.Workflows {
			if wf.Name == workflowKeyRaw {
				return wf
			}
		}
		if resp.JSON200.NextPageToken != "" {
			pageToken = &resp.JSON200.NextPageToken
		} else {
			if expectArchived {
				log.Fatal("unable to find archived workflow: ", workflowKeyRaw)
			} else {
				log.Fatal("unable to find workflow: ", workflowKeyRaw)
			}
		}
	}

	log.Fatal("unable to find workflow: ", workflowKeyRaw)
	return api.Workflow{}
}
