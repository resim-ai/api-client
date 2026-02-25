package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	assetCmd = &cobra.Command{
		Use:     "assets",
		Short:   "assets contains commands for creating and managing assets",
		Long:    ``,
		Aliases: []string{"asset"},
	}

	createAssetCmd = &cobra.Command{
		Use:   "create",
		Short: "create - Creates a new asset",
		Long:  ``,
		Run:   createAsset,
	}

	listAssetsCmd = &cobra.Command{
		Use:   "list",
		Short: "list - List all the assets associated with this project",
		Long:  ``,
		Run:   listAssets,
	}

	getAssetCmd = &cobra.Command{
		Use:   "get",
		Short: "get - Retrieves an asset's latest revision, all revisions, or a specific revision",
		Long:  ``,
		Run:   getAsset,
	}

	updateAssetCmd = &cobra.Command{
		Use:   "update",
		Short: "update - Update an asset's metadata (name or description) without creating a new revision",
		Long:  ``,
		Run:   updateAsset,
	}

	reviseAssetCmd = &cobra.Command{
		Use:   "revise",
		Short: "revise - Create a new revision of an asset with updated locations or version",
		Long:  ``,
		Run:   reviseAsset,
	}

	archiveAssetCmd = &cobra.Command{
		Use:   "archive",
		Short: "archive - Archive an asset",
		Long:  ``,
		Run:   archiveAsset,
	}

	restoreAssetCmd = &cobra.Command{
		Use:   "restore",
		Short: "restore - Restore an archived asset",
		Long:  ``,
		Run:   restoreAsset,
	}

	buildsForAssetCmd = &cobra.Command{
		Use:   "builds",
		Short: "builds - List the builds linked to this asset",
		Long:  ``,
		Run:   buildsForAsset,
	}
)

const (
	assetProjectKey      = "project"
	assetNameKey         = "name"
	assetDescriptionKey  = "description"
	assetLocationsKey    = "locations"
	assetMountFolderKey  = "mount-folder"
	assetVersionKey      = "version"
	assetCacheExemptKey  = "cache-exempt"
	assetKey             = "asset"
	assetRevisionKey     = "revision"
	assetAllRevisionsKey = "all-revisions"
	assetArchivedKey     = "archived"
)

func init() {
	// Create Asset
	createAssetCmd.Flags().String(assetProjectKey, "", "The name or ID of the project to associate with the asset.")
	createAssetCmd.MarkFlagRequired(assetProjectKey)
	createAssetCmd.Flags().String(assetNameKey, "", "The name of the asset.")
	createAssetCmd.MarkFlagRequired(assetNameKey)
	createAssetCmd.Flags().String(assetDescriptionKey, "", "The description of the asset.")
	createAssetCmd.MarkFlagRequired(assetDescriptionKey)
	createAssetCmd.Flags().String(assetLocationsKey, "", "Comma-separated list of location URIs for the asset.")
	createAssetCmd.MarkFlagRequired(assetLocationsKey)
	createAssetCmd.Flags().String(assetMountFolderKey, "", "The mount path in the container for the asset.")
	createAssetCmd.MarkFlagRequired(assetMountFolderKey)
	createAssetCmd.Flags().String(assetVersionKey, "", "The version of the asset.")
	createAssetCmd.MarkFlagRequired(assetVersionKey)
	createAssetCmd.Flags().Bool(assetCacheExemptKey, false, "If true, the asset will not be cached.")
	assetCmd.AddCommand(createAssetCmd)

	// List Assets
	listAssetsCmd.Flags().String(assetProjectKey, "", "The name or ID of the project to list assets for.")
	listAssetsCmd.MarkFlagRequired(assetProjectKey)
	listAssetsCmd.Flags().Bool(assetArchivedKey, false, "Include archived assets.")
	assetCmd.AddCommand(listAssetsCmd)

	// Get Asset
	getAssetCmd.Flags().String(assetProjectKey, "", "The name or ID of the project the asset is associated with.")
	getAssetCmd.MarkFlagRequired(assetProjectKey)
	getAssetCmd.Flags().String(assetKey, "", "The name or ID of the asset to retrieve.")
	getAssetCmd.MarkFlagRequired(assetKey)
	getAssetCmd.Flags().Int64(assetRevisionKey, -1, "The specific revision of an asset to retrieve.")
	getAssetCmd.Flags().Bool(assetAllRevisionsKey, false, "Supply this flag to list all revisions of the asset.")
	assetCmd.AddCommand(getAssetCmd)

	// Update Asset
	updateAssetCmd.Flags().String(assetProjectKey, "", "The name or ID of the project the asset is associated with.")
	updateAssetCmd.MarkFlagRequired(assetProjectKey)
	updateAssetCmd.Flags().String(assetKey, "", "The name or ID of the asset to update.")
	updateAssetCmd.MarkFlagRequired(assetKey)
	updateAssetCmd.Flags().String(assetNameKey, "", "A new name for the asset.")
	updateAssetCmd.Flags().String(assetDescriptionKey, "", "A new description for the asset.")
	updateAssetCmd.MarkFlagsOneRequired(assetNameKey, assetDescriptionKey)
	assetCmd.AddCommand(updateAssetCmd)

	// Revise Asset
	reviseAssetCmd.Flags().String(assetProjectKey, "", "The name or ID of the project to associate with the asset.")
	reviseAssetCmd.MarkFlagRequired(assetProjectKey)
	reviseAssetCmd.Flags().String(assetKey, "", "The name or ID of the asset to revise.")
	reviseAssetCmd.MarkFlagRequired(assetKey)
	reviseAssetCmd.Flags().String(assetLocationsKey, "", "Comma-separated list of new location URIs for the asset revision.")
	reviseAssetCmd.MarkFlagRequired(assetLocationsKey)
	reviseAssetCmd.Flags().String(assetVersionKey, "", "The new version for the asset revision.")
	reviseAssetCmd.MarkFlagRequired(assetVersionKey)
	reviseAssetCmd.Flags().String(assetMountFolderKey, "", "A new mount folder for the asset revision (optional, overrides existing).")
	assetCmd.AddCommand(reviseAssetCmd)

	// Archive Asset
	archiveAssetCmd.Flags().String(assetProjectKey, "", "The name or ID of the project the asset is associated with.")
	archiveAssetCmd.MarkFlagRequired(assetProjectKey)
	archiveAssetCmd.Flags().String(assetKey, "", "The name or ID of the asset to archive.")
	archiveAssetCmd.MarkFlagRequired(assetKey)
	assetCmd.AddCommand(archiveAssetCmd)

	// Restore Asset
	restoreAssetCmd.Flags().String(assetProjectKey, "", "The name or ID of the project to restore the asset within.")
	restoreAssetCmd.MarkFlagRequired(assetProjectKey)
	restoreAssetCmd.Flags().String(assetKey, "", "The name or ID of the asset to restore.")
	restoreAssetCmd.MarkFlagRequired(assetKey)
	assetCmd.AddCommand(restoreAssetCmd)

	// Builds for Asset
	buildsForAssetCmd.Flags().String(assetProjectKey, "", "The name or ID of the project the asset is associated with.")
	buildsForAssetCmd.MarkFlagRequired(assetProjectKey)
	buildsForAssetCmd.Flags().String(assetKey, "", "The name or ID of the asset to list builds for.")
	buildsForAssetCmd.MarkFlagRequired(assetKey)
	buildsForAssetCmd.Flags().Int64(assetRevisionKey, -1, "The specific revision of an asset to list builds for.")
	assetCmd.AddCommand(buildsForAssetCmd)

	rootCmd.AddCommand(assetCmd)
}

func createAsset(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))
	fmt.Println("Creating an asset...")

	name := viper.GetString(assetNameKey)
	if name == "" {
		log.Fatal("empty asset name")
	}

	description := viper.GetString(assetDescriptionKey)
	if description == "" {
		log.Fatal("empty asset description")
	}

	locationsRaw := viper.GetString(assetLocationsKey)
	if locationsRaw == "" {
		log.Fatal("empty asset locations")
	}
	locations := parseCommaSeparated(locationsRaw)

	mountFolder := viper.GetString(assetMountFolderKey)
	if mountFolder == "" {
		log.Fatal("empty asset mount folder")
	}

	version := viper.GetString(assetVersionKey)
	if version == "" {
		log.Fatal("empty asset version")
	}

	body := api.CreateAssetInput{
		Name:        name,
		Description: description,
		Locations:   locations,
		MountFolder: mountFolder,
		Version:     version,
	}

	if viper.IsSet(assetCacheExemptKey) {
		body.CacheExempt = Ptr(viper.GetBool(assetCacheExemptKey))
	}

	response, err := Client.CreateAssetWithResponse(context.Background(), projectID, body)
	if err != nil {
		log.Fatal("failed to create asset:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to create asset", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	asset := *response.JSON201

	fmt.Println("Created asset successfully!")
	if asset.AssetID == uuid.Nil {
		log.Fatal("empty ID")
	}
	fmt.Println("Asset ID:", asset.AssetID.String())
	fmt.Println("Asset Revision:", asset.AssetRevision)
}

func listAssets(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))

	var pageToken *string = nil
	allAssets := []api.Asset{}
	archived := viper.GetBool(assetArchivedKey)

	for {
		response, err := Client.ListAssetsWithResponse(
			context.Background(), projectID, &api.ListAssetsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
				Archived:  Ptr(archived),
			})
		if err != nil {
			log.Fatal("failed to list assets:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list assets", response.HTTPResponse, response.Body)

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || response.JSON200.Assets == nil {
			break
		}
		allAssets = append(allAssets, response.JSON200.Assets...)
		if *pageToken == "" {
			break
		}
	}

	if len(allAssets) == 0 {
		fmt.Println("no assets")
		return
	}

	OutputJson(allAssets)
}

func actualGetAsset(projectID uuid.UUID, assetKeyRaw string, revision *int64, expectArchived bool) *api.Asset {
	var asset *api.Asset
	if assetKeyRaw == "" {
		log.Fatal("must specify the asset name or ID")
	}

	assetID, err := uuid.Parse(assetKeyRaw)
	if err == nil {
		response, err := Client.GetAssetWithResponse(context.Background(), projectID, assetID)
		if err != nil {
			log.Fatal("unable to retrieve asset:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve asset", response.HTTPResponse, response.Body)
		asset = response.JSON200
	} else {
		var pageToken *string = nil
	pageLoop:
		for {
			response, err := Client.ListAssetsWithResponse(context.Background(), projectID, &api.ListAssetsParams{
				PageToken: pageToken,
				OrderBy:   Ptr("timestamp"),
				Archived:  Ptr(expectArchived),
				Name:      Ptr(assetKeyRaw),
			})
			if err != nil {
				log.Fatal("unable to list assets:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list assets", response.HTTPResponse, response.Body)
			if response.JSON200.Assets == nil {
				log.Fatal("unable to find asset: ", assetKeyRaw)
			}
			assets := response.JSON200.Assets

			for _, a := range assets {
				if a.Name == assetKeyRaw {
					asset = &a
					break pageLoop
				}
			}

			if response.JSON200.NextPageToken != "" {
				pageToken = &response.JSON200.NextPageToken
			} else {
				if expectArchived {
					log.Fatal("unable to find archived asset: ", assetKeyRaw)
				} else {
					log.Fatal("unable to find asset: ", assetKeyRaw)
				}
			}
		}
	}

	if asset != nil && revision != nil && *revision != asset.AssetRevision {
		response, err := Client.GetAssetRevisionWithResponse(context.Background(), projectID, asset.AssetID, *revision)
		if err != nil {
			log.Fatal("unable to retrieve asset revision:", err)
		}
		ValidateResponse(http.StatusOK, "unable to retrieve asset revision", response.HTTPResponse, response.Body)
		asset = response.JSON200
	}
	return asset
}

func getAsset(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))
	var revision *int64
	if viper.IsSet(assetRevisionKey) {
		revision = Ptr(viper.GetInt64(assetRevisionKey))
	}
	asset := actualGetAsset(projectID, viper.GetString(assetKey), revision, false)

	if viper.GetBool(assetAllRevisionsKey) {
		var pageToken *string = nil
		allRevisions := []api.Asset{}
		for {
			response, err := Client.ListAssetRevisionsWithResponse(context.Background(), projectID, asset.AssetID, &api.ListAssetRevisionsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
			if err != nil {
				log.Fatal("unable to list asset revisions:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list asset revisions", response.HTTPResponse, response.Body)
			if response.JSON200.Assets == nil {
				log.Fatal("unable to list asset revisions")
			}
			allRevisions = append(allRevisions, response.JSON200.Assets...)
			if response.JSON200.NextPageToken != "" {
				pageToken = &response.JSON200.NextPageToken
			} else {
				break
			}
		}
		OutputJson(allRevisions)
	} else {
		OutputJson(asset)
	}
}

func updateAsset(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))
	fmt.Println("Updating an asset...")

	existingAsset := actualGetAsset(projectID, viper.GetString(assetKey), nil, false)
	if existingAsset == nil {
		log.Fatal("unable to find asset")
	}

	updateRequest := api.UpdateAssetInput{}

	if viper.IsSet(assetNameKey) {
		updateRequest.Name = Ptr(viper.GetString(assetNameKey))
	}
	if viper.IsSet(assetDescriptionKey) {
		updateRequest.Description = Ptr(viper.GetString(assetDescriptionKey))
	}

	response, err := Client.UpdateAssetWithResponse(context.Background(), projectID, existingAsset.AssetID, updateRequest)
	if err != nil {
		log.Fatal("failed to update asset:", err)
	}
	ValidateResponse(http.StatusOK, "failed to update asset", response.HTTPResponse, response.Body)

	if response.JSON200 == nil {
		log.Fatal("empty response")
	}

	fmt.Println("Updated asset successfully!")
	OutputJson(*response.JSON200)
}

func reviseAsset(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))
	fmt.Println("Revising an asset...")

	existingAsset := actualGetAsset(projectID, viper.GetString(assetKey), nil, false)
	if existingAsset == nil {
		log.Fatal("unable to find asset")
	}

	locationsRaw := viper.GetString(assetLocationsKey)
	if locationsRaw == "" {
		log.Fatal("empty asset locations")
	}
	locations := parseCommaSeparated(locationsRaw)

	version := viper.GetString(assetVersionKey)
	if version == "" {
		log.Fatal("empty asset version")
	}

	reviseRequest := api.ReviseAssetInput{
		Locations: locations,
		Version:   version,
	}

	if viper.IsSet(assetMountFolderKey) {
		reviseRequest.MountFolder = Ptr(viper.GetString(assetMountFolderKey))
	}

	response, err := Client.ReviseAssetWithResponse(context.Background(), projectID, existingAsset.AssetID, reviseRequest)
	if err != nil {
		log.Fatal("failed to revise asset:", err)
	}
	ValidateResponse(http.StatusCreated, "failed to revise asset", response.HTTPResponse, response.Body)

	if response.JSON201 == nil {
		log.Fatal("empty response")
	}
	asset := *response.JSON201

	fmt.Println("Revised asset successfully!")
	if asset.AssetID == uuid.Nil {
		log.Fatal("empty ID")
	}
	fmt.Println("Asset ID:", asset.AssetID.String())
	fmt.Println("Asset Revision:", asset.AssetRevision)
}

func archiveAsset(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))
	asset := actualGetAsset(projectID, viper.GetString(assetKey), nil, false)

	response, err := Client.ArchiveAssetWithResponse(context.Background(), projectID, asset.AssetID)
	if err != nil {
		log.Fatal("failed to archive asset:", err)
	}
	ValidateResponse(http.StatusNoContent, "failed to archive asset", response.HTTPResponse, response.Body)
	fmt.Printf("Archived asset %s successfully!\n", viper.GetString(assetKey))
}

func restoreAsset(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))
	asset := actualGetAsset(projectID, viper.GetString(assetKey), nil, true)

	response, err := Client.RestoreAssetWithResponse(context.Background(), projectID, asset.AssetID)
	if err != nil {
		log.Fatal("failed to restore asset:", err)
	}
	ValidateResponse(http.StatusNoContent, "failed to restore asset", response.HTTPResponse, response.Body)
	fmt.Printf("Restored archived asset %s successfully!\n", viper.GetString(assetKey))
}

func buildsForAsset(ccmd *cobra.Command, args []string) {
	projectID := getProjectID(Client, viper.GetString(assetProjectKey))
	var revision *int64
	if viper.IsSet(assetRevisionKey) {
		revision = Ptr(viper.GetInt64(assetRevisionKey))
	}

	asset := actualGetAsset(projectID, viper.GetString(assetKey), revision, false)

	allBuilds := []api.AssetBuildReference{}
	if revision == nil {
		var pageToken *string = nil
		for {
			response, err := Client.ListBuildsForAssetWithResponse(context.Background(), projectID, asset.AssetID, &api.ListBuildsForAssetParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
			if err != nil {
				log.Fatal("unable to list builds for asset:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list builds for asset", response.HTTPResponse, response.Body)
			allBuilds = append(allBuilds, response.JSON200.Builds...)

			if response.JSON200.NextPageToken != "" {
				pageToken = &response.JSON200.NextPageToken
			} else {
				break
			}
		}
	} else {
		var pageToken *string = nil
		for {
			response, err := Client.ListBuildsForAssetRevisionWithResponse(context.Background(), projectID, asset.AssetID, *revision, &api.ListBuildsForAssetRevisionParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
			if err != nil {
				log.Fatal("unable to list builds for asset revision:", err)
			}
			ValidateResponse(http.StatusOK, "unable to list builds for asset revision", response.HTTPResponse, response.Body)
			allBuilds = append(allBuilds, response.JSON200.Builds...)

			if response.JSON200.NextPageToken != "" {
				pageToken = &response.JSON200.NextPageToken
			} else {
				break
			}
		}
	}
	OutputJson(allBuilds)
}

// resolveAssetReferences parses a comma-separated list of asset references and resolves
// each to a concrete (AssetID, AssetRevision) pair. Each entry can be:
//   - "name" or "uuid" -- resolves to the latest revision
//   - "name:revision" or "uuid:revision" -- pins to the specified revision
func resolveAssetReferences(client api.ClientWithResponsesInterface, projectID uuid.UUID, assetRefs string) []api.AssetBuildLinkInput {
	entries := parseCommaSeparated(assetRefs)
	var links []api.AssetBuildLinkInput

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		identifier, revisionPtr := parseAssetRef(entry)

		asset := actualGetAsset(projectID, identifier, nil, false)
		if asset == nil {
			log.Fatal("unable to resolve asset: ", identifier)
		}

		rev := asset.AssetRevision
		if revisionPtr != nil {
			rev = *revisionPtr
		}

		links = append(links, api.AssetBuildLinkInput{
			AssetID:       asset.AssetID,
			AssetRevision: rev,
		})
	}

	return links
}

// parseAssetRef splits "identifier" or "identifier:revision" into its parts.
func parseAssetRef(ref string) (string, *int64) {
	// UUIDs contain hyphens but not colons, names shouldn't contain colons,
	// so split on the last colon to separate identifier from revision.
	idx := strings.LastIndex(ref, ":")
	if idx == -1 {
		return ref, nil
	}

	identifier := ref[:idx]
	revStr := ref[idx+1:]

	rev, err := strconv.ParseInt(revStr, 10, 64)
	if err != nil {
		// The colon wasn't followed by a valid number; treat the whole string as the identifier
		return ref, nil
	}

	return identifier, &rev
}

func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
