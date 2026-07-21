package commands

import (
	"context"
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
	blueprintCmd = &cobra.Command{
		Use:     "blueprints",
		Short:   "blueprints contains commands for creating and managing blueprints",
		Long:    ``,
		Aliases: []string{"blueprint"},
	}

	createBlueprintCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new blueprint, or a new version of an existing blueprint",
		Long:  ``,
		Run:   createBlueprint,
	}

	listBlueprintsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - List all the blueprints in the caller's org",
		Long:  ``,
		Run:   listBlueprints,
	}

	getBlueprintCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Retrieves a blueprint's latest version, or a specific version",
		Long:  ``,
		Run:   getBlueprint,
	}

	archiveBlueprintCmd = &cobra.Command{
		Use:   "archive",
		Short: "archive - Archive a blueprint, or a specific version of a blueprint",
		Long:  ``,
		Run:   archiveBlueprint,
	}
)

const (
	blueprintNameKey    = "name"
	blueprintCueFileKey = "cue-file"
	blueprintVersionKey = "version"
	blueprintCueOnlyKey = "cue-only"
)

func init() {
	// Create Blueprint
	createBlueprintCmd.Flags().String(blueprintNameKey, "", "The name of the blueprint.")
	createBlueprintCmd.MarkFlagRequired(blueprintNameKey)
	createBlueprintCmd.Flags().String(blueprintCueFileKey, "", "Path to a file containing the CUE content for the blueprint.")
	createBlueprintCmd.MarkFlagRequired(blueprintCueFileKey)
	blueprintCmd.AddCommand(createBlueprintCmd)

	// List Blueprints
	blueprintCmd.AddCommand(listBlueprintsCmd)

	// Get Blueprint
	getBlueprintCmd.Flags().String(blueprintNameKey, "", "The name of the blueprint to retrieve.")
	getBlueprintCmd.MarkFlagRequired(blueprintNameKey)
	getBlueprintCmd.Flags().Int(blueprintVersionKey, 0, "The specific version of the blueprint to retrieve. Defaults to the latest version.")
	getBlueprintCmd.Flags().Bool(blueprintCueOnlyKey, false, "Print only the blueprint's CUE content, unformatted, instead of the full JSON. Useful for writing the CUE to a file.")
	blueprintCmd.AddCommand(getBlueprintCmd)

	// Archive Blueprint
	archiveBlueprintCmd.Flags().String(blueprintNameKey, "", "The name of the blueprint to archive.")
	archiveBlueprintCmd.MarkFlagRequired(blueprintNameKey)
	archiveBlueprintCmd.Flags().Int(blueprintVersionKey, 0, "The specific version of the blueprint to archive. Defaults to archiving every version of the blueprint.")
	blueprintCmd.AddCommand(archiveBlueprintCmd)

	rootCmd.AddCommand(blueprintCmd)
}

func createBlueprint(ccmd *cobra.Command, args []string) {
	fmt.Println("Creating a blueprint...")

	name := viper.GetString(blueprintNameKey)
	if name == "" {
		log.Fatal("empty blueprint name")
	}

	cueFile := viper.GetString(blueprintCueFileKey)
	if cueFile == "" {
		log.Fatal("empty blueprint cue file")
	}
	cueContent, err := os.ReadFile(cueFile)
	if err != nil {
		log.Fatal("failed to read cue file: ", err)
	}

	body := api.CreateBlueprintInput{
		Name:       name,
		CueContent: string(cueContent),
	}

	response, err := Client.CreateBlueprintWithResponse(context.Background(), body)
	if err != nil {
		log.Fatal("failed to create blueprint:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create blueprint", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	blueprint := *response.JSON201
	if blueprint.BlueprintID == uuid.Nil {
		log.Fatal("empty blueprint ID")
	}

	fmt.Println("Created blueprint successfully!")
	fmt.Println("Blueprint ID:", blueprint.BlueprintID.String())
	fmt.Println("Blueprint Name:", blueprint.Name)
	fmt.Println("Blueprint Version:", blueprint.Version)
}

func listBlueprints(ccmd *cobra.Command, args []string) {
	var pageToken *string = nil
	allBlueprints := []api.Blueprint{}

	for {
		response, err := Client.ListBlueprintsWithResponse(
			context.Background(), &api.ListBlueprintsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list blueprints:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list blueprints", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		if response.JSON200.Blueprints != nil {
			allBlueprints = append(allBlueprints, *response.JSON200.Blueprints...)
		}
		pageToken = response.JSON200.NextPageToken
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	if len(allBlueprints) == 0 {
		fmt.Println("no blueprints")
		return
	}

	OutputJson(allBlueprints)
}

// actualGetBlueprint retrieves a blueprint by name. When version is nil the
// latest version is returned; otherwise the specific version is fetched.
func actualGetBlueprint(name string, version *int) *api.Blueprint {
	if name == "" {
		log.Fatal("must specify the blueprint name")
	}

	if version != nil {
		response, err := Client.GetBlueprintVersionWithResponse(context.Background(), name, *version)
		if err != nil {
			log.Fatal("unable to retrieve blueprint version:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve blueprint version", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		return response.JSON200
	}

	response, err := Client.GetLatestBlueprintWithResponse(context.Background(), name)
	if err != nil {
		log.Fatal("unable to retrieve blueprint:", err)
	}
	ValidateResponse(http.StatusOK, "unable to retrieve blueprint", response.HTTPResponse, response.Body)
	if response.JSON200 == nil {
		log.Fatal("empty response")
	}
	return response.JSON200
}

func getBlueprint(ccmd *cobra.Command, args []string) {
	var version *int
	if viper.IsSet(blueprintVersionKey) {
		version = Ptr(viper.GetInt(blueprintVersionKey))
	}
	blueprint := actualGetBlueprint(viper.GetString(blueprintNameKey), version)

	if viper.GetBool(blueprintCueOnlyKey) {
		// Print the CUE content verbatim (no added formatting) so it can be
		// redirected straight into a file.
		fmt.Print(blueprint.CueContent)
		return
	}

	OutputJson(blueprint)
}

func archiveBlueprint(ccmd *cobra.Command, args []string) {
	name := viper.GetString(blueprintNameKey)
	if name == "" {
		log.Fatal("must specify the blueprint name")
	}

	if viper.IsSet(blueprintVersionKey) {
		version := viper.GetInt(blueprintVersionKey)
		response, err := Client.ArchiveBlueprintVersionWithResponse(context.Background(), name, version)
		if err != nil {
			log.Fatal("failed to archive blueprint version:", err)
		}
		ValidateResponse(http.StatusNoContent, "failed to archive blueprint version", response.HTTPResponse, response.Body)
		fmt.Printf("Archived blueprint %q version %d successfully!\n", name, version)
		return
	}

	response, err := Client.ArchiveBlueprintWithResponse(context.Background(), name)
	if err != nil {
		log.Fatal("failed to archive blueprint:", err)
	}
	ValidateResponse(http.StatusNoContent, "failed to archive blueprint", response.HTTPResponse, response.Body)
	fmt.Printf("Archived blueprint %q successfully!\n", name)
}
