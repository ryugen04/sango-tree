package process

import (
	"context"
	"fmt"

	"github.com/ryugen04/sango-tree/internal/port"
)

// VerificationResult は起動後検証の結果を保持する
type VerificationResult struct {
	PortListening *bool
	ProcessAlive  *bool
	Errors        []string
}

// RunPostStartVerification はサービス起動後の多段検証を実行する
func RunPostStartVerification(ctx context.Context, serviceName string, portNumber, pid int) VerificationResult {
	result := VerificationResult{}

	if err := ctx.Err(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("起動後検証キャンセル (%s): %v", serviceName, err))
		return result
	}

	// ポートが指定されている場合のみLISTEN確認
	if portNumber > 0 {
		holderPID, err := port.GetPortHolder(portNumber)
		if err != nil {
			listening := false
			result.PortListening = &listening
			result.Errors = append(result.Errors, fmt.Sprintf("ポート確認失敗 (%s:%d): %v", serviceName, portNumber, err))
		} else {
			listening := holderPID > 0
			result.PortListening = &listening
			if !listening {
				result.Errors = append(result.Errors, fmt.Sprintf("ポート未LISTEN (%s:%d)", serviceName, portNumber))
			}
		}
	}

	if err := ctx.Err(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("起動後検証キャンセル (%s): %v", serviceName, err))
		return result
	}

	// PIDが指定されている場合のみ生存確認
	if pid > 0 {
		alive := IsProcessRunning(pid)
		result.ProcessAlive = &alive
		if !alive {
			result.Errors = append(result.Errors, fmt.Sprintf("プロセス停止検知 (%s:%d)", serviceName, pid))
		}
	}

	return result
}

// HasErrors は検証エラー有無を返す
func (r VerificationResult) HasErrors() bool {
	return len(r.Errors) > 0
}
