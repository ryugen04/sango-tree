package runbook

import (
	"testing"

	"github.com/ryugen04/sango-tree/internal/config"
)

var testEntry = config.RunbookEntry{
	Title:    "DB接続エラー",
	Symptoms: []string{"connection refused", "timeout"},
	Cause:    "DBが起動していない",
	Steps:    []string{"docker compose up -d postgres", "sango restart api"},
	Tags:     []string{"db", "connection"},
}

func TestSearch_ByTitle(t *testing.T) {
	services := map[string]*config.Service{
		"api": {Runbook: []config.RunbookEntry{testEntry}},
	}
	results := Search(services, "DB接続")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MatchField != "title" {
		t.Errorf("expected match on title, got %s", results[0].MatchField)
	}
}

func TestSearch_BySymptom(t *testing.T) {
	services := map[string]*config.Service{
		"api": {Runbook: []config.RunbookEntry{testEntry}},
	}
	results := Search(services, "connection refused")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MatchField != "symptoms" {
		t.Errorf("expected match on symptoms, got %s", results[0].MatchField)
	}
}

func TestSearch_ByCause(t *testing.T) {
	services := map[string]*config.Service{
		"api": {Runbook: []config.RunbookEntry{testEntry}},
	}
	results := Search(services, "起動していない")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MatchField != "cause" {
		t.Errorf("expected match on cause, got %s", results[0].MatchField)
	}
}

func TestSearch_ByTag(t *testing.T) {
	entry := config.RunbookEntry{
		Title:    "サーバーエラー",
		Symptoms: []string{"500 error"},
		Cause:    "不明",
		Steps:    []string{"再起動"},
		Tags:     []string{"infra", "network"},
	}
	services := map[string]*config.Service{
		"api": {Runbook: []config.RunbookEntry{entry}},
	}
	results := Search(services, "network")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].MatchField != "tags" {
		t.Errorf("expected match on tags, got %s", results[0].MatchField)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	services := map[string]*config.Service{
		"api": {Runbook: []config.RunbookEntry{testEntry}},
	}
	results := Search(services, "CONNECTION REFUSED")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestSearch_NoMatch(t *testing.T) {
	services := map[string]*config.Service{
		"api": {Runbook: []config.RunbookEntry{testEntry}},
	}
	results := Search(services, "nonexistent")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMatch_MultipleFields(t *testing.T) {
	// "connection"はsymptomsとtagsの両方に含まれるが、先にsymptomsでマッチ
	matched, field := Match(testEntry, "connection")
	if !matched {
		t.Fatal("expected match")
	}
	if field != "symptoms" {
		t.Errorf("expected symptoms (first match), got %s", field)
	}
}
