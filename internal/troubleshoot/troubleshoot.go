package troubleshoot

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/ryugen04/sango-tree/internal/config"
	"github.com/ryugen04/sango-tree/internal/doctor"
)

// CheckResult はトラブルシュートチェックの結果
type CheckResult struct {
	Name        string
	Description string
	Status      Status
	Output      string
	Fix         string
}

// Status はチェック結果のステータス
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
)

// Run は指定サービスのtroubleshootチェックを全て実行する
func Run(checks []config.TroubleshootCheck) []CheckResult {
	results := make([]CheckResult, 0, len(checks))
	for _, c := range checks {
		results = append(results, RunSingle(c))
	}
	return results
}

// RunSingle は単一チェックを実行する
func RunSingle(check config.TroubleshootCheck) CheckResult {
	out, err := exec.Command("sh", "-c", check.Command).CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		return CheckResult{
			Name:        check.Name,
			Description: check.Description,
			Status:      StatusFail,
			Output:      fmt.Sprintf("%s (expected: %s)", output, check.Expect),
			Fix:         check.Fix,
		}
	}

	if check.Expect == "" {
		return CheckResult{
			Name:        check.Name,
			Description: check.Description,
			Status:      StatusPass,
			Output:      output,
			Fix:         check.Fix,
		}
	}

	if doctor.MatchExpect(output, check.Expect) {
		return CheckResult{
			Name:        check.Name,
			Description: check.Description,
			Status:      StatusPass,
			Output:      output,
			Fix:         check.Fix,
		}
	}

	return CheckResult{
		Name:        check.Name,
		Description: check.Description,
		Status:      StatusFail,
		Output:      fmt.Sprintf("%s (expected: %s)", output, check.Expect),
		Fix:         check.Fix,
	}
}
