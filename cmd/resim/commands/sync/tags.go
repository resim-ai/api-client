package sync


type TagUpdates struct {
	additionsByTag map[string][]string
	removalsByTag map[string][]string	
}


// Go through the matches and build a set of new names per tag that I *want*.

// If I'm given a map of tags to lists of old ids, I can determine the new names of the new
// experiences from this.
//func planTagUpdates(currentTags




