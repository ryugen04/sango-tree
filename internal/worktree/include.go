package worktree

import (
	"crypto/sha256"
	"encoding/hex"
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
// projectRoot: プロジェクトルートディレクトリ（sourceの基準パス）
// worktreeDir: 対象worktreeのルートディレクトリ（targetの基準パス）
// services: このworktreeに含まれるサービス名リスト
// include: IncludeConfig
// vars: template展開用の変数マップ (例: {"port": "3000", "services.api.port": "8080"})
// sangoDir: sango設定ディレクトリ（テンプレートキャッシュ用）
func ExpandIncludes(projectRoot, worktreeDir string, services []string, include config.IncludeConfig, vars map[string]string, sangoDir string) *ExpandResult {
	result := &ExpandResult{}

	// rootエントリをworktreeルートに配置する
	// source はプロジェクトルート基準、target はworktreeDir基準
	for _, entry := range include.Root {
		if err := processEntry(projectRoot, worktreeDir, entry, vars, sangoDir); err != nil {
			wrapped := fmt.Errorf("root エントリ (source=%s): %w", entry.Source, err)
			if entry.Required {
				result.Errors = append(result.Errors, wrapped)
			} else {
				result.Warnings = append(result.Warnings, wrapped)
			}
		}
	}

	// per_serviceエントリを該当サービスのみに配置する
	// source はプロジェクトルート基準、target は worktreeDir/svc 基準
	for svc, entries := range include.PerService {
		// このworktreeに含まれるサービスか確認する
		if !containsService(services, svc) {
			continue
		}
		targetDir := filepath.Join(worktreeDir, svc)
		for _, entry := range entries {
			if err := processEntry(projectRoot, targetDir, entry, vars, sangoDir); err != nil {
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
// sangoDir: sango設定ディレクトリ（テンプレートキャッシュ用）
func processEntry(baseDir, targetDir string, entry config.IncludeEntry, vars map[string]string, sangoDir string) error {
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
		return expandTemplate(srcPath, dstPath, vars, sangoDir)
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
// テンプレート元のオリジナル内容をキャッシュして、次のworktree作成時に再利用する
func expandTemplate(src, dst string, vars map[string]string, sangoDir string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("ソースファイルの情報取得に失敗: %w", err)
	}

	srcContent, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("テンプレートファイルの読み込みに失敗: %w", err)
	}

	// キャッシュ用のキーを計算（ソースの絶対パスのSHA256）
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("ソースの絶対パス解決に失敗: %w", err)
	}

	cacheKey := sha256.Sum256([]byte(absSrc))
	cacheKeyHex := hex.EncodeToString(cacheKey[:])
	cachePath := filepath.Join(sangoDir, "template-cache", cacheKeyHex+".template")

	// テンプレート内容を決定
	var templateContent []byte

	// srcContentに${が含まれているかチェック
	if strings.Contains(string(srcContent), "${") {
		// srcはまだテンプレート（展開済みでない）→ キャッシュに保存
		templateContent = srcContent
		if err := saveCachedTemplate(cachePath, templateContent); err != nil {
			// キャッシュ保存失敗は警告にとどめる（致命的ではない）
			fmt.Fprintf(os.Stderr, "警告: テンプレートキャッシュの保存に失敗: %v\n", err)
		}
	} else {
		// srcは展開済み（値が置き換わっている）
		// キャッシュがあればそこから読む
		if cached, err := os.ReadFile(cachePath); err == nil {
			templateContent = cached
		} else {
			// キャッシュもない → フォールバック: srcをそのまま使う
			templateContent = srcContent
		}
	}

	// テンプレートを展開
	content := string(templateContent)
	for k, v := range vars {
		placeholder := "${" + k + "}"
		content = strings.ReplaceAll(content, placeholder, v)
	}

	if err := os.WriteFile(dst, []byte(content), srcInfo.Mode()); err != nil {
		return fmt.Errorf("テンプレート展開結果の書き込みに失敗: %w", err)
	}

	return nil
}

// saveCachedTemplate はテンプレートをキャッシュディレクトリに保存する
func saveCachedTemplate(cachePath string, content []byte) error {
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("キャッシュディレクトリ作成に失敗: %w", err)
	}

	if err := os.WriteFile(cachePath, content, 0o644); err != nil {
		return fmt.Errorf("キャッシュファイル書き込みに失敗: %w", err)
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
