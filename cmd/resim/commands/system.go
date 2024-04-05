package commands

import (
	"context"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/resim-ai/api-client/api"
	. "github.com/resim-ai/api-client/ptr"
)

// TODO(https://app.asana.com/0/1205228215063249/1205227572053894/f): we should have first class support in API for this
func checkSystemID(client api.ClientWithResponsesInterface, projectID uuid.UUID, identifier string) uuid.UUID {
	// Page through systems until we find the one with either a name or an ID
	// that matches the identifier string.
	systemID := uuid.Nil
	// First try the assumption that identifier is a UUID.
	err := uuid.Validate(identifier)
	if err == nil {
		// The identifier is a uuid - but does it refer to an existing system?
		potentialSystemID := uuid.MustParse(identifier)
		response, _ := client.GetSystemWithResponse(context.Background(), projectID, potentialSystemID)
		if response.HTTPResponse.StatusCode == http.StatusOK {
			// System found with ID
			return potentialSystemID
		}
	}
	// If we're here then either the identifier is not a UUID or the UUID was not
	// found. Users could choose to name systems with UUIDs so regardless of how
	// we got here we now search for identifier as a string name.
	var pageToken *string = nil
pageLoop:
	for {
		response, err := client.ListSystemsWithResponse(
			context.Background(), projectID, &api.ListSystemsParams{
				PageSize:  Ptr(100),
				PageToken: pageToken,
			})
		if err != nil {
			log.Fatal("failed to list systems:", err)
		}
		ValidateResponse(http.StatusOK, "failed to list systems", response.HTTPResponse, response.Body)
		if response.JSON200 == nil {
			log.Fatal("empty response")
		}
		pageToken = response.JSON200.NextPageToken
		systems := *response.JSON200.Systems
		for _, system := range systems {
			if system.Name == nil {
				log.Fatal("system has no name")
			}
			if system.SystemID == nil {
				log.Fatal("system ID is empty")
			}
			if *system.Name == identifier {
				systemID = *system.SystemID
				break pageLoop
			}
		}
		if pageToken == nil || *pageToken == "" {
			break
		}
	}
	return systemID
}

func getSystemID(client api.ClientWithResponsesInterface, projectID uuid.UUID, identifier string, failWhenNotFound bool) uuid.UUID {
	systemID := checkSystemID(client, projectID, identifier)
	if systemID == uuid.Nil && failWhenNotFound {
		log.Fatal("failed to find system with name or ID: ", identifier)
	}
	return systemID
}
