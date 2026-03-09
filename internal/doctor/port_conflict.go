package doctor

import (
	"fmt"

	"github.com/ryugen04/sango-tree/internal/port"
	"github.com/ryugen04/sango-tree/internal/process"
)

// PortConflictCheck はサービスのポート競合チェックを実行する
// ports: サービス名→ポートのマップ
// sangoDir: .sangoディレクトリのパス
func CheckPortConflicts(ports map[string]int, sangoDir string) []CheckResult {
	var results []CheckResult

	for svcName, p := range ports {
		if p <= 0 {
			continue
		}

		holderPID, err := port.GetPortHolder(p)
		if err != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("ポート %d (%s)", p, svcName),
				Status:  StatusWarn,
				Message: fmt.Sprintf("ポート確認でエラー: %v", err),
			})
			continue
		}

		if holderPID == 0 {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("ポート %d (%s)", p, svcName),
				Status:  StatusPass,
				Message: "使用可能",
			})
			continue
		}

		// ポートが占有されている
		wtOwner, svcOwner, found := process.FindPIDOwner(sangoDir, holderPID)
		if found {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("ポート %d (%s)", p, svcName),
				Status:  StatusFail,
				Message: fmt.Sprintf("worktree %s の %s (PID %d) が占有中", wtOwner, svcOwner, holderPID),
				Fix:     fmt.Sprintf("kill %d", holderPID),
			})
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("ポート %d (%s)", p, svcName),
				Status:  StatusFail,
				Message: fmt.Sprintf("孤児プロセス (PID %d) が占有中", holderPID),
				Fix:     fmt.Sprintf("kill %d", holderPID),
			})
		}
	}

	return results
}
