package log

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// Rotate はログファイルをローテーションする
// service.jsonl → service.jsonl.1 → service.jsonl.2 ...
func Rotate(basePath string, maxFiles int) {
	// 最も古いファイルを削除
	oldest := fmt.Sprintf("%s.%d", basePath, maxFiles)
	os.Remove(oldest)
	os.Remove(oldest + ".gz")

	// 既存ファイルを1つずつ繰り上げ
	for i := maxFiles - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", basePath, i)
		to := fmt.Sprintf("%s.%d", basePath, i+1)
		// gzファイルの場合
		if _, err := os.Stat(from + ".gz"); err == nil {
			os.Rename(from+".gz", to+".gz")
			continue
		}
		os.Rename(from, to)
	}

	// 現在のファイルを .1 にリネーム
	os.Rename(basePath, basePath+".1")
}

// CompressFile はファイルをgzip圧縮する
func CompressFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(path + ".gz")
	if err != nil {
		return err
	}
	defer dst.Close()

	gz := gzip.NewWriter(dst)
	defer gz.Close()

	if _, err := io.Copy(gz, src); err != nil {
		return err
	}

	if err := gz.Close(); err != nil {
		return err
	}

	return os.Remove(path)
}
