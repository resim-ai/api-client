package sync

import (
	"fmt"
)

// Struct encoding all the updates that need to be made for a single test suite.
type TestSuiteIDUpdate struct {
	Name        string
	TestSuiteID TestSuiteID
	Experiences []*Experience
}

func getTestSuiteIDUpdates(matchedExperiencesByNewName map[string]ExperienceMatch,
	testSuites []TestSuite,
	testSuiteIDsByName map[string]TestSuiteID,
) ([]TestSuiteIDUpdate, error) {
	updates := []TestSuiteIDUpdate{}

	for _, testSuite := range testSuites {
		testSuiteID, exists := testSuiteIDsByName[testSuite.Name]
		if !exists {
			return nil, fmt.Errorf("Test suite not found: %s", testSuite.Name)
		}

		update := TestSuiteIDUpdate{
			Name:        testSuite.Name,
			TestSuiteID: testSuiteID,
			Experiences: []*Experience{},
		}

		for _, exp := range testSuite.Experiences {
			match, exists := matchedExperiencesByNewName[exp]
			if !exists || match.New.Archived {
				return nil, fmt.Errorf("Experience in test suite not found: %s", exp)
			}
			update.Experiences =
				append(update.Experiences, match.New)
		}
		updates = append(updates, update)
	}
	return updates, nil
}

type TestSuiteUpdates struct {
	Name               string
	TestSuiteID        TestSuiteID
	RevisedExperiences []*Experience
}

func getTestSuiteUpdates(matchedExperiencesByNewName map[string]ExperienceMatch,
	currentTestSuiteSetsByName map[string]TestSuiteSet,
	managedTestSuites []TestSuite) (map[string]*TestSuiteUpdates, error) {

	updates := make(map[string]*TestSuiteUpdates)

	for _, testSuite := range managedTestSuites {
		name := testSuite.Name
		testSuiteSet, exists := currentTestSuiteSetsByName[name]
		if !exists {
			return nil, fmt.Errorf("Non-existent test suite: %s", name)
		}
		updates[testSuite.Name] = &TestSuiteUpdates{
			Name:               name,
			TestSuiteID:        testSuiteSet.TestSuiteID,
			RevisedExperiences: []*Experience{},
		}
	}

	for _, match := range matchedExperiencesByNewName {
		// Get all the current test suite sets that we may have to update due to
		// archiving. We need to update these first so that we don't have a bunch of
		// revision when we eventually archive these experiences. We mark such test suites
		// simply by adding them to the updates map.
		beingArchived := match.Original != nil && !match.Original.Archived && match.New.Archived
		if beingArchived {
			for name, testSuiteSet := range currentTestSuiteSetsByName {
				if _, inSet := testSuiteSet.ExperienceIDs[match.Original.ExperienceID.ID]; inSet {
					if _, hasUpdate := updates[name]; !hasUpdate {
						updates[name] = &TestSuiteUpdates{
							Name:               name,
							TestSuiteID:        testSuiteSet.TestSuiteID,
							RevisedExperiences: []*Experience{},
						}
					}
				}
			}
		}

		// Now proceed mostly as normal. Updates may not have all test suites yet, but it
		// will have all those for which this experience is relevant.
		for testSuiteName, update := range updates {
			
		}
		


		

	}

	//	managedTestSuiteSetsByName := make(map[string]map[string]struct{})
	//	for _, testSuite := range managedTestSuites {
	//		_, exists := currentTestSuiteSetsByName[testSuite.Name]
	//		if !exists {
	//			return nil, fmt.Errorf("Non-existent test suite: %s", testSuite.Name)
	//		}
	//		managedTestSuiteSetsByName[testSuite.Name] = make(map[string]struct{})
	//		for _, e := range testSuite.Experiences {
	//			managedTestSuiteSetsByName[testSuite.Name][e] = struct{}{}
	//		}
	//
	//	}
	//
	//
	//	//for name, testSuiteSet  := range currentTestSuiteSetsByName {
	//	//	updates[name] = &TestSuiteUpdates{
	//	//		Name:      name,
	//	//		TestSuiteID:     testSuiteSet.TestSuiteID,
	//	//	}
	//	//
	//	//}
	//
	//	additionsPerTestSuite := make(map[string]map[string]struct{}	)
	//	removalsPerTestSuite := make(map[string]map[string]struct{})
	//	for name, _  := range currentTestSuiteSetsByName {
	//		additionsPerTestSuite[name] = make(map[string]struct{})
	//		removalsPerTestSuite[name] = make(map[string]struct{})
	//	}
	//
	//	for name, match := range matchedExperiencesByNewName {
	//	    for _, testSuiteSet := range currentTestSuiteSetsByName {
	//			    if _, exists := testSuiteSet.ExperienceIDs[match.Original.ExperienceID.ID]; exists {
	//				    // An existing experience in this test suite is going to be
	//				    // archived, so we need to remove it! This is the only case in
	//				    // which we adjust non-managed test suites.
	//				    removalsPerTestSuite[testSuiteSet.Name][name] = struct{}{}
	//			    }
	//		    }
	//	    }
	//
	//
	//		for testSuiteName, experiences := range managedTestSuiteSetsByName {
	//			_, desiredInSuite := experiences[name]
	//			currentlyInSuite := false
	//			if match.Old != nil {
	//				_, currentlyInSuite = currentTestSuiteSetsByName[testSuiteName].ExperienceIDs[match.Old.ExperienceID.ID]
	//			}
	//
	//			if desiredInSuite && !currentlyInSuite {
	//			}
	//
	//
	//
	//
	//		}
	//
	//
	//
	//	}
	return nil, nil
}
