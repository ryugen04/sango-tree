package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadEnvFile はKEY=VALUE形式の.envファイルを読み込む
func LoadEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf(".envファイルの読み込みに失敗: %w", err)
	}
	defer f.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 空行・コメントをスキップ
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// KEY=VALUE の分割（最初の=で分割）
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		// クォート除去
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		env[key] = value
	}
	return env, scanner.Err()
}
