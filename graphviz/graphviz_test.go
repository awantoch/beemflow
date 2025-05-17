package graphviz

import (
	"testing"

	"github.com/awantoch/beemflow/model"
)

func TestExportMermaid_EmptyFlow(t *testing.T) {
	f := &model.Flow{Name: "f"}
	s, err := ExportMermaid(f)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if s != "" {
		t.Errorf("expected empty string, got %q", s)
	}
}

func TestExportMermaid_RealFlow(t *testing.T) {
	f := &model.Flow{
		Name: "tweet_to_instagram",
		Steps: []model.Step{
			{ID: "fetch_tweet", Use: "twitter.tweet.get"},
			{ID: "rewrite", Use: "agent.llm.rewrite"},
			{ID: "post_instagram", Use: "instagram.media.create"},
		},
	}
	s, err := ExportMermaid(f)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if s == "" {
		t.Errorf("expected non-empty string")
	}
	if !(contains(s, "fetch_tweet") && contains(s, "rewrite") && contains(s, "post_instagram")) {
		t.Errorf("output missing step names: %q", s)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (contains(s[1:], substr) || contains(s[:len(s)-1], substr))))
}
