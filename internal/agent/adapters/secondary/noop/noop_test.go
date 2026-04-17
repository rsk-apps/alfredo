package noop

import (
	"context"
	"strings"
	"testing"

	"github.com/rafaelsoares/alfredo/internal/agent/port"
)

func TestAdapterCompleteReturnsPortugueseStub(t *testing.T) {
	out, err := NewAdapter(nil).Complete(context.Background(), port.LLMInput{})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if out.FinalText != Reply {
		t.Fatalf("reply = %q", out.FinalText)
	}
	for _, word := range []string{" the ", " and ", " is ", " please ", " sorry "} {
		if strings.Contains(strings.ToLower(" "+out.FinalText+" "), word) {
			t.Fatalf("reply contains English stop word %q: %q", word, out.FinalText)
		}
	}
}
