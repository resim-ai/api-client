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
	getSystemCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Return details of a system",
		Long:  ``,
		Run:   getSystem,
	}
	listSystemsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - Lists existing systems",
		Long:  ``,
		Run:   listSystems,
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

	getSystemCmd.Flags().String(systemProjectKey, "", "Get system associated with this project")
	getSystemCmd.MarkFlagRequired(systemProjectKey)
	getSystemCmd.Flags().String(systemKey, "", "The name or ID of the system to get details for")
	getSystemCmd.MarkFlagRequired(systemKey)

	listSystemsCmd.Flags().String(systemProjectKey, "", "List systems associated with this project")
	listSystemsCmd.MarkFlagRequired(systemProjectKey)

	listSystemsCmd.Flags().SetNormalizeFunc(AliasNormalizeFunc)

	systemCmd.AddCommand(createSystemCmd)
	systemCmd.AddCommand(getSystemCmd)
	systemCmd.AddCommand(listSystemsCmd)
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
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	enc.Encode(system)
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
			log.Fatal("no systemes")
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

	body := api.System{
		Name:                       &systemName,
		Description:                &systemDescription,
		BuildGpus:                  Ptr(viper.GetInt(systemBuildGPUsKey)),
		BuildVcpus:                 Ptr(viper.GetInt(systemBuildVCPUsKey)),
		BuildMemoryMib:             Ptr(viper.GetInt(systemBuildMemoryMiBKey)),
		BuildSharedMemoryMb:        Ptr(viper.GetInt(systemBuildSharedMemoryMBKey)),
		MetricsBuildVcpus:          Ptr(viper.GetInt(systemMetricsBuildVCPUsKey)),
		MetricsBuildGpus:           Ptr(viper.GetInt(systemMetricsBuildGPUsKey)),
		MetricsBuildMemoryMib:      Ptr(viper.GetInt(systemMetricsBuildMemoryMibKey)),
		MetricsBuildSharedMemoryMb: Ptr(viper.GetInt(systemMetricsBuildSharedMemoryMbKey)),
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
	if system.SystemID == nil {
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
			if system.Name == nil {
				log.Fatal("system has no name")
			}
			if system.SystemID == nil {
				log.Fatal("system ID is empty")
			}
			if *system.Name == identifier {
				systemID = *system.SystemID
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
