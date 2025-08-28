package sync

import (
	"context"
	"fmt"
	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	"github.com/resim-ai/api-client/cmd/resim/commands/utils"
	. "github.com/resim-ai/api-client/ptr"
	"log"
	"net/http"
)

type ExperienceID = api.ExperienceID
type SystemID = api.SystemID
type TagID = api.ExperienceTagID
type TestSuiteID = api.TestSuiteID
type EnvironmentVariable = api.EnvironmentVariable

type TagSet struct {
	Name          string
	TagID         TagID
	ExperienceIDs map[ExperienceID]struct{}
}

type SystemSet struct {
	Name          string
	SystemID      SystemID
	ExperienceIDs map[ExperienceID]struct{}
}

type TestSuiteSet struct {
	Name          string
	TestSuiteID   TestSuiteID
	ExperienceIDs map[ExperienceID]struct{}
}

type DatabaseState struct {
	ExperiencesByName   map[string]*Experience
	TagSetsByName       map[string]TagSet
	SystemSetsByName    map[string]SystemSet
	TestSuiteIDsByName  map[string]TestSuiteID
	TestSuiteSetsByName map[string]TestSuiteSet
}

type Result[T any] struct {
	Val T
	Err error
}

func wrapResult[T any](val T, err error) Result[T] {
	return Result[T]{Val: val, Err: err}
}

func getCurrentDatabaseState(client api.ClientWithResponsesInterface,
	projectID uuid.UUID) (*DatabaseState, error) {
	expCh := make(chan Result[map[string]*Experience])
	tagCh := make(chan Result[map[string]TagSet])
	sysCh := make(chan Result[map[string]SystemSet])
	tsIDCh := make(chan Result[map[string]TestSuiteID])
	tsCh := make(chan Result[map[string]TestSuiteSet])

	go func() {
		exp, err := getCurrentExperiencesByName(client, projectID)
		expCh <- wrapResult(exp, err)
	}()
	go func() {
		tags, err := getCurrentTagSetsByName(client, projectID)
		tagCh <- wrapResult(tags, err)
	}()
	go func() {
		sys, err := getCurrentSystemSetsByName(client, projectID)
		sysCh <- wrapResult(sys, err)
	}()
	go func() {
		ts, err := getCurrentTestSuitesIDsByName(client, projectID)
		tsIDCh <- wrapResult(ts, err)
	}()
	go func() {
		ts, err := getCurrentTestSuiteSetsByName(client, projectID)
		tsCh <- wrapResult(ts, err)
	}()

	expRes := <-expCh
	if expRes.Err != nil {
		return nil, expRes.Err
	}
	tagRes := <-tagCh
	if tagRes.Err != nil {
		return nil, tagRes.Err
	}
	sysRes := <-sysCh
	if sysRes.Err != nil {
		return nil, sysRes.Err
	}
	tsIDRes := <-tsIDCh
	if tsIDRes.Err != nil {
		return nil, tsIDRes.Err
	}

	tsRes := <-tsCh
	if tsRes.Err != nil {
		return nil, tsRes.Err
	}

	state := DatabaseState{
		ExperiencesByName:   expRes.Val,
		TagSetsByName:       tagRes.Val,
		SystemSetsByName:    sysRes.Val,
		TestSuiteIDsByName:  tsIDRes.Val,
		TestSuiteSetsByName: tsRes.Val,
	}

	// Update the tags in each experience
	for _, experience := range state.ExperiencesByName {
		if experience.Archived {
			continue
		}
		for tag, tagSet := range state.TagSetsByName {
			if _, has_tag := tagSet.ExperienceIDs[experience.ExperienceID.ID]; has_tag {
				experience.Tags = append(experience.Tags, tag)
			}
		}
		for system, systemSet := range state.SystemSetsByName {
			if _, has_system := systemSet.ExperienceIDs[experience.ExperienceID.ID]; has_system {
				experience.Systems = append(experience.Systems, system)
			}
		}
	}
	return &state, nil
}

func getCurrentExperiencesByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) (map[string]*Experience, error) {
	archived := true
	unarchived := false
	apiExperiences, err := fetchAllExperiences(client, projectID, unarchived)
	if err != nil {
		return nil, err
	}
	apiArchivedExperiences, err := fetchAllExperiences(client, projectID, archived)
	if err != nil {
		return nil, err
	}
	currentExperiencesByName := make(map[string]*Experience)
	for _, experience := range apiExperiences {
		addApiExperienceToExperienceMap(experience, currentExperiencesByName)
	}
	for _, experience := range apiArchivedExperiences {
		addApiExperienceToExperienceMap(experience, currentExperiencesByName)
	}
	return currentExperiencesByName, nil
}

func getCurrentTagSetsByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) (map[string]TagSet, error) {
	apiExperienceTags, err := fetchAllExperienceTags(client, projectID)
	if err != nil {
		return nil, err
	}

	currentTagSets := make(map[string]TagSet)

	for _, tag := range apiExperienceTags {
		archived := true
		unarchived := false
		apiExperiences, err := fetchAllExperiencesWithTag(client, projectID, tag.ExperienceTagID, unarchived)
		if err != nil {
			return nil, err
		}

		apiArchivedExperiences, err := fetchAllExperiencesWithTag(client, projectID, tag.ExperienceTagID, archived)
		if err != nil {
			return nil, err
		}
		apiExperiences = append(apiExperiences, apiArchivedExperiences...)

		experienceIDs := make(map[ExperienceID]struct{})
		for _, experience := range apiExperiences {
			experienceIDs[experience.ExperienceID] = struct{}{}
		}
		currentTagSets[tag.Name] = TagSet{
			Name:          tag.Name,
			TagID:         tag.ExperienceTagID,
			ExperienceIDs: experienceIDs,
		}
	}
	return currentTagSets, nil
}

func getCurrentSystemSetsByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) (map[string]SystemSet, error) {
	apiSystems, err := fetchAllSystems(client, projectID)
	if err != nil {
		return nil, err
	}

	currentSystemSets := make(map[string]SystemSet)

	for _, system := range apiSystems {
		archived := true
		unarchived := false
		apiExperiences, err := fetchAllExperiencesWithSystem(client, projectID, system.SystemID, unarchived)
		if err != nil {
			return nil, err
		}

		apiArchivedExperiences, err := fetchAllExperiencesWithSystem(client, projectID, system.SystemID, archived)
		if err != nil {
			return nil, err
		}

		apiExperiences = append(apiExperiences, apiArchivedExperiences...)

		experienceIDs := make(map[ExperienceID]struct{})
		for _, experience := range apiExperiences {
			experienceIDs[experience.ExperienceID] = struct{}{}
		}
		currentSystemSets[system.Name] = SystemSet{
			Name:          system.Name,
			SystemID:      system.SystemID,
			ExperienceIDs: experienceIDs,
		}
	}
	return currentSystemSets, nil
}

func getCurrentTestSuiteSetsByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) (map[string]TestSuiteSet, error) {
	apiTestSuites, err := fetchAllTestSuites(client, projectID)
	if err != nil {
		return nil, err
	}

	currentTestSuiteSets := make(map[string]TestSuiteSet)

	for _, testSuite := range apiTestSuites {
		unarchived := false
		apiExperiences, err := fetchAllExperiencesWithTestSuite(client, projectID, testSuite, unarchived)
		if err != nil {
			return nil, err
		}

		experienceIDs := make(map[ExperienceID]struct{})
		for _, experience := range apiExperiences {
			experienceIDs[experience.ExperienceID] = struct{}{}
		}
		currentTestSuiteSets[testSuite.Name] = TestSuiteSet{
			Name:          testSuite.Name,
			TestSuiteID:   testSuite.TestSuiteID,
			ExperienceIDs: experienceIDs,
		}
	}
	return currentTestSuiteSets, nil
}

func getCurrentTestSuitesIDsByName(
	client api.ClientWithResponsesInterface,
	projectID uuid.UUID) (map[string]TestSuiteID, error) {
	testSuiteIDsByName := make(map[string]TestSuiteID)
	var pageToken *string = nil

	for {
		response, err := client.ListTestSuitesWithResponse(
			context.Background(), projectID, &api.ListTestSuitesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list test suites: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusOK, "failed to list test suites", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || len(response.JSON200.TestSuites) == 0 {
			break
		}

		for _, testSuite := range response.JSON200.TestSuites {
			testSuiteIDsByName[testSuite.Name] = testSuite.TestSuiteID
		}

		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return testSuiteIDsByName, nil
}

func addApiExperienceToExperienceMap(experience api.Experience,
	currentExperiences map[string]*Experience) {
	currentExperiences[experience.Name] = &Experience{
		Name:                    experience.Name,
		Description:             experience.Description,
		Locations:               experience.Locations,
		Profile:                 &experience.Profile,
		ExperienceID:            &ExperienceIDWrapper{ID: experience.ExperienceID},
		EnvironmentVariables:    &experience.EnvironmentVariables,
		CacheExempt:             experience.CacheExempt,
		ContainerTimeoutSeconds: &experience.ContainerTimeoutSeconds,
		Archived:                experience.Archived,
	}
}

func fetchAllExperiences(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID,
	archived bool) ([]api.Experience, error) {
	allExperiences := []api.Experience{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperiencesWithResponse(
			context.Background(), projectID, &api.ListExperiencesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				Archived:  Ptr(archived),
				OrderBy:   Ptr("timestamp"),
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list experiences: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Experiences) == 0 {
			break // Either no experiences or we've reached the end of the list matching the page length
		}
		allExperiences = append(allExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allExperiences, nil
}

func fetchAllExperienceTags(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID) ([]api.ExperienceTag, error) {
	allExperienceTags := []api.ExperienceTag{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperienceTagsWithResponse(
			context.Background(), projectID, &api.ListExperienceTagsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list experiences: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusOK, "failed to list tags", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.ExperienceTags) == 0 {
			break // Either no tags or we've reached the end of the list matching the page length
		}
		allExperienceTags = append(allExperienceTags, *response.JSON200.ExperienceTags...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allExperienceTags, nil
}

func fetchAllSystems(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID) ([]api.System, error) {
	allSystems := []api.System{}
	var pageToken *string = nil

	for {
		response, err := client.ListSystemsWithResponse(
			context.Background(), projectID, &api.ListSystemsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list experiences: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusOK, "failed to list systems", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Systems) == 0 {
			break // Either no systems or we've reached the end of the list matching the page length
		}
		allSystems = append(allSystems, *response.JSON200.Systems...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allSystems, nil
}

func fetchAllTestSuites(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID) ([]api.TestSuite, error) {
	allTestSuites := []api.TestSuite{}
	var pageToken *string = nil

	for {
		response, err := client.ListTestSuitesWithResponse(
			context.Background(), projectID, &api.ListTestSuitesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list experiences: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusOK, "failed to list systems", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}

		pageToken = &response.JSON200.NextPageToken
		if response.JSON200 == nil || len(response.JSON200.TestSuites) == 0 {
			break // Either no systems or we've reached the end of the list matching the page length
		}
		allTestSuites = append(allTestSuites, response.JSON200.TestSuites...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allTestSuites, nil
}

func fetchAllExperiencesWithTag(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID,
	tagID TagID,
	archived bool) ([]api.Experience, error) {
	allExperiences := []api.Experience{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperiencesWithExperienceTagWithResponse(
			context.Background(), projectID, tagID, &api.ListExperiencesWithExperienceTagParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				Archived:  Ptr(archived),
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list experiences: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Experiences) == 0 {
			break // Either no experiences or we've reached the end of the list matching the page length
		}
		allExperiences = append(allExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allExperiences, nil
}

func fetchAllExperiencesWithSystem(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID,
	systemID SystemID,
	archived bool) ([]api.Experience, error) {
	allExperiences := []api.Experience{}
	var pageToken *string = nil

	for {
		response, err := client.ListExperiencesForSystemWithResponse(
			context.Background(), projectID, systemID, &api.ListExperiencesForSystemParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				Archived:  Ptr(archived),
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list experiences: %s", err)
		}
		utils.ValidateResponse(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Experiences) == 0 {
			break // Either no experiences or we've reached the end of the list matching the page length
		}
		allExperiences = append(allExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allExperiences, nil
}

func fetchAllExperiencesWithTestSuite(client api.ClientWithResponsesInterface,
	projectID openapi_types.UUID,
	testSuite api.TestSuite,
	archived bool) ([]api.Experience, error) {
	allExperiences := []api.Experience{}
	var pageToken *string = nil

	search := fmt.Sprintf("test_suite=\"%s:%d\"", testSuite.TestSuiteID, testSuite.TestSuiteRevision)
	log.Printf(search)
	for {
		response, err := client.ListExperiencesWithResponse(
			context.Background(), projectID, &api.ListExperiencesParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
				Archived:  Ptr(archived),
				Search:    &search,
			})
		if err != nil {
			return nil, fmt.Errorf("failed to list experiences for test suite: %s", err)
		}
		err = utils.ValidateResponseSafe(http.StatusOK, "failed to list experiences", response.HTTPResponse, response.Body)
		if err != nil {
			return nil, err
		}

		pageToken = response.JSON200.NextPageToken
		if response.JSON200 == nil || len(*response.JSON200.Experiences) == 0 {
			break // Either no experiences or we've reached the end of the list matching the page length
		}
		allExperiences = append(allExperiences, *response.JSON200.Experiences...)
		if pageToken == nil || *pageToken == "" {
			break
		}
	}

	return allExperiences, nil
}
