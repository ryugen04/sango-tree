package worktree

import (
	"fmt"

	"github.com/ryugen04/sango-tree/internal/config"
)

// BuildIncludeVars はinclude/template展開用の変数マップを構築する
// サービスのポートを計算し、worktreeに含まれるサービスはオフセットを加える
//
// 引数:
//   cfg: config.Config ポート情報を含む設定
//   offset: worktreeのポートオフセット値
//   worktreeServices: このworktreeに含まれるサービス名リスト
//
// 戻り値:
//   vars: "services.{name}.port" -> "{計算ポート}" のマップ
func BuildIncludeVars(cfg *config.Config, offset int, worktreeServices []string) map[string]string {
	// worktreeServicesをセット化して高速検索
	wtSet := make(map[string]bool, len(worktreeServices))
	for _, s := range worktreeServices {
		wtSet[s] = true
	}

	vars := make(map[string]string)

	for name, svc := range cfg.Services {
		resolvedPort := svc.Port

		// repo_nameが定義されていればそちらで判定（そうでなければnameを使う）
		repoName := svc.RepoName
		if repoName == "" {
			repoName = name
		}

		// 共有サービスでなく、このworktreeに含まれるサービスならオフセットを加算
		if !svc.Shared && wtSet[repoName] {
			resolvedPort += offset
		}

		vars[fmt.Sprintf("services.%s.port", name)] = fmt.Sprintf("%d", resolvedPort)
	}

	return vars
}
