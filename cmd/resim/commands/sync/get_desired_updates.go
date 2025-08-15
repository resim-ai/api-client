package sync

import (
	"log"
)


func GetDesiredUpdates(config *ExperienceSyncConfig, currentExperiencesByName *map[string]*Experience) *map[string]ExperienceUpdate {
	// Our algorithm can be summarized like so:
	//
	// For each configured experience, we attempt to match it to an existing experience if
	// possible. This procedure works like so:
	//
	// 1. If an existing experience has the same name, we match with it or fail if it's already
	//    been matched with.
	//
	//    If the desired experience has a user-specified ID that is not the same as matched
	//    experience we fail. We fail because this means that some other experience currently
	//    has this name. Because the current state has unique names, this can only happen if
	//    we're trying to update an existing experience to have the name that another currently
	//    uses. It could still be the case that the final state would be valid (e.g. if the
	//    current owner of the name is also going to change *its* name) but we don't allow
	//    this. In some cases, there may exist some ordering of name updates that never tries to
	//    create an invalid state, we don't attempt to determine it. There are easy work-arounds
	//    in such cases (e.g. running this script once to add a prefix to all experiences, and
	//    then again to set them to their new desired names).
	//
	//    Once we perform the matching, we remove the current experience that we matched with
	//    from the remainingCurrentExperiencesByID map so that no future step or experience can
	//    match with it.
	//
	//    We also set the experience ID in the configuration so it can be output for the user.
	//
	// 2. If no existing experience has the same name, and we have a user-specified ID, we
	//    attempt to find a match in the remainingCurrentExperiencesByID map. If we can't,
	//    that's a failure because the experience with that ID either never existed or already
	//    got matched (implying that multiple configured experiences were really referring to
	//    the same experience). If we can match we remove the current experience that we matched
	//    with from the remainingCurrentExperiencesByID map so that no future step can match
	//    with it.
	//
	// 3. If no existing experience has the same name or the same ID, it must be new.
	//
	// After all desired experiences have been matched, any remaining unmatched existing
	// experiences should be archived if they aren't already.
	//
	// The above procedure guarantees that every desired experience has a unique name and ID and
	// is matched to a unique existing experience if that's possible. It also guarantees that no
	// desired experience has a name currently owned by another experience.
	desiredUpdates := make(map[string]ExperienceUpdate)

	remainingCurrentExperiencesByID := byNameToByID(currentExperiencesByName)

	for _, experience := range config.Experiences {
		// Step 1: Attempt to match by name
		currExp, exists := (*currentExperiencesByName)[experience.Name]
		if exists {
			// If the match target has already been matched with, that's a failure
			if _, isAvailable := (*remainingCurrentExperiencesByID)[*currExp.ExperienceID]; !isAvailable {
				log.Fatalf("Experience name collision: %s", currExp.Name)
			}

			// If it exists but it's ID doesn't match a hard-coded one we provide, that's a failure
			if currExp.ExperienceID == nil || (experience.ExperienceID != nil && *experience.ExperienceID != *currExp.ExperienceID) {
				log.Fatalf("Multiple experiences desire the same name.")
			}

			// Experience exists with the same name and should be updated
			experience.ExperienceID = currExp.ExperienceID
			checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
				Original: currExp,
				New:      experience,
			})
			delete(*remainingCurrentExperiencesByID, *currExp.ExperienceID)
			continue
		}
		// Step 2: Attempt to match by ID
		if experience.ExperienceID != nil {
			// Check if there's still an unmatched experience with this ID:
			currExp, exists := (*remainingCurrentExperiencesByID)[*experience.ExperienceID]
			if !exists {
				log.Fatalf("No existing experience available with ID. This could be due to multiple configured experiences requesting the same ID: %s", *experience.ExperienceID)
			}

			checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
				Original: currExp,
				New:      experience,
			})
			delete(*remainingCurrentExperiencesByID, *currExp.ExperienceID)
			continue
		}

		// Step 3: Must be new then:
		checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
			Original: nil,
			New:      experience,
		})

	}
	// Step 4: Any leftover experiences should be archived
	for _, experience := range *remainingCurrentExperiencesByID {
		if experience.Archived {
			// No updates needed
			continue
		}
		archivedVersion := *experience
		archivedVersion.Archived = true
		checkedInsert(&desiredUpdates, experience.Name, ExperienceUpdate{
			Original: experience,
			New:      &archivedVersion,
		})
	}
	return &desiredUpdates
}


func byNameToByID(byName *map[string]*Experience) *map[string]*Experience {
	byID := make(map[string]*Experience)
	for _, v := range *byName {
		if v.ExperienceID != nil {
			byID[*v.ExperienceID] = v
		}
	}
	return &byID
}


func checkedInsert[K comparable, V any](m *map[K]V,
	key K, value V) {
	if _, exists := (*m)[key]; exists {
		log.Fatalf("Duplicate key!")
	}
	(*m)[key] = value
}
