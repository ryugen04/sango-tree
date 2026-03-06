package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	sangoLog "github.com/ryugen04/sango-tree/internal/log"
	"github.com/ryugen04/sango-tree/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	logsSince  string
	logsGrep   string
	logsLevel  string
	logsFollow bool
	logsJSON   bool
	logsLimit  int
)

// サービスごとの色割り当て
var serviceColors = []lipgloss.Color{
	lipgloss.Color("6"),  // cyan
	lipgloss.Color("5"),  // magenta
	lipgloss.Color("3"),  // yellow
	lipgloss.Color("2"),  // green
	lipgloss.Color("4"),  // blue
	lipgloss.Color("13"), // bright magenta
	lipgloss.Color("14"), // bright cyan
	lipgloss.Color("12"), // bright blue
}

var logsCmd = &cobra.Command{
	Use:   "logs [services...]",
	Short: "サービスのログを表示する",
	RunE: func(cmd *cobra.Command, args []string) error {
		sangoDir := worktree.DefaultDir()
		wtName := resolveActiveWorktree(sangoDir)
		wtKey := worktree.ToKey(wtName)

		filter := sangoLog.Filter{
			Services: args,
			Worktree: wtKey,
			Level:    logsLevel,
			Grep:     logsGrep,
			Limit:    logsLimit,
		}

		if logsSince != "" {
			since, err := parseDuration(logsSince)
			if err != nil {
				return fmt.Errorf("--since の解析に失敗: %w", err)
			}
			filter.Since = since
		}

		if logsFollow {
			return followLogs(sangoDir, filter)
		}

		entries, err := sangoLog.ReadLogs(sangoDir, filter)
		if err != nil {
			return fmt.Errorf("ログの読み込みに失敗: %w", err)
		}

		colorMap := buildColorMap(entries)
		for _, entry := range entries {
			printEntry(entry, colorMap)
		}

		return nil
	},
}

func init() {
	logsCmd.Flags().StringVar(&logsSince, "since", "", "時間フィルタ (例: 5m, 1h, 30s)")
	logsCmd.Flags().StringVar(&logsGrep, "grep", "", "テキストフィルタ (正規表現)")
	logsCmd.Flags().StringVar(&logsLevel, "level", "", "ログレベルフィルタ (info/warn/error/debug)")
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "リアルタイムフォロー")
	logsCmd.Flags().BoolVar(&logsJSON, "json", false, "JSON出力 (生JSONL)")
	logsCmd.Flags().IntVarP(&logsLimit, "tail", "n", 0, "表示行数制限")
	rootCmd.AddCommand(logsCmd)
}

func followLogs(sangoDir string, filter sangoLog.Filter) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	ch, err := sangoLog.FollowLogs(ctx, sangoDir, filter)
	if err != nil {
		return err
	}

	colorMap := make(map[string]lipgloss.Style)
	colorIdx := 0

	for entry := range ch {
		if _, ok := colorMap[entry.Service]; !ok {
			colorMap[entry.Service] = lipgloss.NewStyle().Foreground(serviceColors[colorIdx%len(serviceColors)])
			colorIdx++
		}
		printEntry(entry, colorMap)
	}

	return nil
}

func printEntry(entry sangoLog.LogEntry, colorMap map[string]lipgloss.Style) {
	if logsJSON {
		data, _ := json.Marshal(entry)
		fmt.Println(string(data))
		return
	}

	tsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	ts := tsStyle.Render(entry.Timestamp.Format("15:04:05"))

	svcStyle := colorMap[entry.Service]
	svc := svcStyle.Render(fmt.Sprintf("[%-10s]", entry.Service))

	level := formatLevel(entry.Level)

	fmt.Printf("%s %s %s %s\n", ts, svc, level, entry.Message)
}

func formatLevel(level string) string {
	upper := strings.ToUpper(level)
	padded := fmt.Sprintf("%-5s", upper)
	switch level {
	case "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Render(padded)
	case "warn":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(padded)
	case "debug":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(padded)
	default:
		return padded
	}
}

func buildColorMap(entries []sangoLog.LogEntry) map[string]lipgloss.Style {
	colorMap := make(map[string]lipgloss.Style)
	colorIdx := 0
	for _, e := range entries {
		if _, ok := colorMap[e.Service]; !ok {
			colorMap[e.Service] = lipgloss.NewStyle().Foreground(serviceColors[colorIdx%len(serviceColors)])
			colorIdx++
		}
	}
	return colorMap
}

// parseDuration は "5m", "1h", "30s", "7d" 形式の文字列をtime.Timeに変換する
func parseDuration(s string) (time.Time, error) {
	// "d"サフィックスの対応
	re := regexp.MustCompile(`^(\d+)d$`)
	if m := re.FindStringSubmatch(s); m != nil {
		days, _ := strconv.Atoi(m[1])
		return time.Now().Add(-time.Duration(days) * 24 * time.Hour), nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Now().Add(-d), nil
}
