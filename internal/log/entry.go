package log

import (
	"encoding/json"
	"strings"
	"time"
)

// LogEntry はJSONL形式のログエントリ
type LogEntry struct {
	Timestamp time.Time `json:"ts"`
	Service   string    `json:"svc"`
	Worktree  string    `json:"wt"`
	Stream    string    `json:"stream"` // "stdout" | "stderr"
	Level     string    `json:"level"`  // "info" | "warn" | "error" | "debug" | ""
	Message   string    `json:"msg"`
}

// Marshal はLogEntryをJSONバイト列に変換する
func (e *LogEntry) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEntry はJSONバイト列からLogEntryを復元する
func UnmarshalEntry(data []byte) (*LogEntry, error) {
	var e LogEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// DetectLevel は出力行からログレベルを推定する
// stderrの場合はerrorを返す
func DetectLevel(line, stream string) string {
	if stream == "stderr" {
		return "error"
	}

	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "ERR"):
		return "error"
	case strings.Contains(upper, "WARN") || strings.Contains(upper, "WARNING"):
		return "warn"
	case strings.Contains(upper, "DEBUG"):
		return "debug"
	default:
		return "info"
	}
}
