package commands

import (
	"log"
	"strings"

	"github.com/google/uuid"
)

// This function takes a comma-separated list of UUIDs represented as strings
// and returns a separated array of parsed UUIDs.
func parseUUIDs(commaSeparatedUUIDs string) []uuid.UUID {
	if commaSeparatedUUIDs == "" {
		return []uuid.UUID{}
	}
	strs := strings.Split(commaSeparatedUUIDs, ",")
	result := make([]uuid.UUID, len(strs))

	for i := 0; i < len(strs); i++ {
		id, err := uuid.Parse(strings.TrimSpace(strs[i]))
		if err != nil {
			log.Fatal(err)
		}
		result[i] = id
	}
	return result
}
