package llm

import "testing"

type sampleItem struct {
	Word string `json:"word"`
	POS  string `json:"pos"`
}

type sample struct {
	Title string       `json:"title"`
	Items []sampleItem `json:"items"`
	Score int          `json:"score"`
}

func TestSchemaFor_StrictRequirements(t *testing.T) {
	s := SchemaFor[sample]()

	if s["type"] != "object" {
		t.Fatalf("root type=%v want object", s["type"])
	}
	if s["additionalProperties"] != false {
		t.Fatalf("root additionalProperties must be false, got %v", s["additionalProperties"])
	}
	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatalf("missing properties map")
	}
	req, ok := s["required"].([]any)
	if !ok {
		t.Fatalf("missing required slice")
	}
	if len(req) != len(props) {
		t.Fatalf("required(%d) must list every property(%d)", len(req), len(props))
	}
	for _, k := range []string{"$schema", "$id", "$defs", "definitions"} {
		if _, has := s[k]; has {
			t.Fatalf("unexpected %s in schema", k)
		}
	}
	items, ok := props["items"].(map[string]any)
	if !ok {
		t.Fatalf("items property not an object schema: %#v", props["items"])
	}
	itemSchema, ok := items["items"].(map[string]any)
	if !ok {
		t.Fatalf("array items schema missing: %#v", items)
	}
	if itemSchema["additionalProperties"] != false {
		t.Fatalf("nested object additionalProperties must be false")
	}
	nReq, _ := itemSchema["required"].([]any)
	nProps, _ := itemSchema["properties"].(map[string]any)
	if len(nReq) != len(nProps) || len(nProps) == 0 {
		t.Fatalf("nested object required must list all properties (got req=%v props=%v)", nReq, nProps)
	}
}
