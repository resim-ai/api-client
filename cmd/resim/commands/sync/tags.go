package sync

import (
	"fmt"
	"slices"
)

// Struct encoding all the updates that need to be made for a single tag.
type TagUpdates struct {
	Name      string
	TagID     TagID
	Additions []*Experience
	Removals  []*Experience
}

func getTagUpdates(matchedExperiencesByNewName map[string]ExperienceMatch,
	currentTagSetsByName map[string]TagSet,
	managedTags []string) (map[string]*TagUpdates, error) {
	updates := make(map[string]*TagUpdates)

	for tag, set := range currentTagSetsByName {
		updates[tag] = &TagUpdates{
			Name:      tag,
			TagID:     set.TagID,
			Additions: []*Experience{},
			Removals:  []*Experience{},
		}
	}
	for _, tag := range managedTags {
		if _, exists := currentTagSetsByName[tag]; !exists {
			return nil, fmt.Errorf("Managed tag doesn't exist: %s", tag)
		}
	}

	for _, match := range matchedExperiencesByNewName {
		if match.New.Archived {
			// Archived so we don't care
			continue
		}
		for _, tag := range match.New.Tags {
			tag_set, exists := currentTagSetsByName[tag]
			if !exists {
				return nil, fmt.Errorf("Non-existent tag: %s", tag)
			}

			if match.Original == nil {
				// Brand new. Always add!
				updates[tag].Additions = append(updates[tag].Additions, match.New)
				continue
			}
			_, alreadyTagged := tag_set.ExperienceIDs[*match.Original.ExperienceID]
			if !alreadyTagged {
				updates[tag].Additions = append(updates[tag].Additions, match.New)
			}
		}
		if match.Original == nil {
			// Can't possibly remove since it doesn't exist currently
			continue
		}
		for _, tag := range managedTags {
			_, currentlyHasTag := currentTagSetsByName[tag].ExperienceIDs[*match.Original.ExperienceID]
			// Not the fastest, but it will do
			if currentlyHasTag && !slices.Contains(match.New.Tags, tag) {
				updates[tag].Removals = append(updates[tag].Removals, match.New)
			}
		}
	}
	return updates, nil
}
