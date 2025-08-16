package sync

import (
	"log"
	"slices"
)

type TagUpdates struct {
	Name      string
	TagID     TagID
	Additions []*Experience
	Removals  []*Experience
}

// Go through the matches and build a set of new names per tag that I *want*.

// If I'm given a map of tags to lists of old ids, I can determine the new names of the new
// experiences from this.
//func planTagUpdates(currentTags

func getTagUpdates(matchedExperiencesByNewName map[string]*ExperienceMatch,
	currentTagSetsByName map[string]TagSet,
	managedTags []string) map[string]*TagUpdates {
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
			log.Fatal("Managed tag doesn't exist: ", tag)
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
				log.Fatal("Non-existent tag: ", tag)
			}

			if match.Original == nil {
				// Brand new. Always add!
				updates[tag].Additions = append(updates[tag].Additions, match.New)
				continue
			}
			_, alreadyTagged := tag_set.ExperienceIDs[match.Original.ExperienceID.ID]
			if !alreadyTagged {
				updates[tag].Additions = append(updates[tag].Additions, match.New)
			}
		}
		if match.Original == nil {
			// Can't possibly remove since it doesn't exist currently
			continue
		}
		for _, tag := range managedTags {
			_, currentlyHasTag := currentTagSetsByName[tag].ExperienceIDs[match.Original.ExperienceID.ID]
			// Not the fastest, but it will do
			if currentlyHasTag && !slices.Contains(match.New.Tags, tag) {
				updates[tag].Removals = append(updates[tag].Removals, match.New)
			}
		}
	}

	//for _, v := range updates {
	//	log.Print("++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	//	log.Print(v.Name)
	//	log.Print("Additions:")
	//	for _, addition := range v.Additions {
	//		log.Print("    ", addition.Name)
	//	}
	//	log.Print("Removals:")
	//	for _, removal := range v.Removals {
	//		log.Print("    ", removal.Name)
	//	}
	//
	//}

	return updates
}
