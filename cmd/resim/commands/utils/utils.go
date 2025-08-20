package utils

import (
	"encoding/json"
	"log"
	"net/http"
)

// Validate Response fails the command if the response is nil, or the
// status code is not what we expect.
func ValidateResponse(expectedStatusCode int, message string, response *http.Response, body []byte) {
	if response == nil {
		log.Fatal(message, ": ", "no response")
	}
	if response.StatusCode != expectedStatusCode {
		// Unmarshal response as JSON:
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			log.Fatal(message, ": expected status code: ", expectedStatusCode,
				" received: ", response.StatusCode, " status: ", response.Status)
		}
		// Pretty print the response map
		prettyJSON, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Fatal(message, ": expected status code: ", expectedStatusCode,
				" received: ", response.StatusCode, " status: ", response.Status)
		}
		// Handle the unmarshalled data
		log.Fatal(message, ": expected status code: ", expectedStatusCode,
			" received: ", response.StatusCode, " status: ", response.Status, "\n message:\n", string(prettyJSON))
	}

}
