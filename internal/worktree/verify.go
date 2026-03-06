package worktree

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryugen04/sango-tree/internal/config"
)

// VerifyStatus はincludeエントリの検証結果ステータス
type VerifyStatus string

const (
	VerifyOK         VerifyStatus = "ok"
	VerifyMissing    VerifyStatus = "missing"
	VerifyMismatch   VerifyStatus = "mismatch"
	VerifyBrokenLink VerifyStatus = "broken_link"
)

// VerifyEntry はincludeエントリの検証結果
type VerifyEntry struct {
	Service  string             // "" ならrootエントリ
	Entry    config.IncludeEntry
	Status   VerifyStatus
	Detail   string
}

// VerifyIncludes はworktreeのinclude状態を検証する
func VerifyIncludes(worktreeDir string, services []string, include config.IncludeConfig, vars map[string]string) []VerifyEntry {
	var results []VerifyEntry

	// rootエントリを検証する
	for _, entry := range include.Root {
		ve := verifyEntry(worktreeDir, worktreeDir, "", entry, vars)
		results = append(results, ve)
	}

	// per_serviceエントリを検証する
	for svc, entries := range include.PerService {
		if !containsService(services, svc) {
			continue
		}
		targetDir := filepath.Join(worktreeDir, svc)
		for _, entry := range entries {
			ve := verifyEntry(worktreeDir, targetDir, svc, entry, vars)
			results = append(results, ve)
		}
	}

	return results
}

// verifyEntry は単一のIncludeEntryを検証する
func verifyEntry(baseDir, targetDir, service string, entry config.IncludeEntry, vars map[string]string) VerifyEntry {
	srcPath := resolvePath(baseDir, entry.Source)
	dstPath := resolvePath(targetDir, entry.Target)

	ve := VerifyEntry{
		Service: service,
		Entry:   entry,
	}

	switch entry.Strategy {
	case "symlink":
		ve.Status, ve.Detail = verifySymlink(srcPath, dstPath)
	case "copy":
		ve.Status, ve.Detail = verifyCopy(srcPath, dstPath)
	case "template":
		ve.Status, ve.Detail = verifyTemplate(srcPath, dstPath, vars)
	default:
		ve.Status = VerifyMismatch
		ve.Detail = "未知のstrategy: " + entry.Strategy
	}

	return ve
}

// verifySymlink はsymlinkエントリを検証する
func verifySymlink(srcPath, dstPath string) (VerifyStatus, string) {
	// ターゲットがsymlinkか確認する
	info, err := os.Lstat(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			return VerifyMissing, "ターゲットが存在しない"
		}
		return VerifyMissing, err.Error()
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return VerifyMismatch, "symlinkではなく通常ファイル"
	}

	// リンク先がソースの絶対パスと一致するか確認する
	linkTarget, err := os.Readlink(dstPath)
	if err != nil {
		return VerifyBrokenLink, err.Error()
	}

	absSrc, err := filepath.Abs(srcPath)
	if err != nil {
		return VerifyMismatch, err.Error()
	}

	if linkTarget != absSrc {
		return VerifyMismatch, "リンク先が不一致: " + linkTarget
	}

	// リンク先が実在するか確認する
	if _, err := os.Stat(dstPath); err != nil {
		return VerifyBrokenLink, "リンク先が存在しない"
	}

	return VerifyOK, ""
}

// verifyCopy はcopyエントリを検証する
func verifyCopy(srcPath, dstPath string) (VerifyStatus, string) {
	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			return VerifyMissing, "ターゲットが存在しない"
		}
		return VerifyMissing, err.Error()
	}

	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return VerifyMismatch, "ソースの読み込みに失敗: " + err.Error()
	}

	if !bytes.Equal(srcData, dstData) {
		return VerifyMismatch, "内容が不一致"
	}

	return VerifyOK, ""
}

// verifyTemplate はtemplateエントリを検証する
func verifyTemplate(srcPath, dstPath string, vars map[string]string) (VerifyStatus, string) {
	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			return VerifyMissing, "ターゲットが存在しない"
		}
		return VerifyMissing, err.Error()
	}

	srcData, err := os.ReadFile(srcPath)
	if err != nil {
		return VerifyMismatch, "ソースの読み込みに失敗: " + err.Error()
	}

	// テンプレート展開後の期待値を計算する
	expected := string(srcData)
	for k, v := range vars {
		placeholder := "${" + k + "}"
		expected = strings.ReplaceAll(expected, placeholder, v)
	}

	if string(dstData) != expected {
		return VerifyMismatch, "テンプレート展開後の内容が不一致"
	}

	return VerifyOK, ""
}
