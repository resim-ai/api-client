package commands

import (
	"testing"

	"github.com/resim-ai/api-client/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCustomFields(t *testing.T) {
	customFields := []string{
		// basic fields
		"hello=world",
		"number=3",
		"ts=2025-01-01T12:13:45Z",
		`some json={"foo": "bar"}`,

		// duplicates are fine
		"x=1",
		"x=3",
		"x=3",

		// explicit types are parsed too
		"text_with_type:text=hello",
		"number_with_type:number=3",
		"timestamp_with_type:timestamp=2025-01-01T12:13:45Z",
		"json_with_type:json={\"foo\": \"bar\"}",
	}

	parsedFields, err := parseCustomFields(customFields)
	require.NoError(t, err)
	assert.Len(t, parsedFields, 9) // 5 unique fields are present

	// Parsed fields don't come back in the same order, so need to iterate thru them
	for _, field := range parsedFields {
		switch field.Name {
		case "hello":
			assert.Equal(t, api.CustomFieldValueTypeText, field.Type)
			assert.Equal(t, []string{"world"}, field.Values)
		case "number":
			assert.Equal(t, api.CustomFieldValueTypeNumber, field.Type)
			assert.Equal(t, []string{"3"}, field.Values)
		case "ts":
			assert.Equal(t, api.CustomFieldValueTypeTimestamp, field.Type)
			assert.Equal(t, []string{"2025-01-01T12:13:45Z"}, field.Values)
		case "some json":
			assert.Equal(t, api.CustomFieldValueTypeJson, field.Type)
			assert.Equal(t, []string{`{"foo": "bar"}`}, field.Values)
		case "x":
			assert.Equal(t, api.CustomFieldValueTypeNumber, field.Type)
			assert.Equal(t, []string{"1", "3", "3"}, field.Values)
		case "text_with_type":
			assert.Equal(t, api.CustomFieldValueTypeText, field.Type)
			assert.Equal(t, []string{"hello"}, field.Values)
		case "number_with_type":
			assert.Equal(t, api.CustomFieldValueTypeNumber, field.Type)
			assert.Equal(t, []string{"3"}, field.Values)
		case "timestamp_with_type":
			assert.Equal(t, api.CustomFieldValueTypeTimestamp, field.Type)
			assert.Equal(t, []string{"2025-01-01T12:13:45Z"}, field.Values)
		case "json_with_type":
			assert.Equal(t, api.CustomFieldValueTypeJson, field.Type)
			assert.Equal(t, []string{`{"foo": "bar"}`}, field.Values)
		default:
			t.Fatalf("unexpected field name: %s", field.Name)
		}
	}

	t.Run("RejectInvalidFormats", func(t *testing.T) {
		_, err := parseCustomFields([]string{"hello 32"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "invalid custom field format: hello")

		_, err = parseCustomFields([]string{"hello:what-in-the-world=32"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "invalid custom field type: what-in-the-world (expected text|number|timestamp|json)")
	})

	t.Run("RejectMixedTypes", func(t *testing.T) {
		// rejected as hello was first inferred as a text type
		_, err := parseCustomFields([]string{"hello=world", "hello=3"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "custom field hello has type text, but a value of type number was provided")

		// using an explicit type will fix it
		_, err = parseCustomFields([]string{"hello=world", "hello:text=3"})
		require.NoError(t, err)

		// mixing explicit types should be rejected too
		_, err = parseCustomFields([]string{"x:number=43", "x:text=54"})
		require.Error(t, err)
		assert.ErrorContains(t, err, "custom field x has type number, but a value of type text was provided")
	})
}
