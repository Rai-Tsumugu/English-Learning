package llm

import (
	"strings"
	"testing"
)

func TestMask_APIKey(t *testing.T) {
	in := "Authorization: Bearer sk-abcdef1234567890XYZ token used"
	out := Mask(in)
	if strings.Contains(out, "sk-abcdef1234567890XYZ") {
		t.Fatalf("api key leaked: %q", out)
	}
	if !strings.Contains(out, "Bearer ***") && !strings.Contains(out, "sk-***") {
		t.Fatalf("expected masked token, got %q", out)
	}
}

func TestMask_Email(t *testing.T) {
	out := Mask("user alice@example.com signed in")
	if strings.Contains(out, "alice@example.com") {
		t.Fatalf("email leaked: %q", out)
	}
	if !strings.Contains(out, "a***@***") {
		t.Fatalf("unexpected mask: %q", out)
	}
}

func TestMask_Empty(t *testing.T) {
	if Mask("") != "" {
		t.Fatal("empty input must stay empty")
	}
	if Mask("nothing sensitive") != "nothing sensitive" {
		t.Fatal("benign string must pass through unchanged")
	}
}
