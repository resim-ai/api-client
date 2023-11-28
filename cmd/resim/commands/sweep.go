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
		Use:    "create",
		Short:  "create - Creates a new parameter sweep",
		Long:   ``,
		Run:    createSweep,
		PreRun: RegisterViperFlagsAndSetClient,
	}
	getSweepCmd = &cobra.Command{
		Use:    "get",
		Short:  "get - Retrieves a parameter sweep",
		Long:   ``,
		Run:    getSweep,
		PreRun: RegisterViperFlagsAndSetClient,
	}

	listSweepCmd = &cobra.Command{
		Use:    "list",
		Short:  "list - Lists all parameter sweeps",
		Long:   ``,
		Run:    listSweeps,
		PreRun: RegisterViperFlagsAndSetClient,
	}
)

const (
	sweepBuildIDKey          = "build-id"
	sweepExperiencesKey      = "experiences"
	sweepExperienceTagsKey   = "experience-tags"
	sweepIDKey               = "sweep-id"
	sweepNameKey             = "sweep-name"
	sweepMetricsBuildKey     = "metrics-build-id"
	sweepGridSearchConfigKey = "grid-search-config"
	sweepParameterNameKey    = "parameter-name"
	sweepParameterValuesKey  = "parameter-values"
	sweepExitStatusKey       = "exit-status"
	sweepGithubKey           = "github"
)

func init() {
	createSweepCmd.Flags().Bool(sweepGithubKey, false, "Whether to output format in github action friendly format")
	createSweepCmd.Flags().String(sweepBuildIDKey, "", "The ID of the build.")
	createSweepCmd.MarkFlagRequired(sweepBuildIDKey)
	createSweepCmd.Flags().String(sweepMetricsBuildKey, "", "The ID of the metrics build to use in this sweep.")
	createSweepCmd.Flags().String(sweepExperiencesKey, "", "List of experience names or list of experience IDs to run, comma-separated")
	createSweepCmd.Flags().String(sweepExperienceTagsKey, "", "List of experience tag names or list of experience tag IDs to run, comma-separated.")
	createSweepCmd.Flags().String(sweepGridSearchConfigKey, "", "Location of a json file listing parameter names and values to perform an exhaustive (combinatorial!) grid search. The json should be a list of objects with 'name' (parameter name) and 'values' (list of values to sample.)")
	createSweepCmd.Flags().String(sweepParameterNameKey, "", "The name of a single parameter to sweep.")
	createSweepCmd.Flags().StringSlice(sweepParameterValuesKey, []string{}, "A comma separated list of parameter values to sweep.")
	createSweepCmd.MarkFlagsMutuallyExclusive(sweepParameterNameKey, sweepGridSearchConfigKey)
	sweepCmd.AddCommand(createSweepCmd)

	getSweepCmd.Flags().String(sweepIDKey, "", "The ID of the sweep to retrieve.")
	getSweepCmd.Flags().String(sweepNameKey, "", "The name of the sweep to retrieve (e.g. rejoicing-aquamarine-starfish).")
	getSweepCmd.MarkFlagsMutuallyExclusive(sweepIDKey, sweepNameKey)
	getSweepCmd.Flags().Bool(sweepExitStatusKey, false, "If set, exit code corresponds to sweep status (1 = error, 0 = SUCCEEDED, 2=FAILED, 3=SUBMITTED, 4=RUNNING)")
	sweepCmd.AddCommand(getSweepCmd)

	sweepCmd.AddCommand(listSweepCmd)

	rootCmd.AddCommand(sweepCmd)
}

func createSweep(ccmd *cobra.Command, args []string) {
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

	// Build the request body
	body := api.CreateParameterSweepJSONRequestBody{
		BuildID:    &buildID,
		Parameters: &sweepParameters,
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

	// Make the request
	response, err := Client.CreateParameterSweepWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal("failed to create sweep:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create sweep", response.HTTPResponse)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	sweep := *response.JSON201

	if !sweepGithub {
		// Report the results back to the user
		fmt.Println("Created sweep successfully!")
	}
	if sweep.ParameterSweepID == nil {
		log.Fatal("empty ID")
	}
	if !sweepGithub {
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
	var sweep *api.ParameterSweep
	if viper.IsSet(sweepIDKey) {
		sweepID, err := uuid.Parse(viper.GetString(sweepIDKey))
		if err != nil {
			log.Fatal("unable to parse sweep ID: ", err)
		}
		response, err := Client.GetParameterSweepWithResponse(context.Background(), sweepID)
		if err != nil {
			log.Fatal("unable to retrieve sweep:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve sweep", response.HTTPResponse)
		sweep = response.JSON200
	} else if viper.IsSet(sweepNameKey) {
		sweepName := viper.GetString(sweepNameKey)
		var pageToken *string = nil
	pageLoop:
		for {
			response, err := Client.ListParameterSweepsWithResponse(context.Background(), &api.ListParameterSweepsParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
			if err != nil {
				log.Fatal("unable to list sweeps:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list sweeps", response.HTTPResponse)
			if response.JSON200.Sweeps == nil {
				log.Fatal("unable to find sweep: ", sweepName)
			}
			sweeps := *response.JSON200.Sweeps

			for _, b := range sweeps {
				if b.Name != nil && *b.Name == sweepName {
					sweep = &b
					break pageLoop
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

	if viper.GetBool(sweepExitStatusKey) {
		if sweep.Status == nil {
			log.Fatal("no status returned")
		}
		switch *sweep.Status {
		case api.ParameterSweepStatusSUCCEEDED:
			os.Exit(0)
		case api.ParameterSweepStatusFAILED:
			os.Exit(2)
		case api.ParameterSweepStatusSUBMITTED:
			os.Exit(3)
		case api.ParameterSweepStatusRUNNING:
			os.Exit(4)
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
	var pageToken *string = nil
	var allSweeps []api.ParameterSweep

	for {
		response, err := Client.ListParameterSweepsWithResponse(
			context.Background(), &api.ListParameterSweepsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list parameter sweeps:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list parameter sweeps", response.HTTPResponse)
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
