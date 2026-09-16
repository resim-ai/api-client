package sync

import (
	"context"
	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	mockapiclient "github.com/resim-ai/api-client/api/mocks"
	. "github.com/resim-ai/api-client/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"testing"
)

var mockState = createMockState()
var mockProjectID = uuid.New()

var experiencesData = `
- name: Regression Analysis Alpha 99c862
  description: Placeholder description FbFIerQKJF
  locations:
  - loc-2e70
  - loc-f419
  tags:
  - sensor
  - stress
  - load-test
  - suite
  systems:
  - planner
  profile: arms
  experienceID: fb7b6f11-753e-45ee-9dc2-efafa0a2ba17
  environmentVariables:
  - name: ENV_LXJ
    value: vBSswt
  - name: ENV_STH
    value: lE5p2r
  - name: ENV_BLH
    value: 1zXBXC
  cacheExempt: true
  containerTimeoutSeconds: 3345
  archived: true
- name: AI Planning Experiment bb74f0
  description: Placeholder description tCRbvavBOA
  locations: []
  tags:
  - performance
  - sensor
  - load-test
  - alpha-test
  systems:
  - perception
  profile: legs
  experienceID: 792de355-4757-487f-beb9-f4debd2df98a
  environmentVariables:
  - name: ENV_CDU
    value: dKgzGF
  - name: ENV_KZT
    value: hO8mJe
  cacheExempt: false
  containerTimeoutSeconds: 2066
  archived: true
- name: Planner Load Simulation 84d7cb
  description: Placeholder description ctnXbFsILO
  locations:
  - loc-fb00
  - loc-5f0c
  tags:
  - vision
  - load-test
  - stress
  - loop
  systems:
  - analytics
  - planner
  profile: legs
  experienceID: 5cb51c5b-b976-47b5-928e-77d5ba616d07
  environmentVariables:
  - name: ENV_UEE
    value: 5RKX8p
  - name: ENV_PHE
    value: Zk5DTQ
  cacheExempt: false
  containerTimeoutSeconds: 6027
  archived: false
- name: Planner Load Simulation ff9033
  description: Placeholder description zMzqOMGXLc
  locations: []
  tags:
  - loop
  - optimization
  systems:
  - analytics
  profile: arms
  experienceID: 08134d8b-596d-41ad-81d8-0545172376ae
  environmentVariables:
  - name: ENV_MIV
    value: PXrJXa
  cacheExempt: false
  containerTimeoutSeconds: 3336
  archived: true
- name: Planner Optimization Run 347bb5
  description: Placeholder description ElKgGrNeJW
  locations:
  - loc-7b52
  - loc-a39b
  - loc-1cf0
  tags:
  - sensor
  systems:
  - planner
  - perception
  profile: legs
  experienceID: 0ca05584-d066-403d-9a32-9aa6a009688f
  environmentVariables:
  - name: ENV_DAL
    value: ZBfAYp
  cacheExempt: true
  containerTimeoutSeconds: 3277
  archived: true
- name: Sensor Calibration Series aff890
  description: Placeholder description QNKSjttMzV
  locations: []
  tags:
  - load-test
  - beta
  - alpha-test
  - performance
  systems:
  - analytics
  profile: legs
  experienceID: f17ae13a-0124-4cb7-8679-3b6a4a1ff78d
  environmentVariables:
  - name: ENV_DRU
    value: O3wGiY
  - name: ENV_JRF
    value: qFEiPz
  cacheExempt: false
  containerTimeoutSeconds: 4009
  archived: true
- name: Memory Optimization Test 6758de
  description: Placeholder description yBqUJzvkIw
  locations:
  - loc-b2e7
  tags:
  - vision
  - loop
  - performance
  - feedback
  systems:
  - perception
  profile: arms
  experienceID: e60122bf-b387-418a-87b3-3b3d31b7e1ec
  environmentVariables:
  - name: ENV_UWO
    value: c8hMdO
  - name: ENV_PIY
    value: AbOt0o
  cacheExempt: true
  containerTimeoutSeconds: 6437
  archived: false
- name: AI Planning Experiment 0a8ccd
  description: Placeholder description pTisWLZraG
  locations:
  - loc-d375
  - loc-102c
  tags:
  - performance
  systems:
  - analytics
  - perception
  profile: legs
  experienceID: fa58c801-31a1-4e3e-8824-311cf492de98
  environmentVariables:
  - name: ENV_LWV
    value: HBdfPH
  cacheExempt: true
  containerTimeoutSeconds: 3664
  archived: false
- name: Planner Load Simulation 047e54
  description: Placeholder description PqTAsZWJpM
  locations: []
  tags:
  - suite
  - beta
  - loop
  systems:
  - analytics
  - perception
  profile: legs
  experienceID: c0abb5fb-e9ce-4e09-af8b-a83547d25e69
  environmentVariables:
  - name: ENV_GIW
    value: S7GoQH
  - name: ENV_CDU
    value: 0AWzvI
  - name: ENV_JZY
    value: QwrSVh
  cacheExempt: false
  containerTimeoutSeconds: 3619
  archived: true
- name: Vision Calibration d31660
  description: Placeholder description bETJXjznMQ
  locations:
  - loc-ec3c
  - loc-305e
  - loc-50b3
  tags:
  - calibration
  - regression
  - suite
  - vision
  systems:
  - planner
  - perception
  profile: legs
  experienceID: 10bc88e0-8c78-4f52-9a3f-d6cc649940a2
  environmentVariables:
  - name: ENV_FRW
    value: HeXuXF
  cacheExempt: true
  containerTimeoutSeconds: 2087
  archived: false
- name: Perception Loop Debug 48df7e
  description: Placeholder description jgbvSQtrhJ
  locations: []
  tags:
  - alpha-test
  - suite
  - feedback
  systems:
  - planner
  profile: arms
  experienceID: 7f85e5d2-a25d-48bd-b605-b4afd57526fa
  environmentVariables:
  - name: ENV_IVV
    value: 13DfYb
  - name: ENV_YMQ
    value: ocYYOn
  - name: ENV_TJM
    value: ga0ReD
  - name: ENV_QZY
    value: AfmvnR
  cacheExempt: true
  containerTimeoutSeconds: 2918
  archived: false
- name: Sensor Calibration Series 5e8cc8
  description: Placeholder description kQHaCqaCSW
  locations: []
  tags:
  - performance
  - regression
  - load-test
  systems:
  - perception
  profile: arms
  experienceID: 03139286-9184-4fa3-b393-52380c085dce
  environmentVariables:
  - name: ENV_MCU
    value: esJ2vK
  cacheExempt: true
  containerTimeoutSeconds: 3964
  archived: false
- name: Vision Calibration b39a0f
  description: Placeholder description xlKiMNPOLq
  locations:
  - loc-7d90
  - loc-7d77
  tags:
  - load-test
  systems:
  - perception
  profile: arms
  experienceID: 850fb567-dee4-45f8-95bd-6fa1cdc95aff
  environmentVariables:
  - name: ENV_SEY
    value: EJRbjs
  cacheExempt: true
  containerTimeoutSeconds: 3183
  archived: false
- name: Perception Accuracy Test 7ed34b
  description: Placeholder description tYDMKOOFKO
  locations: []
  tags:
  - stress
  - beta
  - regression
  systems:
  - perception
  - planner
  profile: legs
  experienceID: 06a56869-33b2-4063-ba91-8297d313dec5
  environmentVariables:
  - name: ENV_RCX
    value: JMjWtP
  - name: ENV_XQW
    value: KNPcta
  cacheExempt: true
  containerTimeoutSeconds: 3164
  archived: false
- name: Motion Sensor Calibration 494af6
  description: Placeholder description IGLfOsYLGm
  locations:
  - loc-3a62
  - loc-3d17
  tags:
  - load-test
  - performance
  - feedback
  - experiment
  systems:
  - analytics
  profile: arms
  experienceID: 7a042f4c-00e4-4001-a678-17ad626ce39a
  environmentVariables:
  - name: ENV_GIS
    value: 3Sjopx
  - name: ENV_EAW
    value: kGmzXH
  cacheExempt: true
  containerTimeoutSeconds: 3466
  archived: true
- name: AI Planning Experiment 0e64fb
  description: Placeholder description QcznaTIDtz
  locations:
  - loc-8709
  - loc-4299
  tags:
  - performance
  - sensor
  - alpha-test
  systems:
  - perception
  - planner
  profile: legs
  experienceID: dcc2bb74-a111-4911-84af-29b4a67fc8e3
  environmentVariables:
  - name: ENV_HBN
    value: PkZW0v
  - name: ENV_TQW
    value: Woq45V
  cacheExempt: true
  containerTimeoutSeconds: 4934
  archived: true
- name: Motion Sensor Calibration f8b226
  description: Placeholder description JqaOOUFHru
  locations:
  - loc-a4fb
  tags:
  - performance
  - optimization
  - loop
  systems:
  - planner
  profile: arms
  experienceID: 55aa3359-f9b0-4b72-8194-bd569bab0ef3
  environmentVariables:
  - name: ENV_TUP
    value: D301Ku
  - name: ENV_QZQ
    value: ZYhW4b
  - name: ENV_XBT
    value: rIrh0I
  cacheExempt: false
  containerTimeoutSeconds: 3989
  archived: true
- name: Sensor Calibration Series 2a0a45
  description: Placeholder description qRdpknhUEk
  locations:
  - loc-7f89
  - loc-f45a
  tags:
  - load-test
  - vision
  - experiment
  - loop
  systems:
  - planner
  profile: arms
  experienceID: 5e78c760-c153-4d35-850c-809962192725
  environmentVariables:
  - name: ENV_OAO
    value: sJ2d4g
  - name: ENV_IRC
    value: s16dZs
  - name: ENV_TCK
    value: CVKGBQ
  cacheExempt: false
  containerTimeoutSeconds: 2004
  archived: true
- name: Regression Analysis Alpha 353bca
  description: Placeholder description KqtRXrLLNH
  locations:
  - loc-5705
  tags:
  - stress
  - suite
  systems:
  - analytics
  profile: arms
  experienceID: f09703c3-8cf7-47fe-b4e8-55ea0dd7be3d
  environmentVariables:
  - name: ENV_ZEL
    value: zOBH3x
  - name: ENV_WQN
    value: H4A18p
  - name: ENV_TJT
    value: 4OCX2y
  - name: ENV_ZGR
    value: Q9GFlG
  cacheExempt: true
  containerTimeoutSeconds: 3698
  archived: true
- name: Planner Optimization Run 2a48d6
  description: Placeholder description XMAXZwZbDH
  locations:
  - loc-11b8
  tags:
  - sensor
  - suite
  systems:
  - analytics
  profile: arms
  experienceID: 5b469853-a6f9-4215-95e1-a78c94cb4856
  environmentVariables:
  - name: ENV_LAI
    value: 06IRTT
  cacheExempt: false
  containerTimeoutSeconds: 5950
  archived: true
`

type MockState struct {
	DatabaseState
	ExperiencePages [][]*Experience
	TagPages        [][]TagSet
	SystemPages     [][]SystemSet
}

func createMockState() MockState {
	var currentExperiences []*Experience
	err := yaml.Unmarshal([]byte(experiencesData), &currentExperiences)
	numPages := 2
	if err != nil {
		log.Fatal(err)
	}
	currentState := MockState{
		DatabaseState: DatabaseState{
			ExperiencesByName: make(map[string]*Experience),
			TagSetsByName:     make(map[string]TagSet),
			SystemSetsByName:  make(map[string]SystemSet),
			TestSuiteIDsByName: map[string]TestSuiteID{
				"regression": uuid.New(),
			},
		},
		ExperiencePages: make([][]*Experience, numPages),
		TagPages:        make([][]TagSet, numPages),
		SystemPages:     make([][]SystemSet, numPages),
	}

	for ii := 0; ii < numPages; ii++ {
		currentState.ExperiencePages[ii] = []*Experience{}
		currentState.TagPages[ii] = []TagSet{}
		currentState.SystemPages[ii] = []SystemSet{}
	}

	for ii, exp := range currentExperiences {
		currentState.ExperiencesByName[exp.Name] = exp
		for _, tag := range exp.Tags {
			if _, exists := currentState.TagSetsByName[tag]; !exists {
				currentState.TagSetsByName[tag] = TagSet{
					Name:          tag,
					TagID:         uuid.New(),
					ExperienceIDs: make(map[ExperienceID]struct{}),
				}
			}
			currentState.TagSetsByName[tag].ExperienceIDs[*exp.ExperienceID] = struct{}{}
		}
		for _, system := range exp.Systems {
			if _, exists := currentState.SystemSetsByName[system]; !exists {
				currentState.SystemSetsByName[system] = SystemSet{
					Name:          system,
					SystemID:      uuid.New(),
					ExperienceIDs: make(map[ExperienceID]struct{}),
				}
			}
			currentState.SystemSetsByName[system].ExperienceIDs[*exp.ExperienceID] = struct{}{}
		}
		currentState.ExperiencePages[ii%numPages] = append(currentState.ExperiencePages[ii%numPages], exp)
	}
	ii := 0
	for _, tag := range currentState.TagSetsByName {
		currentState.TagPages[ii%numPages] = append(currentState.TagPages[ii%numPages], tag)
		ii++
	}
	ii = 0
	for _, system := range currentState.SystemSetsByName {
		currentState.SystemPages[ii%numPages] = append(currentState.SystemPages[ii%numPages], system)
		ii++
	}
	return currentState
}

func ListExperienceTagsWithResponseMock(ctx context.Context,
	projectID api.ProjectID,
	params *api.ListExperienceTagsParams,
	reqEditors ...api.RequestEditorFn) (*api.ListExperienceTagsResponse, error) {
	if projectID != mockProjectID {
		log.Fatal("Bad project ID")
	}

	tags := []api.ExperienceTag{}

	var page *[]TagSet
	var returnToken *string
	if params.PageToken == nil {
		page = &mockState.TagPages[0]
		returnToken = Ptr("someToken")
	} else {
		page = &mockState.TagPages[1]
		returnToken = nil
	}

	for _, tag := range *page {
		tags = append(tags, api.ExperienceTag{
			Name:            tag.Name,
			ExperienceTagID: tag.TagID,
		})
	}
	return &api.ListExperienceTagsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListExperienceTagsOutput{
			NextPageToken:  returnToken,
			ExperienceTags: &tags,
		},
	}, nil
}

func ListSystemsWithResponseMock(ctx context.Context,
	projectID api.ProjectID,
	params *api.ListSystemsParams,
	reqEditors ...api.RequestEditorFn) (*api.ListSystemsResponse, error) {
	if projectID != mockProjectID {
		log.Fatal("Bad project ID")
	}

	systems := []api.System{}

	var page *[]SystemSet
	var returnToken *string
	if params.PageToken == nil {
		page = &mockState.SystemPages[0]
		returnToken = Ptr("someToken")
	} else {
		page = &mockState.SystemPages[1]
		returnToken = nil
	}

	for _, system := range *page {
		systems = append(systems, api.System{
			Name:     system.Name,
			SystemID: system.SystemID,
		})
	}
	return &api.ListSystemsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListSystemsOutput{
			NextPageToken: returnToken,
			Systems:       &systems,
		},
	}, nil
}

func ListTestSuitesWithResponseMock(ctx context.Context,
	projectID api.ProjectID,
	params *api.ListTestSuitesParams,
	reqEditors ...api.RequestEditorFn) (*api.ListTestSuitesResponse, error) {
	if projectID != mockProjectID {
		log.Fatal("Bad project ID")
	}

	testSuites := []api.TestSuite{}

	for name, testSuiteID := range mockState.TestSuiteIDsByName {
		testSuites = append(testSuites, api.TestSuite{
			Name:        name,
			TestSuiteID: testSuiteID,
		})
	}
	return &api.ListTestSuitesResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListTestSuiteOutput{
			NextPageToken: "",
			TestSuites:    testSuites,
		},
	}, nil
}

func ListExperiencesWithResponseMock(
	ctx context.Context,
	projectID api.ProjectID,
	params *api.ListExperiencesParams,
	reqEditors ...api.RequestEditorFn) (*api.ListExperiencesResponse, error) {
	if projectID != mockProjectID {
		log.Fatal("Bad project ID")
	}

	experiences := []api.Experience{}

	var page *[]*Experience
	var returnToken *string
	if params.PageToken == nil {
		page = &mockState.ExperiencePages[0]
		returnToken = Ptr("someToken")
	} else {
		page = &mockState.ExperiencePages[1]
		returnToken = nil
	}

	for _, experience := range *page {
		if *params.Archived != experience.Archived {
			continue
		}
		experiences = append(experiences, api.Experience{
			Name:                    experience.Name,
			Description:             experience.Description,
			Locations:               experience.Locations,
			Profile:                 *experience.Profile,
			ExperienceID:            *experience.ExperienceID,
			EnvironmentVariables:    *experience.EnvironmentVariables,
			CacheExempt:             *experience.CacheExempt,
			ContainerTimeoutSeconds: *experience.ContainerTimeoutSeconds,
			Archived:                experience.Archived,
		})
	}
	return &api.ListExperiencesResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListExperiencesOutput{
			NextPageToken: returnToken,
			Experiences:   &experiences,
		},
	}, nil
}

func ListExperiencesWithExperienceTagWithResponseMock(
	ctx context.Context,
	projectID api.ProjectID,
	tagID api.ExperienceTagID,
	params *api.ListExperiencesWithExperienceTagParams,
	reqEditors ...api.RequestEditorFn) (*api.ListExperiencesWithExperienceTagResponse, error) {
	if projectID != mockProjectID {
		log.Fatal("Bad project ID")
	}

	experiences := []api.Experience{}

	// Paging these causes issues because an empty return can be generated if we just inherit
	// from the experience pages. We just give one page for these.

	var tag string
	for name, tag_set := range mockState.TagSetsByName {
		if tag_set.TagID == tagID {
			tag = name
		}
	}

	for _, experience := range mockState.ExperiencesByName {
		if *params.Archived != experience.Archived {
			continue
		}
		if _, contains := mockState.TagSetsByName[tag].ExperienceIDs[*experience.ExperienceID]; !contains {
			continue
		}
		experiences = append(experiences, api.Experience{
			Name:         experience.Name,
			ExperienceID: *experience.ExperienceID,
		})
	}
	return &api.ListExperiencesWithExperienceTagResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListExperiencesOutput{
			NextPageToken: nil,
			Experiences:   &experiences,
		},
	}, nil
}

func ListExperiencesForSystemWithResponseMock(
	ctx context.Context,
	projectID api.ProjectID,
	systemID api.SystemID,
	params *api.ListExperiencesForSystemParams,
	reqEditors ...api.RequestEditorFn) (*api.ListExperiencesForSystemResponse, error) {
	if projectID != mockProjectID {
		log.Fatal("Bad project ID")
	}

	experiences := []api.Experience{}

	var system string
	for name, system_set := range mockState.SystemSetsByName {
		if system_set.SystemID == systemID {
			system = name
		}
	}

	for _, experience := range mockState.ExperiencesByName {
		if *params.Archived != experience.Archived {
			continue
		}
		if _, contains := mockState.SystemSetsByName[system].ExperienceIDs[*experience.ExperienceID]; !contains {
			continue
		}
		experiences = append(experiences, api.Experience{
			Name:         experience.Name,
			ExperienceID: *experience.ExperienceID,
		})
	}
	return &api.ListExperiencesForSystemResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK},
		JSON200: &api.ListExperiencesOutput{
			NextPageToken: nil,
			Experiences:   &experiences,
		},
	}, nil
}

func TestGetCurrentDatabaseState(t *testing.T) {
	// SETUP
	var client mockapiclient.ClientWithResponsesInterface
	client.On("ListExperienceTagsWithResponse",
		context.Background(),
		mockProjectID,
		mock.Anything,
	).Return(ListExperienceTagsWithResponseMock)

	client.On("ListSystemsWithResponse",
		context.Background(),
		mockProjectID,
		mock.Anything,
	).Return(ListSystemsWithResponseMock)

	client.On("ListTestSuitesWithResponse",
		context.Background(),
		mockProjectID,
		mock.Anything,
	).Return(ListTestSuitesWithResponseMock)

	client.On("ListExperiencesWithResponse",
		context.Background(),
		mockProjectID,
		mock.Anything,
	).Return(ListExperiencesWithResponseMock)

	client.On("ListExperiencesWithExperienceTagWithResponse",
		context.Background(),
		mockProjectID,
		mock.Anything,
		mock.Anything,
	).Return(ListExperiencesWithExperienceTagWithResponseMock)

	client.On("ListExperiencesForSystemWithResponse",
		context.Background(),
		mockProjectID,
		mock.Anything,
		mock.Anything,
	).Return(ListExperiencesForSystemWithResponseMock)

	// ACTION
	currentDatabaseState, err := getCurrentDatabaseState(&client, mockProjectID)
	assert.NoError(t, err)

	// VERIFICATION
	// Verify that the state we fetched matches the mocked state
	assert.Equal(t, len(currentDatabaseState.ExperiencesByName), len(mockState.ExperiencesByName))
	for name, experience := range currentDatabaseState.ExperiencesByName {
		assert.Equal(t, experience.ExperienceID, mockState.ExperiencesByName[name].ExperienceID)
		assert.Equal(t, experience.Description, mockState.ExperiencesByName[name].Description)
		assert.Equal(t, experience.Profile, mockState.ExperiencesByName[name].Profile)
		assert.Equal(t, experience.Locations, mockState.ExperiencesByName[name].Locations)
		assert.Equal(t, *experience.EnvironmentVariables, *mockState.ExperiencesByName[name].EnvironmentVariables)
		assert.Equal(t, experience.CacheExempt, mockState.ExperiencesByName[name].CacheExempt)
		assert.Equal(t, *experience.ContainerTimeoutSeconds, *mockState.ExperiencesByName[name].ContainerTimeoutSeconds)
		assert.Equal(t, experience.Archived, mockState.ExperiencesByName[name].Archived)
	}

	assert.Equal(t, len(currentDatabaseState.TagSetsByName), len(mockState.TagSetsByName))
	for name, tag_set := range currentDatabaseState.TagSetsByName {
		assert.Equal(t, tag_set.Name, mockState.TagSetsByName[name].Name)
		assert.Equal(t, tag_set.TagID, mockState.TagSetsByName[name].TagID)
		assert.Equal(t, len(tag_set.ExperienceIDs), len(mockState.TagSetsByName[name].ExperienceIDs))
		for eid := range tag_set.ExperienceIDs {
			assert.Contains(t, mockState.TagSetsByName[name].ExperienceIDs, eid)
		}
	}

	assert.Equal(t, len(currentDatabaseState.SystemSetsByName), len(mockState.SystemSetsByName))
	for name, system_set := range currentDatabaseState.SystemSetsByName {
		assert.Equal(t, system_set.Name, mockState.SystemSetsByName[name].Name)
		assert.Equal(t, system_set.SystemID, mockState.SystemSetsByName[name].SystemID)
		assert.Equal(t, len(system_set.ExperienceIDs), len(mockState.SystemSetsByName[name].ExperienceIDs))
		for eid := range system_set.ExperienceIDs {
			assert.Contains(t, mockState.SystemSetsByName[name].ExperienceIDs, eid)
		}
	}

	assert.Equal(t, len(currentDatabaseState.TestSuiteIDsByName), len(mockState.TestSuiteIDsByName))
	for name, testSuiteID := range currentDatabaseState.TestSuiteIDsByName {
		assert.Equal(t, testSuiteID, mockState.TestSuiteIDsByName[name])
	}
}
