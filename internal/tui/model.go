package tui

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryugen04/sango-tree/internal/config"
	"github.com/ryugen04/sango-tree/internal/service"
	"github.com/ryugen04/sango-tree/internal/worktree"
)

// パネル識別
type panel int

const (
	panelWorktrees panel = iota
	panelServices
)

// ワークツリーのサービス情報
type wtEntry struct {
	Name     string
	Offset   int
	IsActive bool
	Services []service.ServiceInfo
}

// Model はTUIの状態
type Model struct {
	cfg       *config.Config
	cfgFile   string
	sangoDir  string

	worktrees []wtEntry
	shared    []service.ServiceInfo

	activePanel panel
	wtCursor    int
	svcCursor   int

	width  int
	height int

	statusMsg string
	running   bool // コマンド実行中
}

// メッセージ型
type statusRefreshed struct {
	worktrees []wtEntry
	shared    []service.ServiceInfo
}
type commandFinished struct {
	msg string
	err error
}
type tickMsg struct{}

// New はTUIモデルを作成する
func New(cfg *config.Config, cfgFile string) Model {
	return Model{
		cfg:         cfg,
		cfgFile:     cfgFile,
		sangoDir:    worktree.DefaultDir(),
		activePanel: panelWorktrees,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(refreshStatus(m.cfg, m.cfgFile, m.sangoDir), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// ステータス取得コマンド
func refreshStatus(cfg *config.Config, cfgFile, sangoDir string) tea.Cmd {
	return func() tea.Msg {
		ws, err := worktree.Load(sangoDir)
		if err != nil {
			return statusRefreshed{}
		}

		var entries []wtEntry
		for name, wt := range ws.Worktrees {
			orch, err := service.NewOrchestratorWithWorktree(cfg, cfgFile, service.OrchestratorOptions{WorktreeFlag: name})
			if err != nil {
				continue
			}
			result, err := orch.Status()
			if err != nil {
				continue
			}

			// サービスをフィルタ（repo-onlyは除外）
			var svcs []service.ServiceInfo
			for _, s := range result.Services {
				if !s.IsRepoOnly && !s.IsShared {
					svcs = append(svcs, s)
				}
			}

			entries = append(entries, wtEntry{
				Name:     name,
				Offset:   wt.Offset,
				IsActive: name == ws.Active,
				Services: svcs,
			})
		}

		// shared サービス取得（最初のworktreeから）
		var shared []service.ServiceInfo
		if len(ws.Worktrees) > 0 {
			for name := range ws.Worktrees {
				orch, err := service.NewOrchestratorWithWorktree(cfg, cfgFile, service.OrchestratorOptions{WorktreeFlag: name})
				if err != nil {
					continue
				}
				result, err := orch.Status()
				if err != nil {
					break
				}
				for _, s := range result.Services {
					if s.IsShared {
						shared = append(shared, s)
					}
				}
				break
			}
		}

		// activeを先頭にソート
		sortWorktrees(entries)

		return statusRefreshed{worktrees: entries, shared: shared}
	}
}

func sortWorktrees(entries []wtEntry) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			// activeを先頭に
			if entries[j].IsActive && !entries[i].IsActive {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.running {
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, keys.Tab):
			if m.activePanel == panelWorktrees {
				m.activePanel = panelServices
				m.svcCursor = 0
			} else {
				m.activePanel = panelWorktrees
			}

		case key.Matches(msg, keys.Up):
			if m.activePanel == panelWorktrees {
				if m.wtCursor > 0 {
					m.wtCursor--
					m.svcCursor = 0
				}
			} else {
				if m.svcCursor > 0 {
					m.svcCursor--
				}
			}

		case key.Matches(msg, keys.Down):
			if m.activePanel == panelWorktrees {
				if m.wtCursor < len(m.worktrees)-1 {
					m.wtCursor++
					m.svcCursor = 0
				}
			} else {
				svcs := m.currentServices()
				if m.svcCursor < len(svcs)-1 {
					m.svcCursor++
				}
			}

		case key.Matches(msg, keys.Start):
			return m, m.execServiceCmd("up")

		case key.Matches(msg, keys.Stop):
			return m, m.execServiceCmd("down")

		case key.Matches(msg, keys.Restart):
			return m, m.execServiceCmd("restart")

		case key.Matches(msg, keys.Logs):
			return m, m.execServiceCmd("logs")

		case key.Matches(msg, keys.StartAll):
			return m, m.execWorktreeCmd("up")

		case key.Matches(msg, keys.StopAll):
			return m, m.execWorktreeCmd("down")

		case key.Matches(msg, keys.Refresh):
			return m, refreshStatus(m.cfg, m.cfgFile, m.sangoDir)
		}

	case statusRefreshed:
		m.worktrees = msg.worktrees
		m.shared = msg.shared
		if m.wtCursor >= len(m.worktrees) {
			m.wtCursor = 0
		}
		return m, nil

	case commandFinished:
		m.running = false
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("[error] %v", msg.err)
		} else {
			m.statusMsg = msg.msg
		}
		return m, refreshStatus(m.cfg, m.cfgFile, m.sangoDir)

	case tickMsg:
		return m, tea.Batch(refreshStatus(m.cfg, m.cfgFile, m.sangoDir), tickCmd())
	}

	return m, nil
}

func (m *Model) currentServices() []service.ServiceInfo {
	if m.wtCursor >= len(m.worktrees) {
		return nil
	}
	return m.worktrees[m.wtCursor].Services
}

func (m *Model) currentWorktree() string {
	if m.wtCursor >= len(m.worktrees) {
		return ""
	}
	return m.worktrees[m.wtCursor].Name
}

func (m *Model) currentServiceName() string {
	svcs := m.currentServices()
	if m.svcCursor >= len(svcs) {
		return ""
	}
	return svcs[m.svcCursor].Name
}

// サービス単体コマンド実行
func (m *Model) execServiceCmd(action string) tea.Cmd {
	wt := m.currentWorktree()
	svc := m.currentServiceName()
	if wt == "" || svc == "" {
		return nil
	}
	if m.activePanel != panelServices {
		return nil
	}

	m.running = true
	m.statusMsg = fmt.Sprintf("[running] sango %s %s --worktree %s ...", action, svc, wt)

	return func() tea.Msg {
		args := []string{action, svc, "--worktree", wt, "--config", m.cfgFile}
		cmd := exec.Command("sango", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return commandFinished{msg: string(out), err: err}
		}
		return commandFinished{msg: fmt.Sprintf("[done] sango %s %s", action, svc)}
	}
}

// ワークツリー全体コマンド実行
func (m *Model) execWorktreeCmd(action string) tea.Cmd {
	wt := m.currentWorktree()
	if wt == "" {
		return nil
	}

	m.running = true
	m.statusMsg = fmt.Sprintf("[running] sango %s --worktree %s ...", action, wt)

	return func() tea.Msg {
		args := []string{action, "--worktree", wt, "--config", m.cfgFile}
		cmd := exec.Command("sango", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return commandFinished{msg: string(out), err: err}
		}
		return commandFinished{msg: fmt.Sprintf("[done] sango %s (worktree: %s)", action, wt)}
	}
}

// View はTUIを描画する
func (m Model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	// レイアウト計算
	leftWidth := 35
	rightWidth := m.width - leftWidth - 3
	if rightWidth < 30 {
		rightWidth = 30
	}
	contentHeight := m.height - 6 // ヘッダー + フッター + shared

	// スタイル
	activeStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Width(leftWidth).
		Height(contentHeight)

	inactiveStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(leftWidth).
		Height(contentHeight)

	activeRightStyle := activeStyle.Width(rightWidth)
	inactiveRightStyle := inactiveStyle.Width(rightWidth)

	// 左パネル: ワークツリー一覧
	leftStyle := inactiveStyle
	if m.activePanel == panelWorktrees {
		leftStyle = activeStyle
	}
	leftContent := m.renderWorktreeList(contentHeight)

	// 右パネル: サービス一覧
	rightStyle := inactiveRightStyle
	if m.activePanel == panelServices {
		rightStyle = activeRightStyle
	}
	rightContent := m.renderServiceList(contentHeight)

	// 組み立て
	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(leftContent),
		rightStyle.Render(rightContent),
	)

	// shared サービス行
	sharedLine := m.renderSharedLine()

	// ステータスバー
	statusBar := m.renderStatusBar()

	// ヘルプ行
	helpLine := m.renderHelp()

	return lipgloss.JoinVertical(lipgloss.Left,
		sharedLine,
		panels,
		statusBar,
		helpLine,
	)
}

func (m Model) renderSharedLine() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	var parts []string
	for _, s := range m.shared {
		status := "stopped"
		if s.Status == "running" {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("running")
		} else {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("stopped")
		}
		parts = append(parts, fmt.Sprintf("%s:%d %s", s.Name, s.Port, status))
	}
	if len(parts) == 0 {
		return ""
	}
	return dim.Render("shared: ") + strings.Join(parts, "  ")
}

func (m Model) renderWorktreeList(height int) string {
	title := lipgloss.NewStyle().Bold(true).Render("Worktrees")
	lines := []string{title, ""}

	for i, wt := range m.worktrees {
		cursor := "  "
		if i == m.wtCursor {
			cursor = "> "
		}

		marker := " "
		if wt.IsActive {
			marker = "*"
		}

		// 起動中サービス数
		running := 0
		for _, s := range wt.Services {
			if s.Status == "running" {
				running++
			}
		}

		name := wt.Name
		nameStyle := lipgloss.NewStyle()
		if i == m.wtCursor && m.activePanel == panelWorktrees {
			nameStyle = nameStyle.Foreground(lipgloss.Color("39")).Bold(true)
		}

		countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		line := fmt.Sprintf("%s%s %s %s",
			cursor,
			marker,
			nameStyle.Render(name),
			countStyle.Render(fmt.Sprintf("[%d/%d]", running, len(wt.Services))),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderServiceList(height int) string {
	wt := m.currentWorktree()
	if wt == "" {
		return "ワークツリーを選択してください"
	}

	title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Services (%s)", wt))
	lines := []string{title, ""}

	// ヘッダー
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	lines = append(lines, headerStyle.Render(
		fmt.Sprintf("  %-18s %-7s %-8s %-8s", "SERVICE", "PORT", "STATUS", "PID"),
	))

	svcs := m.currentServices()
	for i, s := range svcs {
		cursor := "  "
		if i == m.svcCursor && m.activePanel == panelServices {
			cursor = "> "
		}

		// ステータスの色分け
		var statusStr string
		switch s.Status {
		case "running":
			statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("running")
		case "stopped":
			statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("stopped")
		default:
			statusStr = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(s.Status)
		}

		pidStr := "-"
		if s.PID > 0 {
			pidStr = fmt.Sprintf("%d", s.PID)
		}

		nameStyle := lipgloss.NewStyle()
		if i == m.svcCursor && m.activePanel == panelServices {
			nameStyle = nameStyle.Foreground(lipgloss.Color("39")).Bold(true)
		}

		// ANSIエスケープのため固定幅は手動調整
		line := fmt.Sprintf("%s%-18s %-7d %s  %s",
			cursor,
			nameStyle.Render(s.Name),
			s.Port,
			statusStr,
			pidStr,
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderStatusBar() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MaxWidth(m.width)
	if m.statusMsg != "" {
		return style.Render(m.statusMsg)
	}
	return ""
}

func (m Model) renderHelp() string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	help := fmt.Sprintf(
		"%s navigate  %s switch  %s start  %s stop  %s restart  %s start all  %s stop all  %s refresh  %s quit",
		keyStyle.Render("j/k"),
		keyStyle.Render("tab"),
		keyStyle.Render("u"),
		keyStyle.Render("d"),
		keyStyle.Render("r"),
		keyStyle.Render("U"),
		keyStyle.Render("D"),
		keyStyle.Render("R"),
		keyStyle.Render("q"),
	)
	return dim.Render(help)
}
