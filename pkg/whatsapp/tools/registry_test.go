package tools

import (
	"encoding/json"
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// OpenAI's strict schema validation requires every "type": "object" function
// parameter to declare a "properties" key — even when empty. The Gemini API
// is lenient, so this only breaks on OpenAI/OpenRouter routes. A previous bug
// in convertGeminiSchemaToOpenRouter dropped the field when Properties was
// an empty map, causing every tool call routed to gpt-4o-mini to error with
// "Invalid schema for function '...': object schema missing properties".
// That silently flipped 100% of traffic to the Gemini fallback in production.
func TestConvertSchema_ObjectAlwaysIncludesPropertiesEvenWhenEmpty(t *testing.T) {
	in := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: map[string]*genai.Schema{},
	}

	out := convertGeminiSchemaToOpenRouter(in)

	assert.Equal(t, "object", out["type"])
	props, ok := out["properties"]
	require.True(t, ok, "object schemas must always include 'properties' for OpenAI compatibility")
	assert.Equal(t, map[string]interface{}{}, props, "empty Properties map must serialize as {}")
}

func TestConvertSchema_ObjectWithoutPropertiesField(t *testing.T) {
	// Some tools may have a nil Properties map rather than an empty one;
	// both must produce a valid OpenAI-compliant schema.
	in := &genai.Schema{
		Type: genai.TypeObject,
	}

	out := convertGeminiSchemaToOpenRouter(in)
	props, ok := out["properties"]
	require.True(t, ok, "object schemas must include properties even when Gemini schema has nil map")
	assert.Equal(t, map[string]interface{}{}, props)
}

func TestConvertSchema_GetClinicInfoToolSerializesValidJSON(t *testing.T) {
	// Regression test: serialize the actual get_clinic_info parameter shape
	// and confirm OpenAI's required field is present in the JSON sent over the wire.
	in := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: map[string]*genai.Schema{},
	}

	out := convertGeminiSchemaToOpenRouter(in)
	raw, err := json.Marshal(out)
	require.NoError(t, err)

	assert.Contains(t, string(raw), `"properties":{}`,
		"the JSON sent to OpenRouter must include an explicit (possibly empty) properties object")
}

func TestConvertSchema_NonObjectTypesDoNotEmitProperties(t *testing.T) {
	// Strings, numbers, booleans, arrays must NOT carry a properties key —
	// only objects do.
	cases := []genai.Type{
		genai.TypeString, genai.TypeInteger, genai.TypeNumber, genai.TypeBoolean, genai.TypeArray,
	}
	for _, tp := range cases {
		out := convertGeminiSchemaToOpenRouter(&genai.Schema{Type: tp})
		_, has := out["properties"]
		assert.False(t, has, "non-object type %v must not declare properties", tp)
	}
}

func TestConvertSchema_ObjectWithPropertiesKeepsExistingBehavior(t *testing.T) {
	// Make sure the fix does not regress the populated-properties path.
	in := &genai.Schema{
		Type:     genai.TypeObject,
		Required: []string{"name"},
		Properties: map[string]*genai.Schema{
			"name": {Type: genai.TypeString, Description: "patient name"},
		},
	}
	out := convertGeminiSchemaToOpenRouter(in)
	props := out["properties"].(map[string]interface{})
	require.Contains(t, props, "name")
	nameSchema := props["name"].(map[string]interface{})
	assert.Equal(t, "string", nameSchema["type"])
	assert.Equal(t, "patient name", nameSchema["description"])
	assert.Equal(t, []string{"name"}, out["required"])
}
