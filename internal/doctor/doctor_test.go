package doctor

import "testing"

func TestMatchExpect_Substring(t *testing.T) {
	tests := []struct {
		output string
		expect string
		want   bool
	}{
		{"go version go1.22.2 linux/amd64", "go1.22", true},
		{"go version go1.22.2 linux/amd64", "go1.23", false},
		{"Docker version 24.0.7", "Docker", true},
		{"Docker version 24.0.7", "Podman", false},
	}

	for _, tt := range tests {
		got := MatchExpect(tt.output, tt.expect)
		if got != tt.want {
			t.Errorf("MatchExpect(%q, %q) = %v, want %v", tt.output, tt.expect, got, tt.want)
		}
	}
}

func TestMatchExpect_Empty(t *testing.T) {
	if !MatchExpect("anything", "") {
		t.Error("空のexpectはtrueを返すべき")
	}
	if !MatchExpect("", "") {
		t.Error("空の出力と空のexpectはtrueを返すべき")
	}
}

func TestMatchExpect_Percentage(t *testing.T) {
	tests := []struct {
		output string
		expect string
		want   bool
	}{
		{"85%", "<90%", true},
		{"90%", "<90%", false},
		{"95%", "<90%", false},
		{"Disk usage: 50%", "<80%", true},
		{"Disk usage: 85%", "<80%", false},
	}

	for _, tt := range tests {
		got := MatchExpect(tt.output, tt.expect)
		if got != tt.want {
			t.Errorf("MatchExpect(%q, %q) = %v, want %v", tt.output, tt.expect, got, tt.want)
		}
	}
}

func TestRunSingle_Success(t *testing.T) {
	check := Check{
		Name:    "Echo",
		Command: "echo hello",
	}
	result := RunSingle(check)
	if result.Status != StatusPass {
		t.Errorf("echoコマンドはpassであるべき, got %v", result.Status)
	}
	if result.Message != "hello" {
		t.Errorf("Message = %q, want %q", result.Message, "hello")
	}
}

func TestRunSingle_Fail(t *testing.T) {
	check := Check{
		Name:    "NotExist",
		Command: "command_that_does_not_exist_12345",
	}
	result := RunSingle(check)
	if result.Status != StatusFail {
		t.Errorf("存在しないコマンドはfailであるべき, got %v", result.Status)
	}
}

func TestRunSingle_ExpectMismatch(t *testing.T) {
	check := Check{
		Name:    "Version",
		Command: "echo v18.0.0",
		Expect:  "v20",
		Fix:     "nvm install 20",
	}
	result := RunSingle(check)
	if result.Status != StatusFail {
		t.Errorf("expect不一致はfailであるべき, got %v", result.Status)
	}
	if result.Fix != "nvm install 20" {
		t.Errorf("Fix = %q, want %q", result.Fix, "nvm install 20")
	}
}
