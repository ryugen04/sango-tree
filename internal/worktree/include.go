package worktree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryugen04/sango-tree/internal/config"
)

// ExpandResult はinclude展開の結果を保持する
type ExpandResult struct {
	Warnings []error // required=false のエントリの失敗
	Errors   []error // required=true のエントリの失敗
}

// HasErrors はrequiredエントリの失敗があるかを返す
func (r *ExpandResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// WarningError は警告エラーを結合して返す
func (r *ExpandResult) WarningError() error {
	return errors.Join(r.Warnings...)
}

// CriticalError は必須エントリのエラーを結合して返す
func (r *ExpandResult) CriticalError() error {
	return errors.Join(r.Errors...)
}

// ExpandIncludes はworktree作成時にinclude設定に従ってファイルを配置する
// worktreeDir: 対象worktreeのルートディレクトリ
// services: このworktreeに含まれるサービス名リスト
// include: IncludeConfig
// vars: template展開用の変数マップ (例: {"port": "3000", "services.api.port": "8080"})
func ExpandIncludes(worktreeDir string, services []string, include config.IncludeConfig, vars map[string]string) *ExpandResult {
	result := &ExpandResult{}

	// rootエントリをworktreeルートに配置する
	for _, entry := range include.Root {
		if err := processEntry(worktreeDir, worktreeDir, entry, vars); err != nil {
			wrapped := fmt.Errorf("root エントリ (source=%s): %w", entry.Source, err)
			if entry.Required {
				result.Errors = append(result.Errors, wrapped)
			} else {
				result.Warnings = append(result.Warnings, wrapped)
			}
		}
	}

	// per_serviceエントリを該当サービスのみに配置する
	for svc, entries := range include.PerService {
		// このworktreeに含まれるサービスか確認する
		if !containsService(services, svc) {
			continue
		}
		targetDir := filepath.Join(worktreeDir, svc)
		for _, entry := range entries {
			if err := processEntry(worktreeDir, targetDir, entry, vars); err != nil {
				wrapped := fmt.Errorf("per_service エントリ (service=%s, source=%s): %w", svc, entry.Source, err)
				if entry.Required {
					result.Errors = append(result.Errors, wrapped)
				} else {
					result.Warnings = append(result.Warnings, wrapped)
				}
			}
		}
	}

	return result
}

// processEntry は単一のIncludeEntryを処理する
// baseDir: ソースファイルの基準ディレクトリ
// targetDir: ターゲットファイルの基準ディレクトリ
func processEntry(baseDir, targetDir string, entry config.IncludeEntry, vars map[string]string) error {
	// ソース・ターゲットの絶対パスを解決する
	srcPath := resolvePath(baseDir, entry.Source)
	dstPath := resolvePath(targetDir, entry.Target)

	// ターゲットの親ディレクトリを作成する
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("ターゲットディレクトリの作成に失敗: %w", err)
	}

	// ソースの情報を取得する
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	// ディレクトリの場合はsymlinkのみ対応
	if srcInfo.IsDir() {
		if entry.Strategy != "symlink" {
			return fmt.Errorf("ディレクトリは symlink のみ対応: %q", entry.Strategy)
		}
		return createSymlink(srcPath, dstPath)
	}

	switch entry.Strategy {
	case "copy":
		return copyFile(srcPath, dstPath)
	case "symlink":
		return createSymlink(srcPath, dstPath)
	case "template":
		return expandTemplate(srcPath, dstPath, vars)
	default:
		return fmt.Errorf("未知のstrategy: %q (copy/symlink/template のいずれかを指定してください)", entry.Strategy)
	}
}

// copyFile はソースファイルをターゲットにコピーする
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("ソースファイルのオープンに失敗: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("ソースファイルの情報取得に失敗: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("ターゲットファイルのオープンに失敗: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("ファイルコピーに失敗: %w", err)
	}

	return nil
}

// createSymlink はソースへのシンボリックリンクをターゲットに作成する
// 相対パスが指定された場合は絶対パスに変換する
func createSymlink(src, dst string) error {
	// ソースパスを絶対パスに変換する
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("ソースパスの絶対パス変換に失敗: %w", err)
	}

	// 既存のシンボリックリンクや通常ファイルがあれば削除する
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("既存ターゲットの削除に失敗: %w", err)
	}

	if err := os.Symlink(absSrc, dst); err != nil {
		return fmt.Errorf("シンボリックリンクの作成に失敗: %w", err)
	}

	return nil
}

// expandTemplate はソースファイルを読み込み、変数を展開してターゲットに書き込む
// ${varname} 形式のプレースホルダーを vars マップで置換する
func expandTemplate(src, dst string, vars map[string]string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("テンプレートファイルの読み込みに失敗: %w", err)
	}

	content := string(data)
	// ${varname} を vars マップの値で置換する
	for k, v := range vars {
		placeholder := "${" + k + "}"
		content = strings.ReplaceAll(content, placeholder, v)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("ソースファイルの情報取得に失敗: %w", err)
	}

	if err := os.WriteFile(dst, []byte(content), srcInfo.Mode()); err != nil {
		return fmt.Errorf("テンプレート展開結果の書き込みに失敗: %w", err)
	}

	return nil
}

// resolvePath はbaseを基準にpathを解決した絶対パスを返す
// pathが絶対パスの場合はそのまま返す
func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, path)
}

// containsService はservicesスライスにtargetが含まれるか確認する
func containsService(services []string, target string) bool {
	for _, s := range services {
		if s == target {
			return true
		}
	}
	return false
}
