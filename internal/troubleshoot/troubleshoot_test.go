package troubleshoot

import (
	"testing"

	"github.com/ryugen04/sango-tree/internal/config"
)

func TestRunSingle_Pass(t *testing.T) {
	check := config.TroubleshootCheck{
		Name:        "echo test",
		Command:     "echo hello",
		Description: "echoが動作すること",
		Expect:      "hello",
		Fix:         "fix command",
	}

	result := RunSingle(check)
	if result.Status != StatusPass {
		t.Errorf("expected pass, got %s: %s", result.Status, result.Output)
	}
}

func TestRunSingle_Fail(t *testing.T) {
	check := config.TroubleshootCheck{
		Name:        "mismatch test",
		Command:     "echo wrong",
		Description: "不一致で失敗すること",
		Expect:      "expected_value",
		Fix:         "docker compose up -d",
	}

	result := RunSingle(check)
	if result.Status != StatusFail {
		t.Errorf("expected fail, got %s", result.Status)
	}
	if result.Fix != "docker compose up -d" {
		t.Errorf("expected fix command, got %s", result.Fix)
	}
}

func TestRunSingle_CommandError(t *testing.T) {
	check := config.TroubleshootCheck{
		Name:        "error test",
		Command:     "exit 1",
		Description: "コマンドエラーで失敗すること",
		Expect:      "ok",
		Fix:         "fix it",
	}

	result := RunSingle(check)
	if result.Status != StatusFail {
		t.Errorf("expected fail, got %s", result.Status)
	}
}

func TestRun_Multiple(t *testing.T) {
	checks := []config.TroubleshootCheck{
		{
			Name:    "pass check",
			Command: "echo ok",
			Expect:  "ok",
		},
		{
			Name:    "fail check",
			Command: "echo ng",
			Expect:  "ok",
		},
	}

	results := Run(checks)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != StatusPass {
		t.Errorf("first check should pass, got %s", results[0].Status)
	}
	if results[1].Status != StatusFail {
		t.Errorf("second check should fail, got %s", results[1].Status)
	}
}
