package sync

import (
	"fmt"
)

// Struct encoding all the updates that need to be made for a single system.
type SystemUpdates struct {
	Name      string
	SystemID  SystemID
	Additions []*Experience
}

func getSystemUpdates(matchedExperiencesByNewName map[string]ExperienceMatch,
	currentSystemSetsByName map[string]SystemSet) (map[string]*SystemUpdates, error) {
	updates := make(map[string]*SystemUpdates)

	for system, set := range currentSystemSetsByName {
		updates[system] = &SystemUpdates{
			Name:      system,
			SystemID:  set.SystemID,
			Additions: []*Experience{},
		}
	}
	for _, match := range matchedExperiencesByNewName {
		if match.New.Archived {
			// Archived so we don't care
			continue
		}
		for _, system := range match.New.Systems {
			system_set, exists := currentSystemSetsByName[system]
			if !exists {
				return nil, fmt.Errorf("Non-existent system: %s", system)
			}

			if match.Original == nil {
				// Brand new. Always add!
				updates[system].Additions = append(updates[system].Additions, match.New)
				continue
			}
			_, alreadyInSystem := system_set.ExperienceIDs[*match.Original.ExperienceID]
			if !alreadyInSystem {
				updates[system].Additions = append(updates[system].Additions, match.New)
			}
		}
	}
	return updates, nil
}
