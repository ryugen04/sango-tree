package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ryugen04/sango-tree/internal/config"
)

// PostProcessOptions は後処理のオプション
type PostProcessOptions struct {
	SkipIncludes bool
	SkipSetup    bool
	SkipHooks    bool
}

// PostProcessResult は後処理の結果
type PostProcessResult struct {
	IncludeResult *ExpandResult
	SetupErrors   []error
	HookErrors    []error
}

// RunPostProcess はworktreeの後処理（include展開・setup・hooks）を実行する
func RunPostProcess(cfg *config.Config, sangoDir string, wtName string, services []string, offset int, opts PostProcessOptions) *PostProcessResult {
	result := &PostProcessResult{}
	projectRoot := filepath.Dir(sangoDir)
	wtDir := cfg.Worktree.WorktreeDir(wtName)

	// 1. Include再展開
	if !opts.SkipIncludes {
		if len(cfg.Worktree.Include.Root) > 0 || len(cfg.Worktree.Include.PerService) > 0 {
			vars := BuildIncludeVars(cfg, offset, services)
			result.IncludeResult = ExpandIncludes(projectRoot, wtDir, services, cfg.Worktree.Include, vars, sangoDir)
		}
	}

	// 2. Setup実行
	if !opts.SkipSetup && cfg.Worktree.AutoSetup {
		for _, svcName := range services {
			svc, ok := cfg.Services[svcName]
			if !ok || len(svc.Setup) == 0 {
				continue
			}
			// repo_nameが設定されている場合、参照先のディレクトリを使う
			dirName := svcName
			if svc.RepoName != "" {
				dirName = svc.RepoName
			}
			svcDir := filepath.Join(wtDir, dirName)
			for _, setupCmd := range svc.Setup {
				fmt.Fprintf(os.Stderr, "[sango] setup (%s): %s\n", svcName, setupCmd)
				c := exec.Command("sh", "-c", setupCmd)
				c.Dir = svcDir
				c.Stdout = os.Stdout
				c.Stderr = os.Stderr
				if err := c.Run(); err != nil {
					result.SetupErrors = append(result.SetupErrors, fmt.Errorf("%s: %s: %w", svcName, setupCmd, err))
				}
			}
		}
	}

	// 3. Hooks実行
	if !opts.SkipHooks && len(cfg.Worktree.Hooks.PostCreate) > 0 {
		if err := RunHooks(cfg.Worktree.Hooks.PostCreate, wtDir, services); err != nil {
			result.HookErrors = append(result.HookErrors, err)
		}
	}

	return result
}

// HasErrors は後処理でエラーがあったかを返す
func (r *PostProcessResult) HasErrors() bool {
	if r.IncludeResult != nil && r.IncludeResult.HasErrors() {
		return true
	}
	return len(r.SetupErrors) > 0 || len(r.HookErrors) > 0
}
