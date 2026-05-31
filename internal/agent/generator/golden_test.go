package generator

import (
	"testing"

	"github.com/Rai-Tsumugu/English-Learning/internal/llm"
)

// TestOutputSchema_StrictConstraints verifies that the JSON Schema generated
// for generator.Output satisfies strict-mode Structured Outputs:
//
//   - additionalProperties: false on every object node
//   - all properties listed in required
//   - no dialect metadata ($schema / $id / $defs / definitions)
func TestOutputSchema_StrictConstraints(t *testing.T) {
	schema := llm.SchemaFor[Output]()
	for _, k := range []string{"$schema", "$id", "$defs", "definitions"} {
		if _, has := schema[k]; has {
			t.Errorf("schema root must not contain %q", k)
		}
	}
	walk(t, "$", schema)
}

func walk(t *testing.T, path string, node map[string]any) {
	t.Helper()
	if typ, ok := node["type"].(string); ok && typ == "object" {
		if ap, ok := node["additionalProperties"].(bool); !ok || ap {
			t.Errorf("%s: additionalProperties must be false (got %v)", path, node["additionalProperties"])
		}
		props, _ := node["properties"].(map[string]any)
		req, _ := node["required"].([]any)
		if len(props) != len(req) {
			t.Errorf("%s: required must list all properties (props=%d, required=%d)", path, len(props), len(req))
		}
		seen := make(map[string]bool, len(req))
		for _, r := range req {
			if s, ok := r.(string); ok {
				seen[s] = true
			}
		}
		for name, child := range props {
			if !seen[name] {
				t.Errorf("%s: property %q missing from required", path, name)
			}
			if cm, ok := child.(map[string]any); ok {
				walk(t, path+"."+name, cm)
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		walk(t, path+"[]", items)
	}
}
