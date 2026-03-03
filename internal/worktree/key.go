package worktree

import "strings"

const keySeparator = "___"

// ToKey はブランチ名を内部キーに変換する（PID, ロック用）
// 例: "feature/auth" → "feature___auth"
func ToKey(branch string) string {
	return strings.ReplaceAll(branch, "/", keySeparator)
}

// FromKey は内部キーをブランチ名に復元する
// 例: "feature___auth" → "feature/auth"
func FromKey(key string) string {
	return strings.ReplaceAll(key, keySeparator, "/")
}
