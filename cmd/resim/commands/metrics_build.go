package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	metricsBuildCmd = &cobra.Command{
		Use:     "metrics-builds",
		Short:   "metrics-builds contains commands for creating and managing metrics builds",
		Long:    ``,
		Aliases: []string{"metricsBuild", "metricsBuilds", "metricBuild", "metricBuilds", "metrics-build", "metric-build", "metric-builds"},
	}
	createMetricsBuildCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new metrics build",
		Long:  ``,
		Run:   createMetricsBuild,
	}
	listMetricsBuildsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists existing metrics builds",
		Long:  ``,
		Run:   listMetricsBuilds,
	}

	addSystemMetricsBuildCmd = &cobra.Command{
		Use:   "add-system",
		Short: "add-system - Add a system as compatible with an metrics build",
		Long:  ``,
		Run:   addSystemToMetricsBuild,
	}
	removeSystemMetricsBuildCmd = &cobra.Command{
		Use:   "remove-system",
		Short: "remove-system - Remove a system as compatible with an metrics build",
		Long:  ``,
		Run:   removeSystemFromMetricsBuild,
	}
)

const (
	metricsBuildProjectKey  = "project"
	metricsBuildSystemKey   = "system"
	metricsBuildSystemsKey  = "systems"
	metricsBuildKey         = "metrics-build-id"
	metricsBuildNameKey     = "name"
	metricsBuildImageURIKey = "image"
	metricsBuildVersionKey  = "version"
	metricsBuildGithubKey   = "github"
)

func init() {
	createMetricsBuildCmd.Flags().String(metricsBuildProjectKey, "", "The name or ID of the project to associate with the metrics build")
	createMetricsBuildCmd.MarkFlagRequired(metricsBuildProjectKey)
	createMetricsBuildCmd.Flags().String(metricsBuildNameKey, "", "The name of the metrics build")
	createMetricsBuildCmd.MarkFlagRequired(metricsBuildNameKey)
	createMetricsBuildCmd.Flags().String(metricsBuildImageURIKey, "", "The URI of the docker image, including the tag")
	createMetricsBuildCmd.MarkFlagRequired(metricsBuildImageURIKey)
	createMetricsBuildCmd.Flags().String(metricsBuildVersionKey, "", "The version of the metrics build image, usually a commit ID or tag")
	createMetricsBuildCmd.MarkFlagRequired(metricsBuildVersionKey)
	createMetricsBuildCmd.Flags().StringSlice(metricsBuildSystemsKey, []string{}, "A list of system names or IDs to register as compatible with the metrics build")
	createMetricsBuildCmd.Flags().Bool(metricsBuildGithubKey, false, "Whether to output format in github action friendly format")

	metricsBuildCmd.AddCommand(createMetricsBuildCmd)
	listMetricsBuildsCmd.Flags().String(metricsBuildProjectKey, "", "The name or ID of the project to list the metrics builds within")
	listMetricsBuildsCmd.MarkFlagRequired(metricsBuildProjectKey)
	metricsBuildCmd.AddCommand(listMetricsBuildsCmd)

	// Systems-related sub-commands:
	addSystemMetricsBuildCmd.Flags().String(metricsBuildProjectKey, "", "The name or ID of the associated project")
	addSystemMetricsBuildCmd.MarkFlagRequired(metricsBuildProjectKey)
	addSystemMetricsBuildCmd.Flags().String(metricsBuildSystemKey, "", "The name or ID of the system to add")
	addSystemMetricsBuildCmd.MarkFlagRequired(metricsBuildSystemKey)
	addSystemMetricsBuildCmd.Flags().String(metricsBuildKey, "", "The ID of the metrics build register as compatible with the system")
	addSystemMetricsBuildCmd.MarkFlagRequired(metricsBuildKey)
	metricsBuildCmd.AddCommand(addSystemMetricsBuildCmd)
	removeSystemMetricsBuildCmd.Flags().String(metricsBuildProjectKey, "", "The name or ID of the associated project")
	removeSystemMetricsBuildCmd.MarkFlagRequired(metricsBuildProjectKey)
	removeSystemMetricsBuildCmd.Flags().String(metricsBuildSystemKey, "", "The name or ID of the system to remove")
	removeSystemMetricsBuildCmd.MarkFlagRequired(metricsBuildSystemKey)
	removeSystemMetricsBuildCmd.Flags().String(metricsBuildKey, "", "The ID of the metrics build to deregister as compatible with the system")
	removeSystemMetricsBuildCmd.MarkFlagRequired(metricsBuildKey)
	metricsBuildCmd.AddCommand(removeSystemMetricsBuildCmd)

	rootCmd.AddCommand(metricsBuildCmd)
}

func listMetricsBuilds(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(metricsBuildProjectKey))
	var pageToken *string = nil
	var allMetricsBuilds []api.MetricsBuild

	for {
		response, err := Client.ListMetricsBuildsWithResponse(
			context.Background(), projectID, &api.ListMetricsBuildsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list metrics builds:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list metrics builds", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.MetricsBuilds == nil {
			log.Fatal("no metrics builds")
		}
		allMetricsBuilds = append(allMetricsBuilds, response.JSON200.MetricsBuilds...)
		if *pageToken == "" {
			break
		}
	}

	OutputJson(allMetricsBuilds)
}

func createMetricsBuild(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(metricsBuildProjectKey))
	metricsBuildGithub := viper.GetBool(metricsBuildGithubKey)
	if !metricsBuildGithub {
		fmt.Println("Creating a metrics build...")
	}

	// Parse the various arguments from command line
	metricsBuildName := viper.GetString(metricsBuildNameKey)
	if metricsBuildName == "" {
		log.Fatal("empty metrics build name")
	}

	metricsBuildVersion := viper.GetString(metricsBuildVersionKey)
	if metricsBuildVersion == "" {
		log.Fatal("empty metrics build version")
	}

	metricsBuildImageURI := viper.GetString(metricsBuildImageURIKey)
	if metricsBuildImageURI == "" {
		log.Fatal("empty metrics build image URI")
	}
	// Validate that the image URI is valid:
	_, err := name.ParseReference(metricsBuildImageURI, name.StrictValidation)
	if err != nil {
		log.Fatal("failed to parse the image URI - it must be a valid docker image URI, including tag or digest")
	}

	body := api.CreateMetricsBuildInput{
		Name:     metricsBuildName,
		ImageUri: metricsBuildImageURI,
		Version:  metricsBuildVersion,
	}

	response, err := Client.CreateMetricsBuildWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("unable to create metrics build:", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create metrics build", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	metricsBuild := *response.JSON201
	if metricsBuild.MetricsBuildID == uuid.Nil {
		log.Fatal("no metrics build ID")
	}

	// For each system, add that system to the metrics build:
	systems := viper.GetStringSlice(metricsBuildSystemsKey)
	for _, systemName := range systems {
		systemID := getSystemID(Client, projectID, systemName, true)
		_, err := Client.AddSystemToMetricsBuildWithResponse(
			context.Background(), projectID,
			systemID,
			metricsBuild.MetricsBuildID,
		)
		if err != nil {
			log.Fatal("failed to register metrics build with system", err)
		}
	}

	// Report the results back to the user
	if metricsBuildGithub {
		fmt.Printf("metrics_build_id=%s\n", metricsBuild.MetricsBuildID.String())
	} else {
		fmt.Println("Created metrics build successfully!")
		fmt.Printf("Metrics Build ID: %s\n", metricsBuild.MetricsBuildID.String())
	}
}

func addSystemToMetricsBuild(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(metricsBuildProjectKey))

	systemName := viper.GetString(metricsBuildSystemKey)
	if systemName == "" {
		log.Fatal("empty system name")
	}
	systemID := getSystemID(Client, projectID, systemName, true)
	if viper.GetString(metricsBuildKey) == "" {
		log.Fatal("empty metrics build name")
	}

	// Validate the metrics build exists:
	metricsBuildIDString := viper.GetString(metricsBuildKey)
	err := uuid.Validate(metricsBuildIDString)
	if err != nil {
		log.Fatal("invalid metrics build ID")
	}
	metricsBuildID := uuid.MustParse(metricsBuildIDString)
	getResponse, _ := Client.GetMetricsBuildWithResponse(context.Background(), projectID, metricsBuildID)
	if !(getResponse.HTTPResponse.StatusCode == http.StatusOK) {
		log.Fatal("failed to find metrics build with id:", viper.GetString(metricsBuildKey))
	}

	response, err := Client.AddSystemToMetricsBuildWithResponse(
		context.Background(), projectID,
		systemID,
		metricsBuildID,
	)
	if err != nil {
		log.Fatal("failed to register metrics build with system", err)
	}
	if response.HTTPResponse.StatusCode == 409 {
		log.Fatal("failed to register metrics build with system, it may already be registered ", systemName)
	}
	ValidateResponse(http.StatusCreated, "failed to register metrics build with system", response.HTTPResponse, response.Body)
}

func removeSystemFromMetricsBuild(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(metricsBuildProjectKey))

	systemName := viper.GetString(metricsBuildSystemKey)
	if systemName == "" {
		log.Fatal("empty system name")
	}
	systemID := getSystemID(Client, projectID, systemName, true)
	if viper.GetString(metricsBuildKey) == "" {
		log.Fatal("empty metrics build name")
	}

	// Validate the metrics build exists:
	metricsBuildIDString := viper.GetString(metricsBuildKey)
	err := uuid.Validate(metricsBuildIDString)
	if err != nil {
		log.Fatal("invalid metrics build ID")
	}
	metricsBuildID := uuid.MustParse(metricsBuildIDString)
	getResponse, _ := Client.GetMetricsBuildWithResponse(context.Background(), projectID, metricsBuildID)
	if !(getResponse.HTTPResponse.StatusCode == http.StatusOK) {
		log.Fatal("failed to find metrics build with id:", viper.GetString(metricsBuildKey))
	}

	response, err := Client.RemoveSystemFromMetricsBuildWithResponse(
		context.Background(), projectID,
		systemID,
		metricsBuildID,
	)
	if err != nil {
		log.Fatal("failed to deregister metrics build with system", err)
	}
	if response.HTTPResponse.StatusCode == 409 {
		log.Fatal("failed to deregister metrics build with system, it may not be registered ", systemName)
	}
	ValidateResponse(http.StatusNoContent, "failed to deregister metrics build with system", response.HTTPResponse, response.Body)
}
