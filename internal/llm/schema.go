package llm

import (
	"encoding/json"
	"reflect"

	"github.com/invopop/jsonschema"
)

// SchemaFor generates a JSON Schema (as a map) for T suitable for use as a
// strict json_schema response format.
//
// Strict requirements enforced:
//   - additionalProperties: false on every object
//   - every property listed in "required"
//   - no $schema / $id / definitions / $defs (inlined / removed)
func SchemaFor[T any]() map[string]any {
	var zero T
	r := &jsonschema.Reflector{
		Anonymous:                  true,
		ExpandedStruct:             true,
		DoNotReference:             true,
		AllowAdditionalProperties:  false,
		RequiredFromJSONSchemaTags: false,
	}
	s := r.ReflectFromType(reflect.TypeOf(zero))
	b, err := json.Marshal(s)
	if err != nil {
		return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}, "required": []any{}}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}, "required": []any{}}
	}
	for _, k := range []string{"$schema", "$id", "definitions", "$defs", "title"} {
		delete(m, k)
	}
	strictify(m)
	return m
}

func strictify(node map[string]any) {
	for _, k := range []string{"$schema", "$id", "definitions", "$defs"} {
		delete(node, k)
	}
	if t, ok := node["type"].(string); ok && t == "object" {
		node["additionalProperties"] = false
		if props, ok := node["properties"].(map[string]any); ok {
			required := make([]any, 0, len(props))
			for name, v := range props {
				required = append(required, name)
				if child, ok := v.(map[string]any); ok {
					strictify(child)
				}
			}
			node["required"] = required
		} else {
			if _, has := node["properties"]; !has {
				node["properties"] = map[string]any{}
			}
			if _, has := node["required"]; !has {
				node["required"] = []any{}
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		strictify(items)
	}
	for _, key := range []string{"anyOf", "allOf", "oneOf"} {
		if arr, ok := node[key].([]any); ok {
			for _, e := range arr {
				if cm, ok := e.(map[string]any); ok {
					strictify(cm)
				}
			}
		}
	}
}
