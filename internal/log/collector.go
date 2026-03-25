package log

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Collector はサービスのstdout/stderrをJSONLファイルに収集する
type Collector struct {
	logDir   string
	service  string
	worktree string
	file     *os.File
	mu       sync.Mutex

	// ローテーション設定
	maxSize  int64
	maxFiles int

	// パイプの参照（Close時に閉じる）
	stdoutPipeW *os.File
	stderrPipeW *os.File
}

// NewCollector はログコレクターを生成する
func NewCollector(sangoDir, worktree, service string) (*Collector, error) {
	logDir := filepath.Join(sangoDir, "logs", worktree)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, service+".jsonl")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return &Collector{
		logDir:   logDir,
		service:  service,
		worktree: worktree,
		file:     f,
		maxSize:  50 * 1024 * 1024, // 50MB
		maxFiles: 5,
	}, nil
}

// StdoutFile はcmd.Stdoutに設定する*os.Fileを返す
// ログファイルに直接書き込むことで、パイプによるEPIPE/SIGPIPEを完全に防ぐ
func (c *Collector) StdoutFile() (*os.File, error) {
	return c.file, nil
}

// StderrFile はcmd.Stderrに設定する*os.Fileを返す
// stdoutと同じログファイルに書き込む
func (c *Collector) StderrFile() (*os.File, error) {
	return c.file, nil
}

// StdoutWriter はcmd.Stdoutに設定するio.Writerを返す（後方互換性用）
func (c *Collector) StdoutWriter() (io.Writer, error) {
	return c.StdoutFile()
}

// StderrWriter はcmd.Stderrに設定するio.Writerを返す（後方互換性用）
func (c *Collector) StderrWriter() (io.Writer, error) {
	return c.StderrFile()
}

// scanAndWrite はパイプから行を読み取り、ターミナルとJSONLファイルの両方に書き込む
func (c *Collector) scanAndWrite(r io.Reader, stream string, terminal *os.File) {
	scanner := bufio.NewScanner(r)
	// 64KBバッファで長い行にも対応
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		// ターミナルに出力
		terminal.WriteString(line + "\n")

		// JSONL書き込み
		entry := &LogEntry{
			Timestamp: time.Now(),
			Service:   c.service,
			Worktree:  c.worktree,
			Stream:    stream,
			Level:     DetectLevel(line, stream),
			Message:   line,
		}

		data, err := entry.Marshal()
		if err != nil {
			// マーシャリング失敗時はフォールバック
			fallback := fmt.Sprintf(`{"ts":"%s","svc":"%s","stream":"%s","msg":"[marshal error] %s"}`,
				time.Now().Format(time.RFC3339), c.service, stream, line)
			data = []byte(fallback)
		}

		c.mu.Lock()
		c.writeWithRotation(append(data, '\n'))
		c.mu.Unlock()
	}
}

// writeWithRotation はサイズチェック付きでファイルに書き込む
func (c *Collector) writeWithRotation(data []byte) {
	info, err := c.file.Stat()
	if err == nil && info.Size()+int64(len(data)) > c.maxSize {
		c.rotate()
	}
	c.file.Write(data)
}

// rotate はログファイルをローテーションする
func (c *Collector) rotate() {
	basePath := filepath.Join(c.logDir, c.service+".jsonl")

	// 新ファイルを先に開く準備のため、現在のファイルを閉じてローテーション
	c.file.Close()
	Rotate(basePath, c.maxFiles)

	f, err := os.OpenFile(basePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		// 失敗時は /dev/null に書き込みを逃がす
		f, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
	c.file = f
}

// SetRotationConfig はローテーション設定を変更する
func (c *Collector) SetRotationConfig(maxSize int64, maxFiles int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxSize = maxSize
	c.maxFiles = maxFiles
}

// Close はログファイルとパイプを閉じる
func (c *Collector) Close() error {
	if c.stdoutPipeW != nil {
		c.stdoutPipeW.Close()
	}
	if c.stderrPipeW != nil {
		c.stderrPipeW.Close()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.file.Close()
}
