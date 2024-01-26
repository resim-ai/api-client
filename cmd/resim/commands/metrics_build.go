package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	metricsBuildCmd = &cobra.Command{
		Use:     "metrics-builds",
		Short:   "metrics-builds contains commands for creating and managing metrics builds",
		Long:    ``,
		Aliases: []string{"metricsBuild, metricsBuilds, metricBuild, metricBuilds, metrics-build, metric-build"},
	}
	createMetricsBuildCmd = &cobra.Command{
		Use:    "create",
		Short:  "create - Creates a new metrics build",
		Long:   ``,
		Run:    createMetricsBuild,
		PreRun: RegisterViperFlagsAndSetClient,
	}
	listMetricsBuildsCmd = &cobra.Command{
		Use:    "list",
		Short:  "list - Lists existing metrics builds",
		Long:   ``,
		Run:    listMetricsBuilds,
		PreRun: RegisterViperFlagsAndSetClient,
	}
)

const (
	metricsBuildNameKey     = "name"
	metricsBuildImageURIKey = "image"
	metricsBuildVersionKey  = "version"
	metricsBuildGithubKey   = "github"
)

func init() {
	createMetricsBuildCmd.Flags().String(metricsBuildNameKey, "", "The name of the metrics build")
	createMetricsBuildCmd.MarkFlagRequired(metricsBuildNameKey)
	createMetricsBuildCmd.Flags().String(metricsBuildImageURIKey, "", "The URI of the docker image, including the tag")
	createMetricsBuildCmd.MarkFlagRequired(metricsBuildImageURIKey)
	createMetricsBuildCmd.Flags().String(metricsBuildVersionKey, "", "The version of the metrics build image, usually a commit ID or tag")
	createMetricsBuildCmd.MarkFlagRequired(metricsBuildVersionKey)
	createMetricsBuildCmd.Flags().Bool(metricsBuildGithubKey, false, "Whether to output format in github action friendly format")

	metricsBuildCmd.AddCommand(createMetricsBuildCmd)
	metricsBuildCmd.AddCommand(listMetricsBuildsCmd)
	rootCmd.AddCommand(metricsBuildCmd)
}

func listMetricsBuilds(ccmd *cobra.Command, args []string) {
	var pageToken *string = nil
	var allMetricsBuilds []api.MetricsBuild

	for {
		response, err := Client.ListMetricsBuildsWithResponse(
			context.Background(), &api.ListMetricsBuildsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list metrics builds:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list metrics builds", response.HTTPResponse)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.MetricsBuilds == nil {
			log.Fatal("no metrics builds")
		}
		allMetricsBuilds = append(allMetricsBuilds, *response.JSON200.MetricsBuilds...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	OutputJson(allMetricsBuilds)
}

func createMetricsBuild(ccmd *cobra.Command, args []string) {
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

	body := api.CreateMetricsBuildJSONRequestBody{
		Name:     &metricsBuildName,
		ImageUri: &metricsBuildImageURI,
		Version:  &metricsBuildVersion,
	}

	response, err := Client.CreateMetricsBuildWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal("unable to create metrics build:", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create metrics build", response.HTTPResponse)
	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	metricsBuild := *response.JSON201
	if metricsBuild.MetricsBuildID == nil {
		log.Fatal("no metrics build ID")
	}

	// Report the results back to the user
	if metricsBuildGithub {
		fmt.Printf("metrics_build_id=%s\n", metricsBuild.MetricsBuildID.String())
	} else {
		fmt.Println("Created metrics build successfully!")
		fmt.Printf("Metrics Build ID: %s\n", metricsBuild.MetricsBuildID.String())
	}
}
