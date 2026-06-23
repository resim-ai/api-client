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
	mcapCmd = &cobra.Command{
		Use:     "mcap",
		Short:   "mcap contains commands for managing log ingestion",
		Long:    ``,
		Aliases: []string{"mcap"},
	}

	mcapCreateParserCmd = &cobra.Command{
		Use:   "create-parser",
		Short: "create-parser - Creates an mcap parser build",
		Long:  ``,
		Run:   mcapCreateParser,
	}

	mcapListParsersCmd = &cobra.Command{
		Use:   "list-parsers",
		Short: "list-parsers - Lists mcap parser builds",
		Long:  ``,
		Run:   mcapListParsers,
	}

	mcapIngestCmd = &cobra.Command{
		Use:   "ingest",
		Short: "ingest - Create mcap ingestion session",
		Long:  ``,
		Run:   mcapIngest,
	}
)

const (
	mcapParserSystemName            = "mcap-parser"
	mcapParserBranchName            = "mcap-parser-main"
	mcapParserMetricsBuildImageURI  = "public.ecr.aws/docker/library/hello-world:latest"
	mcapParserContainerTimeoutSecs  = int32(3600)
	mcapParserNameKey               = "name"
	mcapParserDescriptionKey        = "description"
	mcapProjectKey                  = "project"
	mcapParserImageURIKey           = "image"
	mcapIngestSessionNameKey        = "session-name"
	mcapIngestSessionDescriptionKey = "session-description"
	mcapIngestLocationKey           = "location"
	mcapIngestParserIDKey           = "parser"

	// mcapBatchSessionNameParameter is the key under which the session name is
	// passed to the parser container via batch parameters.
	mcapBatchSessionNameParameter = "session_name"
)

func init() {
	mcapCreateParserCmd.Flags().String(mcapParserNameKey, "", "The name of the parser")
	mcapCreateParserCmd.Flags().String(mcapParserDescriptionKey, "", "The description of the parser")
	mcapCreateParserCmd.MarkFlagRequired(mcapParserDescriptionKey)
	mcapCreateParserCmd.Flags().String(mcapProjectKey, "", "The name or ID of the project to create the parser in")
	mcapCreateParserCmd.MarkFlagRequired(mcapProjectKey)
	mcapCreateParserCmd.Flags().String(mcapParserImageURIKey, "", "The URI of the docker image")
	mcapCreateParserCmd.MarkFlagRequired(mcapParserImageURIKey)

	mcapListParsersCmd.Flags().String(mcapProjectKey, "", "The name or ID of the project to list the parsers from")
	mcapListParsersCmd.MarkFlagRequired(mcapProjectKey)

	mcapIngestCmd.Flags().String(mcapIngestSessionNameKey, "", "The name of the session")
	mcapIngestCmd.MarkFlagRequired(mcapIngestSessionNameKey)
	mcapIngestCmd.Flags().String(mcapProjectKey, "", "The name or ID of the project to run the ingest batch in")
	mcapIngestCmd.MarkFlagRequired(mcapProjectKey)
	mcapIngestCmd.Flags().String(mcapIngestSessionDescriptionKey, "", "The description of the session")
	mcapIngestCmd.MarkFlagRequired(mcapIngestSessionDescriptionKey)
	mcapIngestCmd.Flags().String(mcapIngestLocationKey, "", "The location containing MCAPs - same as experience location")
	mcapIngestCmd.MarkFlagRequired(mcapIngestLocationKey)
	mcapIngestCmd.Flags().String(mcapIngestParserIDKey, "", "The ID of the parser (build) to run against the session")
	mcapIngestCmd.MarkFlagRequired(mcapIngestParserIDKey)

	mcapCmd.AddCommand(mcapCreateParserCmd)
	mcapCmd.AddCommand(mcapListParsersCmd)
	mcapCmd.AddCommand(mcapIngestCmd)

	rootCmd.AddCommand(mcapCmd)
}

func mcapCreateParser(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(mcapProjectKey))

	buildDescription := viper.GetString(mcapParserDescriptionKey)
	buildName := viper.GetString(mcapParserNameKey)
	if buildName == "" {
		buildName = buildDescription
	}

	inputBuildImageURI := viper.GetString(mcapParserImageURIKey)
	if _, err := name.ParseReference(inputBuildImageURI, name.StrictValidation); err != nil {
		log.Fatal("failed to parse the image URI - it must be a valid docker image URI, including tag or digest")
	}

	systemID := getOrCreateMcapParserSystem(projectID)
	branchID := getOrCreateBranchID(Client, projectID, mcapParserBranchName, false)

	buildBody := api.CreateBuildForBranchInput{
		Name:        &buildName,
		Description: &buildDescription,
		ImageUri:    Ptr(inputBuildImageURI),
		Version:     "1",
		SystemID:    systemID,
	}

	buildResponse, err := Client.CreateBuildForBranchWithResponse(context.Background(), projectID, branchID, buildBody)
	if err != nil {
		log.Fatal("unable to create build: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create build", buildResponse.HTTPResponse, buildResponse.Body)
	if buildResponse.JSON201 == nil {
		log.Fatal("empty build response")
	}
	build := *buildResponse.JSON201
	if build.BuildID == uuid.Nil {
		log.Fatal("no build ID")
	}

	fmt.Println("Created mcap parser successfully!")
	fmt.Printf("Parser ID: %s\n", build.BuildID.String())
}

// getOrCreateMcapParserSystem returns the project's shared mcap-parser system,
// bootstrapping it (and a no-op metrics build registered with it) on first use.
func getOrCreateMcapParserSystem(projectID uuid.UUID) uuid.UUID {
	if systemID := getSystemID(Client, projectID, mcapParserSystemName, false); systemID != uuid.Nil {
		return systemID
	}

	systemBody := api.CreateSystemInput{
		Name:                       mcapParserSystemName,
		Description:                "Shared system that scopes mcap parser builds",
		BuildGpus:                  0,
		BuildVcpus:                 4,
		BuildMemoryMib:             16384,
		BuildSharedMemoryMb:        64,
		MetricsBuildVcpus:          1,
		MetricsBuildGpus:           0,
		MetricsBuildMemoryMib:      512,
		MetricsBuildSharedMemoryMb: 64,
	}
	systemResponse, err := Client.CreateSystemWithResponse(context.Background(), projectID, systemBody)
	if err != nil {
		log.Fatal("unable to create system: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create system", systemResponse.HTTPResponse, systemResponse.Body)
	if systemResponse.JSON201 == nil {
		log.Fatal("empty system response")
	}
	systemID := systemResponse.JSON201.SystemID
	if systemID == uuid.Nil {
		log.Fatal("no system ID")
	}

	// Batches require a metrics build registered with the system. We don't
	// actually compute metrics for ingestion, so register a no-op image once
	// at system-bootstrap time.
	metricsBuildBody := api.CreateMetricsBuildInput{
		Name:     mcapParserSystemName + "-metrics",
		ImageUri: mcapParserMetricsBuildImageURI,
		Version:  "1",
	}
	metricsBuildResponse, err := Client.CreateMetricsBuildWithResponse(context.Background(), projectID, metricsBuildBody)
	if err != nil {
		log.Fatal("unable to create metrics build: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create metrics build", metricsBuildResponse.HTTPResponse, metricsBuildResponse.Body)
	if metricsBuildResponse.JSON201 == nil {
		log.Fatal("empty metrics build response")
	}
	metricsBuildID := metricsBuildResponse.JSON201.MetricsBuildID
	if metricsBuildID == uuid.Nil {
		log.Fatal("no metrics build ID")
	}

	if _, err := Client.AddSystemToMetricsBuildWithResponse(context.Background(), projectID, systemID, metricsBuildID); err != nil {
		log.Fatal("failed to register metrics build with system: ", err)
	}

	return systemID
}

func mcapListParsers(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(mcapProjectKey))
	systemID := getSystemID(Client, projectID, mcapParserSystemName, true)
	OutputJson(listBuildsBySystem(projectID, systemID))
}

func mcapIngest(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(mcapProjectKey))
	sessionName := viper.GetString(mcapIngestSessionNameKey)
	sessionDescription := viper.GetString(mcapIngestSessionDescriptionKey)
	location := viper.GetString(mcapIngestLocationKey)

	// Fail fast if the parser system isn't bootstrapped yet.
	systemID := getSystemID(Client, projectID, mcapParserSystemName, true)

	buildID, err := uuid.Parse(viper.GetString(mcapIngestParserIDKey))
	if err != nil || buildID == uuid.Nil {
		log.Fatal("failed to parse parser ID (expected build UUID): ", err)
	}

	// Reuse an existing experience by name if one exists, otherwise create one.
	experienceID := getExperienceID(Client, projectID, sessionName, false, false)
	if experienceID == uuid.Nil {
		experienceBody := api.CreateExperienceInput{
			Name:                    sessionName,
			Description:             sessionDescription,
			Locations:               &[]string{location},
			ContainerTimeoutSeconds: Ptr(mcapParserContainerTimeoutSecs),
			SystemIDs:               &[]api.SystemID{systemID},
			CacheExempt:             Ptr(true),
		}
		experienceResponse, err := Client.CreateExperienceWithResponse(context.Background(), projectID, experienceBody)
		if err != nil {
			log.Fatal("failed to create experience: ", err)
		}
		ValidateResponse(http.StatusCreated, "failed to create experience", experienceResponse.HTTPResponse, experienceResponse.Body)
		if experienceResponse.JSON201 == nil {
			log.Fatal("empty experience response")
		}
		experienceID = experienceResponse.JSON201.ExperienceID
		if experienceID == uuid.Nil {
			log.Fatal("no experience ID")
		}
	}

	associatedAccount := GetCIEnvironmentVariableAccount()
	batchBody := api.BatchInput{
		BuildID:           Ptr(buildID),
		ExperienceIDs:     &[]api.ExperienceID{experienceID},
		BatchName:         Ptr(sessionName),
		AssociatedAccount: &associatedAccount,
		TriggeredVia:      DetermineTriggerMethod(),
		Parameters: &api.BatchParameters{
			mcapBatchSessionNameParameter: sessionName,
		},
	}

	batchResponse, err := Client.CreateBatchWithResponse(context.Background(), projectID, batchBody)
	if err != nil {
		log.Fatal("unable to create batch: ", err)
	}
	ValidateResponse(http.StatusCreated, "unable to create batch", batchResponse.HTTPResponse, batchResponse.Body)
	if batchResponse.JSON201 == nil {
		log.Fatal("empty batch response")
	}
	batch := *batchResponse.JSON201
	if batch.BatchID == nil {
		log.Fatal("no batch ID")
	}

	fmt.Println("Created mcap ingestion session successfully!")
	fmt.Printf("Experience ID: %s\n", experienceID.String())
	fmt.Printf("Batch ID: %s\n", batch.BatchID.String())
}
