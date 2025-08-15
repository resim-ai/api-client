package utils

import (
	"net/http"
	"encoding/json"
	"fmt"
	"log"
)

// Validate Response fails the command if the response is nil, or the
// status code is not what we expect.
func ValidateResponse(expectedStatusCode int, message string, response *http.Response, body []byte) {
	if err := ValidateResponseSafe(expectedStatusCode, message, response, body); err != nil {
		log.Fatal(err) // same behavior as before
	}
}


// ValidateResponseSafe returns an error when the command if the response is nil, or the status code
// is not what we expect.
func ValidateResponseSafe(expectedStatusCode int, message string, response *http.Response, body []byte) error {
	if response == nil {
		return fmt.Errorf("%s: no response", message)
	}

	if response.StatusCode != expectedStatusCode {
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("%s: expected status code: %d received: %d status: %s (invalid JSON body)",
				message, expectedStatusCode, response.StatusCode, response.Status)
		}

		prettyJSON, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("%s: expected status code: %d received: %d status: %s (failed to format JSON)",
				message, expectedStatusCode, response.StatusCode, response.Status)
		}

		return fmt.Errorf("%s: expected status code: %d received: %d status: %s\nmessage:\n%s",
			message, expectedStatusCode, response.StatusCode, response.Status, prettyJSON)
	}

	return nil
}
