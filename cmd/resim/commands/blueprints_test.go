package commands

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateBlueprintCmdHasRequiredFlags(t *testing.T) {
	requiredFlags := []string{blueprintNameKey, blueprintCueFileKey}
	for _, name := range requiredFlags {
		flag := createBlueprintCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on createBlueprintCmd", name)
		assert.Equal(t, []string{"true"}, flag.Annotations[cobra.BashCompOneRequiredFlag], "--%s should be required", name)
	}
}

func TestReviseBlueprintCmdHasRequiredFlags(t *testing.T) {
	requiredFlags := []string{blueprintNameKey, blueprintCueFileKey}
	for _, name := range requiredFlags {
		flag := reviseBlueprintCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on reviseBlueprintCmd", name)
		assert.Equal(t, []string{"true"}, flag.Annotations[cobra.BashCompOneRequiredFlag], "--%s should be required", name)
	}
}

func TestGetBlueprintCmdHasFlags(t *testing.T) {
	nameFlag := getBlueprintCmd.Flags().Lookup(blueprintNameKey)
	assert.NotNil(t, nameFlag, "--name flag should exist on getBlueprintCmd")
	assert.Equal(t, []string{"true"}, nameFlag.Annotations[cobra.BashCompOneRequiredFlag], "--name should be required")

	versionFlag := getBlueprintCmd.Flags().Lookup(blueprintVersionKey)
	assert.NotNil(t, versionFlag, "--version flag should exist on getBlueprintCmd")
	assert.Nil(t, versionFlag.Annotations[cobra.BashCompOneRequiredFlag], "--version should not be required")

	cueOnlyFlag := getBlueprintCmd.Flags().Lookup(blueprintCueOnlyKey)
	assert.NotNil(t, cueOnlyFlag, "--cue-only flag should exist on getBlueprintCmd")
	assert.Nil(t, cueOnlyFlag.Annotations[cobra.BashCompOneRequiredFlag], "--cue-only should not be required")
}

func TestArchiveBlueprintCmdHasFlags(t *testing.T) {
	nameFlag := archiveBlueprintCmd.Flags().Lookup(blueprintNameKey)
	assert.NotNil(t, nameFlag, "--name flag should exist on archiveBlueprintCmd")
	assert.Equal(t, []string{"true"}, nameFlag.Annotations[cobra.BashCompOneRequiredFlag], "--name should be required")

	versionFlag := archiveBlueprintCmd.Flags().Lookup(blueprintVersionKey)
	assert.NotNil(t, versionFlag, "--version flag should exist on archiveBlueprintCmd")
	assert.Nil(t, versionFlag.Annotations[cobra.BashCompOneRequiredFlag], "--version should not be required")
}

func TestBlueprintSubcommandsRegistered(t *testing.T) {
	expected := map[string]bool{"create": false, "revise": false, "list": false, "get": false, "archive": false}
	for _, sub := range blueprintCmd.Commands() {
		if _, ok := expected[sub.Name()]; ok {
			expected[sub.Name()] = true
		}
	}
	for name, found := range expected {
		assert.True(t, found, "%s subcommand should be registered under blueprintCmd", name)
	}
	assert.Contains(t, blueprintCmd.Aliases, "blueprint", "blueprintCmd should have a 'blueprint' alias")
}

// TestCreateBlueprint verifies the happy path: when no blueprint with the name
// exists (GetLatestBlueprint 404s), `create` creates version 1.
func (s *CommandsSuite) TestCreateBlueprint() {
	viper.Reset()
	blueprintID := uuid.New()
	cueContent := "package blueprint\n\nfoo: \"bar\"\n"

	cueFile := filepath.Join(s.T().TempDir(), "blueprint.cue")
	s.Require().NoError(os.WriteFile(cueFile, []byte(cueContent), 0644))

	viper.Set(blueprintNameKey, "my-blueprint")
	viper.Set(blueprintCueFileKey, cueFile)

	// The name is free: no blueprint with it exists yet.
	s.mockClient.On("GetLatestBlueprintWithResponse", matchContext, "my-blueprint").Return(
		&api.GetLatestBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNotFound},
		}, nil)

	s.mockClient.On("CreateBlueprintWithResponse", matchContext,
		mock.MatchedBy(func(body api.CreateBlueprintInput) bool {
			return body.Name == "my-blueprint" && body.CueContent == cueContent
		})).Return(
		&api.CreateBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201: &api.Blueprint{
				BlueprintID: blueprintID,
				Name:        "my-blueprint",
				CueContent:  cueContent,
				Version:     1,
			},
		}, nil)

	out := captureStdout(s, func() { createBlueprint(nil, nil) })
	s.Assert().Contains(out, "Created blueprint successfully!")
	s.Assert().Contains(out, blueprintID.String())
	s.Assert().Contains(out, "Blueprint Version: 1")
}

// TestReviseBlueprint verifies the happy path: when a blueprint with the name
// already exists (GetLatestBlueprint 200s), `revise` creates the next version.
func (s *CommandsSuite) TestReviseBlueprint() {
	viper.Reset()
	blueprintID := uuid.New()
	cueContent := "package blueprint\n\nfoo: \"baz\"\n"

	cueFile := filepath.Join(s.T().TempDir(), "blueprint.cue")
	s.Require().NoError(os.WriteFile(cueFile, []byte(cueContent), 0644))

	viper.Set(blueprintNameKey, "my-blueprint")
	viper.Set(blueprintCueFileKey, cueFile)

	// The blueprint already exists, so revising is allowed.
	s.mockClient.On("GetLatestBlueprintWithResponse", matchContext, "my-blueprint").Return(
		&api.GetLatestBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Blueprint{
				BlueprintID: blueprintID,
				Name:        "my-blueprint",
				Version:     1,
			},
		}, nil)

	s.mockClient.On("CreateBlueprintWithResponse", matchContext,
		mock.MatchedBy(func(body api.CreateBlueprintInput) bool {
			return body.Name == "my-blueprint" && body.CueContent == cueContent
		})).Return(
		&api.CreateBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusCreated},
			JSON201: &api.Blueprint{
				BlueprintID: blueprintID,
				Name:        "my-blueprint",
				CueContent:  cueContent,
				Version:     2,
			},
		}, nil)

	out := captureStdout(s, func() { reviseBlueprint(nil, nil) })
	s.Assert().Contains(out, "Revised blueprint successfully!")
	s.Assert().Contains(out, blueprintID.String())
	s.Assert().Contains(out, "Blueprint Version: 2")
}

// TestBlueprintExistsTrue verifies blueprintExists reports true when the
// GetLatestBlueprint endpoint returns a blueprint. This is the decision that
// makes `create` refuse and `revise` proceed.
func (s *CommandsSuite) TestBlueprintExistsTrue() {
	viper.Reset()
	s.mockClient.On("GetLatestBlueprintWithResponse", matchContext, "existing").Return(
		&api.GetLatestBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.Blueprint{Name: "existing", Version: 1},
		}, nil)

	s.Assert().True(blueprintExists("existing"))
}

// TestBlueprintExistsFalse verifies blueprintExists reports false on a 404. This
// is the decision that makes `create` proceed and `revise` refuse.
func (s *CommandsSuite) TestBlueprintExistsFalse() {
	viper.Reset()
	s.mockClient.On("GetLatestBlueprintWithResponse", matchContext, "missing").Return(
		&api.GetLatestBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNotFound},
		}, nil)

	s.Assert().False(blueprintExists("missing"))
}

func TestListBlueprintsCmdHasFlags(t *testing.T) {
	for _, name := range []string{blueprintJSONKey, blueprintAllKey} {
		flag := listBlueprintsCmd.Flags().Lookup(name)
		assert.NotNil(t, flag, "--%s flag should exist on listBlueprintsCmd", name)
		assert.Nil(t, flag.Annotations[cobra.BashCompOneRequiredFlag], "--%s should not be required", name)
	}
}

// TestListBlueprints verifies the default output: a table of name/version/created-at
// showing only the most recent version of each blueprint name.
func (s *CommandsSuite) TestListBlueprints() {
	viper.Reset()

	s.mockClient.On("ListBlueprintsWithResponse", matchContext,
		mock.MatchedBy(func(params *api.ListBlueprintsParams) bool {
			return params.PageSize != nil && *params.PageSize == 100 && params.PageToken == nil
		})).Return(
		&api.ListBlueprintsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListBlueprintsOutput{
				Blueprints: &[]api.Blueprint{
					{BlueprintID: uuid.New(), Name: "alpha", Version: 1},
					{BlueprintID: uuid.New(), Name: "alpha", Version: 3},
					{BlueprintID: uuid.New(), Name: "alpha", Version: 2},
					{BlueprintID: uuid.New(), Name: "beta", Version: 1},
				},
			},
		}, nil)

	out := captureStdout(s, func() { listBlueprints(nil, nil) })
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Header + one row per distinct name (latest version only).
	s.Require().Len(lines, 3)
	s.Assert().Contains(lines[0], "NAME")
	s.Assert().Contains(lines[0], "VERSION")
	s.Assert().Contains(lines[0], "CREATED AT")
	// Sorted by name; alpha collapses to its latest version (3), not 1 or 2.
	s.Assert().Contains(lines[1], "alpha")
	s.Assert().Contains(lines[1], "3")
	s.Assert().Contains(lines[2], "beta")
}

// TestListBlueprintsJSON verifies --json output omits the CUE content and still
// collapses to the latest version per name.
func (s *CommandsSuite) TestListBlueprintsJSON() {
	viper.Reset()
	viper.Set(blueprintJSONKey, true)

	s.mockClient.On("ListBlueprintsWithResponse", matchContext,
		mock.AnythingOfType("*api.ListBlueprintsParams")).Return(
		&api.ListBlueprintsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListBlueprintsOutput{
				Blueprints: &[]api.Blueprint{
					{BlueprintID: uuid.New(), Name: "alpha", Version: 1, CueContent: "secret-cue-1"},
					{BlueprintID: uuid.New(), Name: "alpha", Version: 3, CueContent: "secret-cue-3"},
					{BlueprintID: uuid.New(), Name: "beta", Version: 1, CueContent: "secret-cue-b"},
				},
			},
		}, nil)

	out := captureStdout(s, func() { listBlueprints(nil, nil) })

	// CUE content must never appear in list JSON.
	s.Assert().NotContains(out, "cueContent")
	s.Assert().NotContains(out, "secret-cue")

	// Collapsed to latest version per name.
	var got []blueprintSummary
	s.Require().NoError(json.Unmarshal([]byte(out), &got))
	s.Require().Len(got, 2)
	byName := map[string]blueprintSummary{}
	for _, b := range got {
		byName[b.Name] = b
	}
	s.Assert().Equal(3, byName["alpha"].Version)
	s.Assert().Equal(1, byName["beta"].Version)
}

// TestListBlueprintsAllVersions verifies --all-versions lists every version.
func (s *CommandsSuite) TestListBlueprintsAllVersions() {
	viper.Reset()
	viper.Set(blueprintAllKey, true)

	s.mockClient.On("ListBlueprintsWithResponse", matchContext,
		mock.AnythingOfType("*api.ListBlueprintsParams")).Return(
		&api.ListBlueprintsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListBlueprintsOutput{
				Blueprints: &[]api.Blueprint{
					{BlueprintID: uuid.New(), Name: "alpha", Version: 1},
					{BlueprintID: uuid.New(), Name: "alpha", Version: 3},
					{BlueprintID: uuid.New(), Name: "alpha", Version: 2},
					{BlueprintID: uuid.New(), Name: "beta", Version: 1},
				},
			},
		}, nil)

	out := captureStdout(s, func() { listBlueprints(nil, nil) })
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Header + every version of every name.
	s.Require().Len(lines, 5)
	s.Assert().Contains(lines[0], "NAME")
	s.Assert().Contains(lines[1], "alpha")
	s.Assert().Contains(lines[2], "alpha")
	s.Assert().Contains(lines[3], "alpha")
	s.Assert().Contains(lines[4], "beta")
}

// TestListBlueprintsEmpty verifies the friendly message when there are none.
func (s *CommandsSuite) TestListBlueprintsEmpty() {
	viper.Reset()

	s.mockClient.On("ListBlueprintsWithResponse", matchContext,
		mock.AnythingOfType("*api.ListBlueprintsParams")).Return(
		&api.ListBlueprintsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200:      &api.ListBlueprintsOutput{},
		}, nil)

	out := captureStdout(s, func() { listBlueprints(nil, nil) })
	s.Assert().Contains(out, "no blueprints")
}

func (s *CommandsSuite) TestListBlueprintsPaginates() {
	viper.Reset()
	nextPageToken := "page-2"

	s.mockClient.On("ListBlueprintsWithResponse", matchContext,
		mock.MatchedBy(func(params *api.ListBlueprintsParams) bool {
			return params.PageSize != nil && *params.PageSize == 100 && params.PageToken == nil
		})).Return(
		&api.ListBlueprintsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListBlueprintsOutput{
				Blueprints:    &[]api.Blueprint{{BlueprintID: uuid.New(), Name: "blueprint-1", Version: 1}},
				NextPageToken: &nextPageToken,
			},
		}, nil).Once()

	s.mockClient.On("ListBlueprintsWithResponse", matchContext,
		mock.MatchedBy(func(params *api.ListBlueprintsParams) bool {
			return params.PageToken != nil && *params.PageToken == nextPageToken
		})).Return(
		&api.ListBlueprintsResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.ListBlueprintsOutput{
				Blueprints: &[]api.Blueprint{{BlueprintID: uuid.New(), Name: "blueprint-2", Version: 1}},
			},
		}, nil).Once()

	out := captureStdout(s, func() { listBlueprints(nil, nil) })
	s.Assert().Contains(out, "blueprint-1")
	s.Assert().Contains(out, "blueprint-2")
}

// TestGetBlueprintLatest exercises getBlueprint with no --version set: viper.IsSet
// is false, so the latest-version endpoint is used.
func (s *CommandsSuite) TestGetBlueprintLatest() {
	viper.Reset()
	blueprintID := uuid.New()
	viper.Set(blueprintNameKey, "my-blueprint")

	s.mockClient.On("GetLatestBlueprintWithResponse", matchContext, "my-blueprint").Return(
		&api.GetLatestBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Blueprint{
				BlueprintID: blueprintID,
				Name:        "my-blueprint",
				Version:     4,
			},
		}, nil)

	out := captureStdout(s, func() { getBlueprint(nil, nil) })
	s.Assert().Contains(out, "my-blueprint")
	s.Assert().Contains(out, blueprintID.String())
	s.Assert().Contains(out, "\"version\": 4")
}

// TestGetBlueprintVersion exercises getBlueprint with --version set: viper.IsSet
// is true, so the specific-version endpoint is used with the requested version.
func (s *CommandsSuite) TestGetBlueprintVersion() {
	viper.Reset()
	blueprintID := uuid.New()
	viper.Set(blueprintNameKey, "my-blueprint")
	viper.Set(blueprintVersionKey, 2)

	s.mockClient.On("GetBlueprintVersionWithResponse", matchContext, "my-blueprint", 2).Return(
		&api.GetBlueprintVersionResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Blueprint{
				BlueprintID: blueprintID,
				Name:        "my-blueprint",
				Version:     2,
			},
		}, nil)

	out := captureStdout(s, func() { getBlueprint(nil, nil) })
	s.Assert().Contains(out, "\"version\": 2")
}

// TestGetBlueprintCueOnly exercises getBlueprint with --cue-only: the raw CUE
// content is printed verbatim, with none of the surrounding JSON.
func (s *CommandsSuite) TestGetBlueprintCueOnly() {
	viper.Reset()
	blueprintID := uuid.New()
	cueContent := "package blueprint\n\nfoo: \"bar\"\n"
	viper.Set(blueprintNameKey, "my-blueprint")
	viper.Set(blueprintCueOnlyKey, true)

	s.mockClient.On("GetLatestBlueprintWithResponse", matchContext, "my-blueprint").Return(
		&api.GetLatestBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusOK},
			JSON200: &api.Blueprint{
				BlueprintID: blueprintID,
				Name:        "my-blueprint",
				CueContent:  cueContent,
				Version:     4,
			},
		}, nil)

	out := captureStdout(s, func() { getBlueprint(nil, nil) })
	s.Assert().Equal(cueContent, out)
	s.Assert().NotContains(out, blueprintID.String())
	s.Assert().NotContains(out, "\"name\"")
}

func (s *CommandsSuite) TestArchiveBlueprint() {
	viper.Reset()
	viper.Set(blueprintNameKey, "my-blueprint")

	s.mockClient.On("ArchiveBlueprintWithResponse", matchContext, "my-blueprint").Return(
		&api.ArchiveBlueprintResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil)

	out := captureStdout(s, func() { archiveBlueprint(nil, nil) })
	s.Assert().Contains(out, "Archived blueprint \"my-blueprint\" successfully!")
}

func (s *CommandsSuite) TestArchiveBlueprintVersion() {
	viper.Reset()
	viper.Set(blueprintNameKey, "my-blueprint")
	viper.Set(blueprintVersionKey, 2)

	s.mockClient.On("ArchiveBlueprintVersionWithResponse", matchContext, "my-blueprint", 2).Return(
		&api.ArchiveBlueprintVersionResponse{
			HTTPResponse: &http.Response{StatusCode: http.StatusNoContent},
		}, nil)

	out := captureStdout(s, func() { archiveBlueprint(nil, nil) })
	s.Assert().Contains(out, "Archived blueprint \"my-blueprint\" version 2 successfully!")
}
