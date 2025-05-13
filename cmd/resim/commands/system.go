package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	systemCmd = &cobra.Command{
		Use:     "systems",
		Short:   "systems contains commands for creating and managing systems",
		Long:    ``,
		Aliases: []string{"system"},
	}
	createSystemCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new system",
		Long:  ``,
		Run:   createSystem,
	}
	updateSystemCmd = &cobra.Command{
		Use:   "update",
		Short: "update - Update an existing system",
		Long:  ``,
		Run:   updateSystem,
	}
	getSystemCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Return details of a system",
		Long:  ``,
		Run:   getSystem,
	}
	archiveSystemCmd = &cobra.Command{
		Use:   "archive",
		Short: "archive - Archive a system",
		Long:  ``,
		Run:   archiveSystem,
	}
	listSystemsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists existing systems",
		Long:  ``,
		Run:   listSystems,
	}
	systemsBuildsCmd = &cobra.Command{
		Use:   "builds",
		Short: "builds - Lists builds of a system",
		Long:  ``,
		Run:   systemBuilds,
	}
	systemsExperiencesCmd = &cobra.Command{
		Use:   "experiences",
		Short: "experiences - Lists experiences compatible with a system",
		Long:  ``,
		Run:   systemExperiences,
	}
	systemsMetricsBuildsCmd = &cobra.Command{
		Use:   "metrics-builds",
		Short: "metrics-builds - Lists metrics builds compatible with a system",
		Long:  ``,
		Run:   systemMetricsBuilds,
	}
)

const (
	systemNameKey                       = "name"
	systemDescriptionKey                = "description"
	systemBuildVCPUsKey                 = "build-vcpus"
	systemMetricsBuildVCPUsKey          = "metrics-build-vcpus"
	systemBuildGPUsKey                  = "build-gpus"
	systemMetricsBuildGPUsKey           = "metrics-build-gpus"
	systemBuildMemoryMiBKey             = "build-memory-mib"
	systemMetricsBuildMemoryMibKey      = "metrics-build-memory-mib"
	systemBuildSharedMemoryMBKey        = "build-shared-memory-mb"
	systemMetricsBuildSharedMemoryMbKey = "metrics-build-shared-memory-mb"
	systemProjectKey                    = "project"
	systemKey                           = "system"
	systemGithubKey                     = "github"
	//Defaults:
	DefaultCPUs           = 4
	DefaultGPUs           = 0
	DefaultMemoryMiB      = 16384
	DefaultSharedMemoryMB = 64
)

func init() {
	createSystemCmd.Flags().String(systemProjectKey, "", "The name or ID of the project to create the system in")
	createSystemCmd.MarkFlagRequired(systemProjectKey)
	createSystemCmd.Flags().String(systemNameKey, "", "The name of the system, unique per project")
	createSystemCmd.MarkFlagRequired(systemNameKey)
	createSystemCmd.Flags().String(systemDescriptionKey, "", "The description of the system")
	createSystemCmd.MarkFlagRequired(systemDescriptionKey)
	createSystemCmd.Flags().Int(systemBuildVCPUsKey, DefaultCPUs, "The number of vCPUs required to execute the build (default: 4)")
	createSystemCmd.Flags().Int(systemMetricsBuildVCPUsKey, DefaultCPUs, "The number of vCPUs required to execute the metrics build (default: 4)")
	createSystemCmd.Flags().Int(systemBuildGPUsKey, DefaultGPUs, "The number of GPUs required to execute the build (default: 0)")
	createSystemCmd.Flags().Int(systemMetricsBuildGPUsKey, DefaultGPUs, "The number of GPUs required to execute the metrics build (default: 0)")
	createSystemCmd.Flags().Int(systemBuildMemoryMiBKey, DefaultMemoryMiB, "The amount of memory in MiB required to execute the build (default: 16384)")
	createSystemCmd.Flags().Int(systemMetricsBuildMemoryMibKey, DefaultMemoryMiB, "The amount of memory in MiB required to execute the metrics build (default: 16384)")
	createSystemCmd.Flags().Int(systemBuildSharedMemoryMBKey, DefaultSharedMemoryMB, "The amount of shared memory in MB required to execute the build (default: 64)")
	createSystemCmd.Flags().Int(systemMetricsBuildSharedMemoryMbKey, DefaultSharedMemoryMB, "The amount of shared memory in MB required to execute the metrics build (default: 64)")
	createSystemCmd.Flags().Bool(systemGithubKey, false, "Whether to output format in github action friendly format")
	createSystemCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	updateSystemCmd.Flags().String(systemProjectKey, "", "The name or ID of the project the system belongs to")
	updateSystemCmd.MarkFlagRequired(systemProjectKey)
	updateSystemCmd.Flags().String(systemKey, "", "The name or ID of the system to update")
	updateSystemCmd.MarkFlagRequired(systemKey)
	updateSystemCmd.Flags().String(systemNameKey, "", "New value for the system name")
	updateSystemCmd.Flags().String(systemDescriptionKey, "", "New value for the description of the system")
	updateSystemCmd.Flags().Int(systemBuildVCPUsKey, DefaultCPUs, "New value for the number of vCPUs required to execute the build")
	updateSystemCmd.Flags().Int(systemMetricsBuildVCPUsKey, DefaultCPUs, "New value for the number of vCPUs required to execute the metrics build")
	updateSystemCmd.Flags().Int(systemBuildGPUsKey, DefaultGPUs, "New value for the number of GPUs required to execute the build")
	updateSystemCmd.Flags().Int(systemMetricsBuildGPUsKey, DefaultGPUs, "New value for the number of GPUs required to execute the metrics build")
	updateSystemCmd.Flags().Int(systemBuildMemoryMiBKey, DefaultMemoryMiB, "New value for the amount of memory in MiB required to execute the build")
	updateSystemCmd.Flags().Int(systemMetricsBuildMemoryMibKey, DefaultMemoryMiB, "New value for the amount of memory in MiB required to execute the metrics build")
	updateSystemCmd.Flags().Int(systemBuildSharedMemoryMBKey, DefaultSharedMemoryMB, "New value for the amount of shared memory in MB required to execute the build")
	updateSystemCmd.Flags().Int(systemMetricsBuildSharedMemoryMbKey, DefaultSharedMemoryMB, "The amount of shared memory in MB required to execute the metrics build")
	updateSystemCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	getSystemCmd.Flags().String(systemProjectKey, "", "Get system associated with this project")
	getSystemCmd.MarkFlagRequired(systemProjectKey)
	getSystemCmd.Flags().String(systemKey, "", "The name or ID of the system to get details for")
	getSystemCmd.MarkFlagRequired(systemKey)
	getSystemCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	archiveSystemCmd.Flags().String(systemProjectKey, "", "System associated with this project")
	archiveSystemCmd.MarkFlagRequired(systemProjectKey)
	archiveSystemCmd.Flags().String(systemKey, "", "The name or ID of the system to delete")
	archiveSystemCmd.MarkFlagRequired(systemKey)
	archiveSystemCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	listSystemsCmd.Flags().String(systemProjectKey, "", "List systems associated with this project")
	listSystemsCmd.MarkFlagRequired(systemProjectKey)

	listSystemsCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	systemsBuildsCmd.Flags().String(systemProjectKey, "", "The project the system is associated with")
	systemsBuildsCmd.MarkFlagRequired(systemProjectKey)
	systemsBuildsCmd.Flags().String(systemKey, "", "The system whose builds to list")
	systemsBuildsCmd.MarkFlagRequired(systemKey)
	systemsBuildsCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	systemsExperiencesCmd.Flags().String(systemProjectKey, "", "The project the system is associated with")
	systemsExperiencesCmd.MarkFlagRequired(systemProjectKey)
	systemsExperiencesCmd.Flags().String(systemKey, "", "The system whose compatible experiences to list")
	systemsExperiencesCmd.MarkFlagRequired(systemKey)
	systemsExperiencesCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	systemsMetricsBuildsCmd.Flags().String(systemProjectKey, "", "The project the system is associated with")
	systemsMetricsBuildsCmd.MarkFlagRequired(systemProjectKey)
	systemsMetricsBuildsCmd.Flags().String(systemKey, "", "The system whose compatible metrics builds to list")
	systemsMetricsBuildsCmd.MarkFlagRequired(systemKey)
	systemsMetricsBuildsCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	systemCmd.AddCommand(createSystemCmd)
	systemCmd.AddCommand(updateSystemCmd)
	systemCmd.AddCommand(getSystemCmd)
	systemCmd.AddCommand(archiveSystemCmd)
	systemCmd.AddCommand(listSystemsCmd)
	systemCmd.AddCommand(systemsBuildsCmd)
	systemCmd.AddCommand(systemsExperiencesCmd)
	systemCmd.AddCommand(systemsMetricsBuildsCmd)

	rootCmd.AddCommand(systemCmd)
}

func getSystem(ccmd *cobra.Command, args []string) {
	var system *api.System
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))
	systemID := getSystemID(Client, projectID, viper.GetString(systemKey), true)
	response, err := Client.GetSystemWithResponse(context.Background(), projectID, systemID)
	if err != nil {
		log.Fatal("unable to retrieve system:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatal("failed to find system with requested id: ", projectID.String())
	} else {
		ValidateResponse(http.StatusOK, "unable to retrieve system", response.HTTPResponse, response.Body)
	}
	system = response.JSON200
	OutputJson(system)
}

func archiveSystem(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))
	systemID := getSystemID(Client, projectID, viper.GetString(systemKey), true)
	response, err := Client.ArchiveSystemWithResponse(context.Background(), projectID, systemID)
	if err != nil {
		log.Fatal("unable to archive system:", err)
	}
	if response.HTTPResponse.StatusCode == http.StatusNotFound {
		log.Fatal("failed to find system with requested id: ", systemID.String())
	} else {
		ValidateResponse(http.StatusNoContent, "unable to archive system", response.HTTPResponse, response.Body)
	}
	fmt.Println("Archived system successfully!")
}

func updateSystem(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))
	systemID := getSystemID(Client, projectID, viper.GetString(systemKey), true)
	updateSystemInput := api.UpdateSystemInput{}
	if viper.IsSet(systemNameKey) {
		updateSystemInput.Name = Ptr(viper.GetString(systemNameKey))
	}
	if viper.IsSet(systemDescriptionKey) {
		updateSystemInput.Description = Ptr(viper.GetString(systemDescriptionKey))
	}
	if viper.IsSet(systemBuildVCPUsKey) {
		updateSystemInput.BuildVcpus = Ptr(viper.GetInt(systemBuildVCPUsKey))
	}
	if viper.IsSet(systemMetricsBuildVCPUsKey) {
		updateSystemInput.MetricsBuildVcpus = Ptr(viper.GetInt(systemMetricsBuildVCPUsKey))
	}
	if viper.IsSet(systemBuildGPUsKey) {
		updateSystemInput.BuildGpus = Ptr(viper.GetInt(systemBuildGPUsKey))
	}
	if viper.IsSet(systemMetricsBuildGPUsKey) {
		updateSystemInput.MetricsBuildGpus = Ptr(viper.GetInt(systemMetricsBuildGPUsKey))
	}
	if viper.IsSet(systemBuildMemoryMiBKey) {
		updateSystemInput.BuildMemoryMib = Ptr(viper.GetInt(systemBuildMemoryMiBKey))
	}
	if viper.IsSet(systemMetricsBuildMemoryMibKey) {
		updateSystemInput.MetricsBuildMemoryMib = Ptr(viper.GetInt(systemMetricsBuildMemoryMibKey))
	}
	if viper.IsSet(systemBuildSharedMemoryMBKey) {
		updateSystemInput.BuildSharedMemoryMb = Ptr(viper.GetInt(systemBuildSharedMemoryMBKey))
	}
	if viper.IsSet(systemMetricsBuildSharedMemoryMbKey) {
		updateSystemInput.MetricsBuildSharedMemoryMb = Ptr(viper.GetInt(systemMetricsBuildSharedMemoryMbKey))
	}
	response, err := Client.UpdateSystemWithResponse(context.Background(), projectID, systemID, updateSystemInput)
	if err != nil {
		log.Fatal("unable to update system:", err)
	}
	ValidateResponse(http.StatusOK, "unable to update system", response.HTTPResponse, response.Body)
	fmt.Println("Updated system successfully!")
}

func listSystems(cmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))

	var pageToken *string = nil

	var allSystems []api.System

	for {
		response, err := Client.ListSystemsWithResponse(
			context.Background(), projectID, &api.ListSystemsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list systems:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list systems", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Systems == nil {
			log.Fatal("no systems")
		}
		allSystems = append(allSystems, *response.JSON200.Systems...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	OutputJson(allSystems)
}

func createSystem(cmd *cobra.Command, args []string) {
	if !viper.GetBool(systemGithubKey) {
		fmt.Println("Creating a system...")
	}
	// Parse the various arguments from command line
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))

	systemName := viper.GetString(systemNameKey)
	if systemName == "" {
		log.Fatal("empty system name")
	}

	systemDescription := viper.GetString(systemDescriptionKey)
	if systemDescription == "" {
		log.Fatal("empty system description")
	}

	body := api.CreateSystemInput{
		Name:                       systemName,
		Description:                systemDescription,
		BuildGpus:                  viper.GetInt(systemBuildGPUsKey),
		BuildVcpus:                 viper.GetInt(systemBuildVCPUsKey),
		BuildMemoryMib:             viper.GetInt(systemBuildMemoryMiBKey),
		BuildSharedMemoryMb:        viper.GetInt(systemBuildSharedMemoryMBKey),
		MetricsBuildVcpus:          viper.GetInt(systemMetricsBuildVCPUsKey),
		MetricsBuildGpus:           viper.GetInt(systemMetricsBuildGPUsKey),
		MetricsBuildMemoryMib:      viper.GetInt(systemMetricsBuildMemoryMibKey),
		MetricsBuildSharedMemoryMb: viper.GetInt(systemMetricsBuildSharedMemoryMbKey),
	}

	response, err := Client.CreateSystemWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("unable to create system: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create system", response.HTTPResponse, response.Body)
	if response.JSON201 == nil {
		log.Fatal("empty system returned")
	}
	system := *response.JSON201
	if system.SystemID == uuid.Nil {
		log.Fatal("no system ID")
	}

	// Report the results back to the user
	if viper.GetBool(systemGithubKey) {
		fmt.Printf("system_id=%s\n", system.SystemID.String())
	} else {
		fmt.Println("Created system successfully!")
		fmt.Printf("System ID: %s\n", system.SystemID.String())
	}
}

func systemExperiences(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))
	systemID := getSystemID(Client, projectID, viper.GetString(systemKey), true)
	var pageToken *string = nil

	var allExperiences []api.Experience

	for {
		response, err := Client.ListExperiencesForSystemWithResponse(
			context.Background(), projectID, systemID, &api.ListExperiencesForSystemParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list experiences for system:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list experiences for system", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Experiences == nil {
			log.Fatal("no experiences")
		}
		allExperiences = append(allExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	OutputJson(allExperiences)
}

func systemMetricsBuilds(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))
	systemID := getSystemID(Client, projectID, viper.GetString(systemKey), true)
	var pageToken *string = nil

	var allMetricsBuilds []api.MetricsBuild

	for {

		response, err := Client.ListMetricsBuildsWithResponse(
			context.Background(), projectID, &api.ListMetricsBuildsParams{
				PageSize:  Ptr(100),
				SystemID:  &systemID,
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list metrics builds for system:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list metrics builds for system", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.MetricsBuilds == nil {
			log.Fatal("no experiences")
		}
		allMetricsBuilds = append(allMetricsBuilds, response.JSON200.MetricsBuilds...)
		if *pageToken == "" {
			break
		}
	}
	OutputJson(allMetricsBuilds)
}

func systemBuilds(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(systemProjectKey))
	systemID := getSystemID(Client, projectID, viper.GetString(systemKey), true)
	var pageToken *string = nil

	var allBuilds []api.Build

	for {
		response, err := Client.ListBuildsForSystemWithResponse(
			context.Background(), projectID, systemID, &api.ListBuildsForSystemParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			log.Fatal("failed to list builds for system:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list builds for system", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Builds == nil {
			log.Fatal("no builds")
		}
		allBuilds = append(allBuilds, response.JSON200.Builds...)
		if *pageToken == "" {
			break
		}
	}
	OutputJson(allBuilds)
}

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func checkSystemID(client api.ClientWithResponsesInterface, projectID uuid.UUID, identifier string) uuid.UUID {
	// Page through systems until we find the one with either a name or an ID
	// that matches the identifier string.
	systemID := uuid.Nil
	// First try the assumption that identifier is a UUID.
	err := uuid.Validate(identifier)
	if err == nil {
		// The identifier is a uuid - but does it refer to an existing system?
		potentialSystemID := uuid.MustParse(identifier)
		response, _ := client.GetSystemWithResponse(context.Background(), projectID, potentialSystemID)
		if response.HTTPResponse.StatusCode == http.StatusOK {
			// System found with ID
			return potentialSystemID
		}
	}
	// If we're here then either the identifier is not a UUID or the UUID was not
	// found. Users could choose to name systems with UUIDs so regardless of how
	// we got here we now search for identifier as a string name.
	var pageToken *string = nil
pageLoop:
	for {
		response, err := client.ListSystemsWithResponse(
			context.Background(), projectID, &api.ListSystemsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list systems:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list systems", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		pageToken = response.JSON200.NextPageToken
		systems := *response.JSON200.Systems
		for _, system := range systems {
			if system.Name == "" {
				log.Fatal("system has no name")
			}
			if system.SystemID == uuid.Nil {
				log.Fatal("system ID is empty")
			}
			if system.Name == identifier {
				systemID = system.SystemID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return systemID
}

func getSystemID(client api.ClientWithResponsesInterface, projectID uuid.UUID, identifier string, failWhenNotFound bool) uuid.UUID {
	systemID := checkSystemID(client, projectID, identifier)
	if systemID == uuid.Nil && failWhenNotFound {
		log.Fatal("failed to find system with name or ID: ", identifier)
	}
	return systemID
}
