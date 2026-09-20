package sync

import (
	"fmt"
)

// Struct encoding all the updates that need to be made for a single test suite.
type TestSuiteUpdate struct {
	Name        string
	TestSuiteID TestSuiteID
	Experiences []*Experience
}

func getTestSuiteUpdates(matchedExperiencesByNewName map[string]ExperienceMatch,
	testSuites []TestSuite,
	testSuiteIDsByName map[string]TestSuiteID,
) ([]TestSuiteUpdate, error) {
	updates := []TestSuiteUpdate{}

	for _, testSuite := range testSuites {
		testSuiteID, exists := testSuiteIDsByName[testSuite.Name]
		if !exists {
			return nil, fmt.Errorf("Test suite not found: %s", testSuite.Name)
		}

		update := TestSuiteUpdate{
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
