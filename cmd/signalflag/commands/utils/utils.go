package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// ValidateResponseSafe checks if the response is nil or if the status code
// differs from the expected status code.
func ValidateResponseSafe(expectedStatusCode int, message string, response *http.Response, body []byte) error {
	if response == nil {
		return fmt.Errorf("%s: no response", message)
	}

	if response.StatusCode != expectedStatusCode {
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("%s: expected status code %d, received %d (%s)", message, expectedStatusCode, response.StatusCode, response.Status)
		}

		prettyJSON, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("%s: expected status code %d, received %d (%s)", message, expectedStatusCode, response.StatusCode, response.Status)
		}

		return fmt.Errorf("%s: expected status code %d, received %d (%s)\nresponse body:\n%s",
			message, expectedStatusCode, response.StatusCode, response.Status, string(prettyJSON))
	}

	return nil
}

// Checked version of ValidateResponseSafe
func ValidateResponse(expectedStatusCode int, message string, response *http.Response, body []byte) {
	if err := ValidateResponseSafe(expectedStatusCode, message, response, body); err != nil {
		log.Fatal(err)
	}
}
