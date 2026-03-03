package doctor

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// CheckResult は個別チェックの結果
type CheckResult struct {
	Name    string
	Status  Status
	Message string
	Fix     string
}

// Status はチェック結果のステータス
type Status string

const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	StatusWarn Status = "warn"
)

// Check はDoctorCheckの定義
type Check struct {
	Name    string
	Command string
	Expect  string
	Fix     string
}

// Run は全チェックを実行して結果を返す
func Run(checks []Check) []CheckResult {
	results := make([]CheckResult, 0, len(checks))
	for _, c := range checks {
		results = append(results, RunSingle(c))
	}
	return results
}

// RunSingle は単一チェックを実行する
func RunSingle(check Check) CheckResult {
	out, err := exec.Command("sh", "-c", check.Command).CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		return CheckResult{
			Name:    check.Name,
			Status:  StatusFail,
			Message: output,
			Fix:     check.Fix,
		}
	}

	// expectが空ならコマンド成功でpass
	if check.Expect == "" {
		return CheckResult{
			Name:    check.Name,
			Status:  StatusPass,
			Message: output,
			Fix:     check.Fix,
		}
	}

	// expectパターンで判定
	if MatchExpect(output, check.Expect) {
		return CheckResult{
			Name:    check.Name,
			Status:  StatusPass,
			Message: output,
			Fix:     check.Fix,
		}
	}

	return CheckResult{
		Name:    check.Name,
		Status:  StatusFail,
		Message: fmt.Sprintf("%s (expected: %s)", output, check.Expect),
		Fix:     check.Fix,
	}
}

// MatchExpect は出力がexpectパターンにマッチするか判定する
// パターン種類:
// - 空文字列: コマンドが成功（exit 0）すればpass
// - 通常文字列: 部分一致でpass
// - "<N%": 出力から数値を抽出し、N未満ならpass（ディスク容量チェック用）
func MatchExpect(output, expect string) bool {
	if expect == "" {
		return true
	}

	// "<N%" パターン
	if strings.HasPrefix(expect, "<") && strings.HasSuffix(expect, "%") {
		thresholdStr := expect[1 : len(expect)-1]
		threshold, err := strconv.Atoi(thresholdStr)
		if err != nil {
			return false
		}

		// 出力から数値を抽出
		re := regexp.MustCompile(`(\d+)%`)
		matches := re.FindStringSubmatch(output)
		if len(matches) < 2 {
			return false
		}
		value, err := strconv.Atoi(matches[1])
		if err != nil {
			return false
		}
		return value < threshold
	}

	// 通常文字列: 部分一致
	return strings.Contains(output, expect)
}
