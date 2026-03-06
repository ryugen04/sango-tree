package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/ryugen04/sango-tree/internal/config"
)

// RunHooks はフックエントリを実行する
// per_service=trueなら各サービスディレクトリで実行
func RunHooks(hooks []config.HookEntry, branchDir string, serviceNames []string) error {
	for _, hook := range hooks {
		if hook.PerService {
			for _, name := range serviceNames {
				dir := filepath.Join(branchDir, name)
				fmt.Printf("[sango] フック実行 (%s): %s\n", name, hook.Command)
				c := exec.Command("sh", "-c", hook.Command)
				c.Dir = dir
				if out, err := c.CombinedOutput(); err != nil {
					fmt.Printf("[sango] フック警告 (%s): %v\n%s", name, err, out)
				}
			}
		} else {
			fmt.Printf("[sango] フック実行: %s\n", hook.Command)
			c := exec.Command("sh", "-c", hook.Command)
			c.Dir = branchDir
			if out, err := c.CombinedOutput(); err != nil {
				fmt.Printf("[sango] フック警告: %v\n%s", err, out)
			}
		}
	}
	return nil
}
