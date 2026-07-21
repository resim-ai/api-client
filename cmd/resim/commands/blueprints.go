package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"text/tabwriter"

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
		Short: "create - Creates a new blueprint. Fails if one with the same name already exists; use `revise` to add a version.",
		Long:  ``,
		Run:   createBlueprint,
	}

	reviseBlueprintCmd = &cobra.Command{
		Use:   "revise",
		Short: "revise - Creates a new version of an existing blueprint. Fails if no blueprint with the name exists; use `create` to make one.",
		Long:  ``,
		Run:   reviseBlueprint,
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
	blueprintJSONKey    = "json"
	blueprintAllKey     = "all-versions"
)

const blueprintListHeader = "NAME\tVERSION\tCREATED AT\n"

func init() {
	// Create Blueprint
	createBlueprintCmd.Flags().String(blueprintNameKey, "", "The name of the blueprint.")
	createBlueprintCmd.MarkFlagRequired(blueprintNameKey)
	createBlueprintCmd.Flags().String(blueprintCueFileKey, "", "Path to a file containing the CUE content for the blueprint.")
	createBlueprintCmd.MarkFlagRequired(blueprintCueFileKey)
	blueprintCmd.AddCommand(createBlueprintCmd)

	// Revise Blueprint
	reviseBlueprintCmd.Flags().String(blueprintNameKey, "", "The name of the existing blueprint to revise.")
	reviseBlueprintCmd.MarkFlagRequired(blueprintNameKey)
	reviseBlueprintCmd.Flags().String(blueprintCueFileKey, "", "Path to a file containing the CUE content for the new version.")
	reviseBlueprintCmd.MarkFlagRequired(blueprintCueFileKey)
	blueprintCmd.AddCommand(reviseBlueprintCmd)

	// List Blueprints
	listBlueprintsCmd.Flags().Bool(blueprintJSONKey, false, "Output raw JSON instead of a table. The CUE content is omitted; use `blueprints get` to fetch it.")
	listBlueprintsCmd.Flags().Bool(blueprintAllKey, false, "List every version of each blueprint instead of only the most recent.")
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
	name, cueContent := readBlueprintInputs()

	// `create` only ever makes a brand-new blueprint. Creating a further version
	// of an existing one is `revise`'s job; refusing here guards against
	// accidentally spawning unwanted versions by re-running `create`. This is
	// stricter than the API, which would happily add a version.
	if blueprintExists(name) {
		log.Fatalf("blueprint %q already exists; use `resim blueprints revise` to create a new version", name)
	}

	fmt.Println("Creating a blueprint...")
	blueprint := writeBlueprint(name, cueContent)
	fmt.Println("Created blueprint successfully!")
	printBlueprintResult(blueprint)
}

func reviseBlueprint(ccmd *cobra.Command, args []string) {
	name, cueContent := readBlueprintInputs()

	// `revise` only ever adds a version to an existing blueprint. Use `create` to
	// make a brand-new one.
	if !blueprintExists(name) {
		log.Fatalf("blueprint %q does not exist; use `resim blueprints create` to create it", name)
	}

	fmt.Println("Revising blueprint...")
	blueprint := writeBlueprint(name, cueContent)
	fmt.Println("Revised blueprint successfully!")
	printBlueprintResult(blueprint)
}

// readBlueprintInputs reads and validates the --name and --cue-file flags shared
// by `create` and `revise`, returning the blueprint name and the CUE file's
// contents.
func readBlueprintInputs() (string, string) {
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
	return name, string(cueContent)
}

// blueprintExists reports whether a blueprint with the given name already exists
// in the caller's org. It fatals on any error other than a clean 404.
func blueprintExists(name string) bool {
	response, err := Client.GetLatestBlueprintWithResponse(context.Background(), name)
	if err != nil {
		log.Fatal("failed to check whether blueprint exists:", err)
	}
	if response.HTTPResponse != nil && response.HTTPResponse.StatusCode == http.StatusNotFound {
		return false
	}
	ValidateResponse(http.StatusOK, "failed to check whether blueprint exists", response.HTTPResponse, response.Body)
	return true
}

// writeBlueprint sends the create request. The API creates a new blueprint or,
// if one with this name already exists, a new version of it; `create` and
// `revise` gate which of those is allowed before calling here.
func writeBlueprint(name, cueContent string) api.Blueprint {
	body := api.CreateBlueprintInput{
		Name:       name,
		CueContent: cueContent,
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
	return blueprint
}

func printBlueprintResult(blueprint api.Blueprint) {
	fmt.Println("Blueprint ID:", blueprint.BlueprintID.String())
	fmt.Println("Blueprint Name:", blueprint.Name)
	fmt.Println("Blueprint Version:", blueprint.Version)
}

// blueprintSummary is the JSON shape emitted by `blueprints list`: a blueprint
// without its (potentially large) CUE content, which is available via
// `blueprints get`.
type blueprintSummary struct {
	BlueprintID uuid.UUID     `json:"blueprintID"`
	Name        string        `json:"name"`
	Version     int           `json:"version"`
	CreatedAt   api.Timestamp `json:"createdAt"`
	OrgID       api.OrgID     `json:"orgID"`
	UserID      api.UserID    `json:"userID"`
}

func listBlueprints(ccmd *cobra.Command, args []string) {
	blueprints := fetchAllBlueprints()

	// By default collapse to the most recent version of each blueprint name;
	// --all-versions shows every version.
	if !viper.GetBool(blueprintAllKey) {
		blueprints = latestBlueprintPerName(blueprints)
	}

	// Stable ordering: by name, then most-recent version first.
	sort.Slice(blueprints, func(i, j int) bool {
		if blueprints[i].Name != blueprints[j].Name {
			return blueprints[i].Name < blueprints[j].Name
		}
		return blueprints[i].Version > blueprints[j].Version
	})

	if viper.GetBool(blueprintJSONKey) {
		OutputJson(blueprintSummaries(blueprints))
		return
	}

	if len(blueprints) == 0 {
		fmt.Println("no blueprints")
		return
	}

	// Route the header and every row through a tabwriter so the tab-separated
	// columns are padded to a consistent width.
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
	fmt.Fprint(w, blueprintListHeader)
	for _, b := range blueprints {
		fmt.Fprintf(w, "%s\t%d\t%s\n", b.Name, b.Version, b.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	w.Flush()
}

// fetchAllBlueprints pages through every blueprint visible to the caller's org.
func fetchAllBlueprints() []api.Blueprint {
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

	return allBlueprints
}

// latestBlueprintPerName keeps only the highest-versioned blueprint for each
// distinct name.
func latestBlueprintPerName(blueprints []api.Blueprint) []api.Blueprint {
	latest := map[string]api.Blueprint{}
	for _, b := range blueprints {
		if existing, ok := latest[b.Name]; !ok || b.Version > existing.Version {
			latest[b.Name] = b
		}
	}
	result := make([]api.Blueprint, 0, len(latest))
	for _, b := range latest {
		result = append(result, b)
	}
	return result
}

func blueprintSummaries(blueprints []api.Blueprint) []blueprintSummary {
	summaries := make([]blueprintSummary, 0, len(blueprints))
	for _, b := range blueprints {
		summaries = append(summaries, blueprintSummary{
			BlueprintID: b.BlueprintID,
			Name:        b.Name,
			Version:     b.Version,
			CreatedAt:   b.CreatedAt,
			OrgID:       b.OrgID,
			UserID:      b.UserID,
		})
	}
	return summaries
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
