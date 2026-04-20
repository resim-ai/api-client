package commands

import (
	"log"

	"github.com/spf13/viper"
)

const (
	requestPriorityMin     = 1
	requestPriorityMax     = 32767
	requestPriorityDefault = 1000
)

const requestPriorityDescription = "Optional execution priority for the request. Lower numbers are higher priority. Valid range: 1-32767."

func getRequestPriority(flagKey string) *int {
	priority := requestPriorityDefault
	if viper.IsSet(flagKey) {
		priority = viper.GetInt(flagKey)
	}

	if priority < requestPriorityMin || priority > requestPriorityMax {
		log.Fatalf(
			"priority must be between %d and %d, got %d",
			requestPriorityMin,
			requestPriorityMax,
			priority,
		)
	}

	return &priority
}
