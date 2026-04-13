package appserver

import (
	"strings"
	"testing"
)

func TestAuthoritativeTurnTextRequiresCompletedItem(t *testing.T) {
	t.Parallel()

	_, err := authoritativeTurnText(RunSingleTurnResult{
		Transcript: []ClientMessage{
			{
				AgentMessageCompletedEvent: &AgentMessageCompletedEventParams{
					Item: AgentMessageItem{Text: "transcript fallback"},
				},
			},
		},
	}, "review")
	if err == nil {
		t.Fatalf("expected error when CompletedItem is missing")
	}
	if got, want := err.Error(), "review turn completed without an authoritative agent message"; got != want {
		t.Fatalf("expected error %q, got %q", want, got)
	}
}

func TestAuthoritativeTurnTextRejectsEmptyCompletedItem(t *testing.T) {
	t.Parallel()

	_, err := authoritativeTurnText(RunSingleTurnResult{
		CompletedItem: &AgentMessageCompletedEventParams{
			Item: AgentMessageItem{
				Content: []AgentMessageContentPart{
					{Type: "text", Text: "   "},
					{Type: "text", Text: "\n"},
				},
			},
		},
	}, "review")
	if err == nil {
		t.Fatalf("expected error when authoritative message is empty")
	}
	if got, want := err.Error(), "review turn returned an empty authoritative agent message"; got != want {
		t.Fatalf("expected error %q, got %q", want, got)
	}
	if strings.Contains(err.Error(), "fallback") {
		t.Fatalf("authoritative completion error must not mention transcript fallback: %q", err)
	}
}
