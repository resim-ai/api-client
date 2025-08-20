package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	sweepCmd = &cobra.Command{
		Use:     "sweeps",
		Short:   "sweeps contains commands for creating and managing parameter sweeps",
		Long:    ``,
		Aliases: []string{"sweep"},
	}
	createSweepCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new parameter sweep",
		Long:  ``,
		Run:   createSweep,
	}
	getSweepCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Retrieves a parameter sweep",
		Long:  ``,
		Run:   getSweep,
	}

	listSweepCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists all parameter sweeps",
		Long:  ``,
		Run:   listSweeps,
	}
	cancelSweepCmd = &cobra.Command{
		Use:   "cancel",
		Short: "cancel - Cancels a parameter sweep",
		Long:  ``,
		Run:   cancelSweep,
	}
)

const (
	sweepProjectKey          = "project"
	sweepBuildIDKey          = "build-id"
	sweepExperiencesKey      = "experiences"
	sweepExperienceTagsKey   = "experience-tags"
	sweepIDKey               = "sweep-id"
	sweepNameKey             = "sweep-name"
	sweepMetricsBuildKey     = "metrics-build-id"
	sweepMetricsSetKey       = "metrics-set"
	sweepGridSearchConfigKey = "grid-search-config"
	sweepParameterNameKey    = "parameter-name"
	sweepParameterValuesKey  = "parameter-values"
	sweepPoolLabelsKey       = "pool-labels"
	sweepExitStatusKey       = "exit-status"
	sweepGithubKey           = "github"
	sweepAccountKey          = "account"
)

func init() {
	createSweepCmd.Flags().Bool(sweepGithubKey, false, "Whether to output format in github action friendly format")
	createSweepCmd.Flags().String(sweepProjectKey, "", "The name or ID of the project to associate with the sweep")
	createSweepCmd.MarkFlagRequired(sweepProjectKey)
	createSweepCmd.Flags().String(sweepBuildIDKey, "", "The ID of the build.")
	createSweepCmd.MarkFlagRequired(sweepBuildIDKey)
	createSweepCmd.Flags().String(sweepMetricsBuildKey, "", "The ID of the metrics build to use in this sweep.")
	createSweepCmd.Flags().String(sweepMetricsSetKey, "", "The name of the metrics set to use in this sweep.")
	createSweepCmd.Flags().String(sweepExperiencesKey, "", "List of experience names or list of experience IDs to run, comma-separated")
	createSweepCmd.Flags().String(sweepExperienceTagsKey, "", "List of experience tag names or list of experience tag IDs to run, comma-separated.")
	createSweepCmd.Flags().String(sweepGridSearchConfigKey, "", "Location of a json file listing parameter names and values to perform an exhaustive (combinatorial!) grid search. The json should be a list of objects with 'name' (parameter name) and 'values' (list of values to sample.)")
	createSweepCmd.Flags().String(sweepParameterNameKey, "", "The name of a single parameter to sweep.")
	createSweepCmd.Flags().StringSlice(sweepParameterValuesKey, []string{}, "A comma separated list of parameter values to sweep.")
	createSweepCmd.Flags().StringSlice(sweepPoolLabelsKey, []string{}, "Pool labels to determine where to run this parameter sweep. Pool labels are interpreted as a logical AND. Accepts repeated labels or comma-separated labels.")
	createSweepCmd.MarkFlagsMutuallyExclusive(sweepParameterNameKey, sweepGridSearchConfigKey)
	createSweepCmd.Flags().String(sweepAccountKey, "", "Specify a username for a CI/CD platform account to associate with this parameter sweep.")
	sweepCmd.AddCommand(createSweepCmd)
	getSweepCmd.Flags().String(sweepProjectKey, "", "The name or ID of the project to get the sweep from")
	getSweepCmd.MarkFlagRequired(sweepProjectKey)
	getSweepCmd.Flags().String(sweepIDKey, "", "The ID of the sweep to retrieve.")
	getSweepCmd.Flags().String(sweepNameKey, "", "The name of the sweep to retrieve (e.g. rejoicing-aquamarine-starfish).")
	getSweepCmd.MarkFlagsMutuallyExclusive(sweepIDKey, sweepNameKey)
	getSweepCmd.Flags().Bool(sweepExitStatusKey, false, "If set, exit code corresponds to sweep status (1 = internal CLI error, 0 = SUCCEEDED, 2=ERROR, 3=SUBMITTED, 4=RUNNING, 5=CANCELLED)")
	sweepCmd.AddCommand(getSweepCmd)

	cancelSweepCmd.Flags().String(sweepProjectKey, "", "The name or ID of the project to cancel the sweep from")
	cancelSweepCmd.MarkFlagRequired(sweepProjectKey)
	cancelSweepCmd.Flags().String(sweepIDKey, "", "The ID of the sweep to cancel.")
	cancelSweepCmd.Flags().String(sweepNameKey, "", "The name of the sweep to cancel (e.g. rejoicing-aquamarine-starfish).")
	cancelSweepCmd.MarkFlagsMutuallyExclusive(sweepIDKey, sweepNameKey)
	sweepCmd.AddCommand(cancelSweepCmd)

	listSweepCmd.Flags().String(sweepProjectKey, "", "The name or ID of the project to list the sweeps within")
	listSweepCmd.MarkFlagRequired(sweepProjectKey)
	sweepCmd.AddCommand(listSweepCmd)

	rootCmd.AddCommand(sweepCmd)
}

func createSweep(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(sweepProjectKey))
	sweepGithub := viper.GetBool(sweepGithubKey)
	if !sweepGithub {
		fmt.Println("Creating a sweep...")
	}

	if viper.IsSet(sweepGridSearchConfigKey) && (viper.IsSet(sweepParameterNameKey) || viper.IsSet(sweepParameterValuesKey)) {
		log.Fatal("failed to create sweep: you cannot specify both a grid search config file location and a parameter name/values. For multi-dimensional sweeps, please use a config file.")
	}

	if !viper.IsSet(sweepGridSearchConfigKey) && !(viper.IsSet(sweepParameterNameKey) && viper.IsSet(sweepParameterValuesKey)) {
		log.Fatal("failed to create sweep: you must specify either a grid search config file location or a parameter name *and* values")
	}

	if !viper.IsSet(sweepExperiencesKey) && !viper.IsSet(sweepExperienceTagsKey) {
		log.Fatal("failed to create sweep: you must choose at least one experience or experience tag to run")
	}

	// Parse the grid search config file or parameter name and values
	sweepParameters := []api.SweepParameter{}
	if viper.IsSet(sweepGridSearchConfigKey) {
		configFile := viper.GetString(sweepGridSearchConfigKey)
		config, err := os.Open(configFile)
		if err != nil {
			log.Fatal("failed to open grid search config file: ", err)
		}
		defer config.Close()
		decoder := json.NewDecoder(config)
		err = decoder.Decode(&sweepParameters)
		if err != nil {
			log.Fatal("failed to parse grid search config file: ", err)
		}
	} else {
		// Parse the parameter name
		parameterName := viper.GetString(sweepParameterNameKey)

		// Parse the parameter values
		parameterValues := viper.GetStringSlice(sweepParameterValuesKey)
		if len(parameterValues) == 0 {
			log.Fatal("failed to create sweep: you must specify at least one parameter value")
		}

		sweepParameters = append(sweepParameters, api.SweepParameter{
			Name:   &parameterName,
			Values: &parameterValues,
		})
	}

	if !sweepGithub {
		fmt.Println("Sweep parameters:")
		for _, parameter := range sweepParameters {
			fmt.Printf("  %s: %s\n", *parameter.Name, *parameter.Values)
		}
	}

	// Parse the build ID
	buildID, err := uuid.Parse(viper.GetString(sweepBuildIDKey))
	if err != nil || buildID == uuid.Nil {
		log.Fatal("failed to parse build ID: ", err)
	}

	var allExperienceIDs []uuid.UUID
	var allExperienceNames []string

	// Parse --experiences into either IDs or names
	if viper.IsSet(sweepExperiencesKey) {
		experienceIDs, experienceNames := parseUUIDsAndNames(viper.GetString(sweepExperiencesKey))
		allExperienceIDs = append(allExperienceIDs, experienceIDs...)
		allExperienceNames = append(allExperienceNames, experienceNames...)
	}

	metricsBuildID := uuid.Nil
	if viper.IsSet(sweepMetricsBuildKey) {
		metricsBuildID, err = uuid.Parse(viper.GetString(sweepMetricsBuildKey))
		if err != nil {
			log.Fatal("failed to parse metrics-build ID: ", err)
		}
	}

	var allExperienceTagIDs []uuid.UUID
	var allExperienceTagNames []string

	// Parse --experience-tags
	if viper.IsSet(sweepExperienceTagsKey) {
		experienceTagIDs, experienceTagNames := parseUUIDsAndNames(viper.GetString(sweepExperienceTagsKey))
		allExperienceTagIDs = append(allExperienceTagIDs, experienceTagIDs...)
		allExperienceTagNames = append(allExperienceTagNames, experienceTagNames...)
	}

	// Parse --pool-labels (if any provided)
	poolLabels := []api.PoolLabel{}
	if viper.IsSet(sweepPoolLabelsKey) {
		poolLabels = viper.GetStringSlice(sweepPoolLabelsKey)
	}
	for i := range poolLabels {
		poolLabels[i] = strings.TrimSpace(poolLabels[i])
		if poolLabels[i] == "resim" {
			log.Fatal("failed to run sweep: resim is a reserved pool label")
		}
	}

	var metricsSet *string
	if viper.IsSet(sweepMetricsSetKey) {
		metricsSet = Ptr(viper.GetString(sweepMetricsSetKey))
		// Metrics 2.0 steps will only be run if we use the special pool
		// label, so let's enable it automatically if the user requested a
		// metrics set
		if len(poolLabels) == 0 {
			poolLabels = append(poolLabels, METRICS_2_POOL_LABEL)
		}
	}

	// Process the associated account: by default, we try to get from CI/CD environment variables
	// Otherwise, we use the account flag. The default is "".
	associatedAccount := GetCIEnvironmentVariableAccount()
	if viper.IsSet(sweepAccountKey) {
		associatedAccount = viper.GetString(sweepAccountKey)
	}

	// Build the request body
	body := api.ParameterSweepInput{
		BuildID:           &buildID,
		Parameters:        &sweepParameters,
		AssociatedAccount: &associatedAccount,
		TriggeredVia:      DetermineTriggerMethod(),
		MetricsSetName:    metricsSet,
	}

	if allExperienceIDs != nil {
		body.ExperienceIDs = &allExperienceIDs
	}

	if allExperienceNames != nil {
		body.ExperienceNames = &allExperienceNames
	}

	if allExperienceTagIDs != nil {
		body.ExperienceTagIDs = &allExperienceTagIDs
	}

	if allExperienceTagNames != nil {
		body.ExperienceTagNames = &allExperienceTagNames
	}

	if metricsBuildID != uuid.Nil {
		body.MetricsBuildID = &metricsBuildID
	}

	// Add the pool labels if any
	if len(poolLabels) > 0 {
		body.PoolLabels = &poolLabels
	}

	// Make the request
	response, err := Client.CreateParameterSweepWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create sweep:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create sweep", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	sweep := *response.JSON201

	if sweep.ParameterSweepID == nil {
		log.Fatal("empty ID")
	}
	if !sweepGithub {
		fmt.Println("Created sweep successfully!")
		fmt.Println("Sweep ID:", sweep.ParameterSweepID.String())
	} else {
		fmt.Printf("sweep_id=%s\n", sweep.ParameterSweepID.String())
	}
	if sweep.Name == nil {
		log.Fatal("empty name")
	}
	if !sweepGithub {
		fmt.Println("Sweep name:", *sweep.Name)
	}
	if sweep.Status == nil {
		log.Fatal("empty status")
	}
	if !sweepGithub {
		fmt.Println("Status:", *sweep.Status)
	}
}

func getSweep(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(sweepProjectKey))
	sweep := fetchSweep(projectID, viper.GetString(sweepIDKey), viper.GetString(sweepNameKey))

	if viper.GetBool(sweepExitStatusKey) {
		if sweep.Status == nil {
			log.Fatal("no status returned")
		}
		switch *sweep.Status {
		case api.ParameterSweepStatusSUCCEEDED:
			os.Exit(0)
		case api.ParameterSweepStatusERROR:
			os.Exit(2)
		case api.ParameterSweepStatusSUBMITTED:
			os.Exit(3)
		case api.ParameterSweepStatusRUNNING:
			os.Exit(4)
		case api.ParameterSweepStatusCANCELLED:
			os.Exit(5)
		default:
			log.Fatal("unknown sweep status: ", sweep.Status)
		}
	}

	bytes, err := json.MarshalIndent(sweep, "", "  ")
	if err != nil {
		log.Fatal("unable to serialize sweep: ", err)
	}
	fmt.Println(string(bytes))
}

func listSweeps(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(sweepProjectKey))
	var pageToken *string = nil
	var allSweeps []api.ParameterSweep

	for {
		response, err := Client.ListParameterSweepsWithResponse(
			context.Background(), projectID, &api.ListParameterSweepsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list parameter sweeps:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list parameter sweeps", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		pageToken = response.JSON200.NextPageToken
		allSweeps = append(allSweeps, *response.JSON200.Sweeps...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	OutputJson(allSweeps)
}

func cancelSweep(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(sweepProjectKey))
	sweep := fetchSweep(projectID, viper.GetString(sweepIDKey), viper.GetString(sweepNameKey))

	response, err := Client.CancelParameterSweepWithResponse(context.Background(), projectID, *sweep.ParameterSweepID)
	if err != nil {
		log.Fatal("failed to cancel sweep:", err)
	}
	ValidateResponse(http.StatusOK, "failed to cancel sweep", response.HTTPResponse, response.Body)
	fmt.Println("Sweep cancelled successfully!")
}

func fetchSweep(projectID uuid.UUID, sweepIDRaw string, sweepName string) *api.ParameterSweep {
	var sweep *api.ParameterSweep
	if sweepIDRaw != "" {
		sweepID, err := uuid.Parse(sweepIDRaw)
		if err != nil {
			log.Fatal("unable to parse sweep ID: ", err)
		}
		response, err := Client.GetParameterSweepWithResponse(context.Background(), projectID, sweepID)
		if err != nil {
			log.Fatal("unable to retrieve sweep:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve sweep", response.HTTPResponse, response.Body)
		sweep = response.JSON200
		return sweep
	} else if sweepName != "" {
		var pageToken *string = nil
		for {
			response, err := Client.ListParameterSweepsWithResponse(context.Background(), projectID, &api.ListParameterSweepsParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list sweeps:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list sweeps", response.HTTPResponse, response.Body)
			if response.JSON200.Sweeps == nil {
				log.Fatal("unable to find sweep: ", sweepName)
			}
			sweeps := *response.JSON200.Sweeps

			for _, b := range sweeps {
				if b.Name != nil && *b.Name == sweepName {
					sweep = &b
					return sweep
				}
			}

			if response.JSON200.NextPageToken != nil && *response.JSON200.NextPageToken != "" {
				pageToken = response.JSON200.NextPageToken
			} else {
				log.Fatal("unable to find sweep: ", sweepName)
			}
		}
	} else {
		log.Fatal("must specify either the sweep ID or the sweep name")
	}
	return sweep
}
