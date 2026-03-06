package log

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Filter はログの読み込みフィルタ
type Filter struct {
	Services []string
	Worktree string
	Since    time.Time
	Until    time.Time
	Grep     string
	Level    string
	Limit    int
}

// compileGrep はgrepパターンをコンパイルする
func compileGrep(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("grepパターンが不正: %w", err)
	}
	return re, nil
}

// ReadLogs はフィルタに従ってログを読み込む
func ReadLogs(sangoDir string, filter Filter) ([]LogEntry, error) {
	grepRe, err := compileGrep(filter.Grep)
	if err != nil {
		return nil, err
	}

	logsDir := filepath.Join(sangoDir, "logs")
	files, err := findLogFiles(logsDir, filter)
	if err != nil {
		return nil, err
	}

	var entries []LogEntry
	for _, f := range files {
		fileEntries, err := readJSONLFile(f, filter, grepRe)
		if err != nil {
			continue
		}
		entries = append(entries, fileEntries...)
	}

	// タイムスタンプでソート
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	// Limit適用
	if filter.Limit > 0 && len(entries) > filter.Limit {
		entries = entries[len(entries)-filter.Limit:]
	}

	return entries, nil
}

// FollowLogs はリアルタイムでログをフォローする
func FollowLogs(ctx context.Context, sangoDir string, filter Filter) (<-chan LogEntry, error) {
	grepRe, err := compileGrep(filter.Grep)
	if err != nil {
		return nil, err
	}

	logsDir := filepath.Join(sangoDir, "logs")
	files, err := findLogFiles(logsDir, filter)
	if err != nil {
		return nil, err
	}

	ch := make(chan LogEntry, 100)

	go func() {
		defer close(ch)

		type fileState struct {
			path   string
			offset int64
		}
		states := make([]fileState, 0, len(files))
		for _, f := range files {
			info, err := os.Stat(f)
			if err != nil {
				continue
			}
			states = append(states, fileState{path: f, offset: info.Size()})
		}

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 新しいファイルの検出
				currentFiles, _ := findLogFiles(logsDir, filter)
				for _, cf := range currentFiles {
					found := false
					for _, s := range states {
						if s.path == cf {
							found = true
							break
						}
					}
					if !found {
						states = append(states, fileState{path: cf, offset: 0})
					}
				}

				for i := range states {
					entries, newOffset := readFromOffset(states[i].path, states[i].offset, filter, grepRe)
					for _, entry := range entries {
						select {
						case ch <- entry:
						case <-ctx.Done():
							return
						}
					}
					states[i].offset = newOffset
				}
			}
		}
	}()

	return ch, nil
}

// findLogFiles はフィルタに合致するJSONLファイルのパスを返す
func findLogFiles(logsDir string, filter Filter) ([]string, error) {
	var files []string

	wtDirs, err := os.ReadDir(logsDir)
	if err != nil {
		return nil, err
	}

	for _, wtDir := range wtDirs {
		if !wtDir.IsDir() {
			continue
		}

		if filter.Worktree != "" && wtDir.Name() != filter.Worktree {
			continue
		}

		serviceFiles, err := os.ReadDir(filepath.Join(logsDir, wtDir.Name()))
		if err != nil {
			continue
		}

		for _, sf := range serviceFiles {
			if sf.IsDir() || !strings.HasSuffix(sf.Name(), ".jsonl") {
				continue
			}

			serviceName := strings.TrimSuffix(sf.Name(), ".jsonl")

			if len(filter.Services) > 0 {
				found := false
				for _, s := range filter.Services {
					if s == serviceName {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			files = append(files, filepath.Join(logsDir, wtDir.Name(), sf.Name()))
		}
	}

	return files, nil
}

// readJSONLFile はJSONLファイルを読んでフィルタ適用後のエントリを返す
func readJSONLFile(path string, filter Filter, grepRe *regexp.Regexp) ([]LogEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		entry, err := UnmarshalEntry(scanner.Bytes())
		if err != nil {
			continue
		}

		if matchesFilter(entry, filter, grepRe) {
			entries = append(entries, *entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("ログファイル %s の読み込み中にエラー: %w", path, err)
	}

	return entries, nil
}

// readFromOffset はファイルの指定オフセットから新しいエントリを読む
// 実際に読み取った位置を返す
func readFromOffset(path string, offset int64, filter Filter, grepRe *regexp.Regexp) ([]LogEntry, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset
	}

	var entries []LogEntry
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		entry, err := UnmarshalEntry(scanner.Bytes())
		if err != nil {
			continue
		}
		if matchesFilter(entry, filter, grepRe) {
			entries = append(entries, *entry)
		}
	}

	// 実際のファイル位置を取得
	newOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		newOffset = offset
	}

	return entries, newOffset
}

// matchesFilter はエントリがフィルタ条件に合致するか判定する
func matchesFilter(entry *LogEntry, filter Filter, grepRe *regexp.Regexp) bool {
	if !filter.Since.IsZero() && entry.Timestamp.Before(filter.Since) {
		return false
	}
	if !filter.Until.IsZero() && entry.Timestamp.After(filter.Until) {
		return false
	}
	if filter.Level != "" && entry.Level != filter.Level {
		return false
	}
	if grepRe != nil && !grepRe.MatchString(entry.Message) {
		return false
	}
	return true
}
