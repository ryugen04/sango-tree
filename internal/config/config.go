package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config はsango.yamlのルート構造体
type Config struct {
	Name     string              `yaml:"name"`
	Version  string              `yaml:"version"`
	Services map[string]*Service `yaml:"services"`
	Ports    PortConfig          `yaml:"ports"`
	Profiles map[string]Profile  `yaml:"profiles"`
	Doctor   DoctorConfig        `yaml:"doctor"`
	Worktree WorktreeConfig      `yaml:"worktree"`
	Log      LogConfig           `yaml:"log"`
}

// LogConfig はログ管理の設定
type LogConfig struct {
	MaxSize  string `yaml:"max_size"`  // "50MB"
	MaxFiles int    `yaml:"max_files"` // 5
	MaxAge   string `yaml:"max_age"`   // "7d"
	Compress bool   `yaml:"compress"`  // gzip圧縮
}

// WorktreeConfig はワークツリー管理の設定
type WorktreeConfig struct {
	BaseDir       string        `yaml:"base_dir"`
	AutoSetup     bool          `yaml:"auto_setup"`
	DefaultBranch string        `yaml:"default_branch"`
	Include       IncludeConfig `yaml:"include"`
	Hooks         HooksConfig   `yaml:"hooks"`
}

// ResolveBaseDir はworktreeのベースディレクトリを返す
// 未設定の場合はデフォルト "worktrees" を返す
func (w *WorktreeConfig) ResolveBaseDir() string {
	if w.BaseDir != "" {
		return w.BaseDir
	}
	return "worktrees"
}

// WorktreeDir はworktree名からディレクトリパスを返す
// 例: WorktreeDir("main") → "worktrees/main"
func (w *WorktreeConfig) WorktreeDir(wtName string) string {
	return filepath.Join(w.ResolveBaseDir(), wtName)
}

// IncludeConfig はworktree作成時のファイル配置設定
type IncludeConfig struct {
	Root       []IncludeEntry            `yaml:"root"`
	PerService map[string][]IncludeEntry `yaml:"per_service"`
}

// IncludeEntry はinclude対象のファイル定義
type IncludeEntry struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	Strategy string `yaml:"strategy"` // copy | symlink | template
	Required bool   `yaml:"required"` // trueなら失敗時にworktree作成を中止
}

// HooksConfig はworktreeライフサイクルのフック設定
type HooksConfig struct {
	PostCreate []HookEntry `yaml:"post_create"`
	PreRemove  []HookEntry `yaml:"pre_remove"`
}

// HookEntry はフック実行定義
type HookEntry struct {
	Command    string `yaml:"command"`
	PerService bool   `yaml:"per_service"`
}

// Service は個別サービスの定義
type Service struct {
	Type         string            `yaml:"type"`
	Image        string            `yaml:"image"`
	Port         int               `yaml:"port"`
	Shared       bool              `yaml:"shared"`
	DependsOn    []string          `yaml:"depends_on"`
	WorkingDir   string            `yaml:"working_dir"`
	Setup        []string          `yaml:"setup"`
	Command      string            `yaml:"command"`
	CommandArgs  []string          `yaml:"command_args"`
	Env          map[string]string `yaml:"env"`
	EnvFile      string            `yaml:"env_file"`
	EnvDynamic   map[string]string `yaml:"env_dynamic"`
	Healthcheck  *Healthcheck      `yaml:"healthcheck"`
	Restart      string            `yaml:"restart"`
	RestartDelay string            `yaml:"restart_delay"`
	MaxRestarts  int               `yaml:"max_restarts"`
	Volumes      []string          `yaml:"volumes"`
	Repo         string            `yaml:"repo"`
	RepoName     string            `yaml:"repo_name"`
	RepoPath     string            `yaml:"repo_path"`
	RunOn        []string          `yaml:"run_on"`
	Troubleshoot []TroubleshootCheck `yaml:"troubleshoot"`
	Runbook      []RunbookEntry      `yaml:"runbook"`
}

// Healthcheck はヘルスチェック設定
type Healthcheck struct {
	Command     string `yaml:"command"`
	URL         string `yaml:"url"`
	Interval    string `yaml:"interval"`
	Timeout     string `yaml:"timeout"`
	Retries     int    `yaml:"retries"`
	StartPeriod string `yaml:"start_period"`
}

// PortConfig はポート割り当て設定
type PortConfig struct {
	Strategy   string `yaml:"strategy"`
	BaseOffset int    `yaml:"base_offset"`
	Reserved   []int  `yaml:"reserved"`
	Range      [2]int `yaml:"range"`
}

// Profile はサービスのグループ定義
type Profile struct {
	Services []string `yaml:"services"`
}

// DoctorConfig は環境チェック設定
type DoctorConfig struct {
	Checks []DoctorCheck `yaml:"checks"`
}

// DoctorCheck は個別の環境チェック項目
type DoctorCheck struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
	Expect  string `yaml:"expect"`
	Fix     string `yaml:"fix"`
}

// RunbookEntry はサービス固有のナレッジベースエントリ
type RunbookEntry struct {
	Title    string   `yaml:"title"`
	Symptoms []string `yaml:"symptoms"`
	Cause    string   `yaml:"cause"`
	Steps    []string `yaml:"steps"`
	Tags     []string `yaml:"tags"`
}

// TroubleshootCheck はトラブルシュート用チェック項目
type TroubleshootCheck struct {
	Name        string `yaml:"name"`
	Command     string `yaml:"command"`
	Description string `yaml:"description"`
	Expect      string `yaml:"expect"`
	Fix         string `yaml:"fix"`
}

// ParseInterval はintervalをtime.Durationに変換する。デフォルト5s
func (h *Healthcheck) ParseInterval() time.Duration {
	if h.Interval == "" {
		return 5 * time.Second
	}
	d, err := time.ParseDuration(h.Interval)
	if err != nil {
		return 5 * time.Second
	}
	return d
}

// ParseTimeout はtimeoutをtime.Durationに変換する。デフォルト3s
func (h *Healthcheck) ParseTimeout() time.Duration {
	if h.Timeout == "" {
		return 3 * time.Second
	}
	d, err := time.ParseDuration(h.Timeout)
	if err != nil {
		return 3 * time.Second
	}
	return d
}

// ParseStartPeriod はstart_periodをtime.Durationに変換する。デフォルト0
func (h *Healthcheck) ParseStartPeriod() time.Duration {
	if h.StartPeriod == "" {
		return 0
	}
	d, err := time.ParseDuration(h.StartPeriod)
	if err != nil {
		return 0
	}
	return d
}

// ParseRestartDelay はrestart_delayをtime.Durationに変換する。デフォルト1s
func (s *Service) ParseRestartDelay() time.Duration {
	if s.RestartDelay == "" {
		return 1 * time.Second
	}
	d, err := time.ParseDuration(s.RestartDelay)
	if err != nil {
		return 1 * time.Second
	}
	return d
}

// 許可されるサービスタイプ
var validServiceTypes = map[string]bool{
	"docker":  true,
	"process": true,
	"script":  true,
}

// Load はsango.yamlを読み込んでConfigを返す
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("設定ファイルの読み込みに失敗: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("YAMLパースに失敗: %w", err)
	}

	return &cfg, nil
}

// Validate は設定の妥当性を検証する
func (c *Config) Validate() error {
	// Nameが空でないこと
	if c.Name == "" {
		return fmt.Errorf("name は必須です")
	}

	for name, svc := range c.Services {
		// Typeが有効か
		if !validServiceTypes[svc.Type] {
			return fmt.Errorf("サービス %q: 不正なtype %q (docker/process/scriptのいずれか)", name, svc.Type)
		}

		// dockerタイプの場合Imageが必須
		if svc.Type == "docker" && svc.Image == "" {
			return fmt.Errorf("サービス %q: dockerタイプにはimageが必須です", name)
		}

		// process/scriptタイプの場合Commandが必須（repoのみのサービスは除外）
		if (svc.Type == "process" || svc.Type == "script") && svc.Command == "" && svc.Repo == "" {
			return fmt.Errorf("サービス %q: %sタイプにはcommandが必須です", name, svc.Type)
		}

		// repo_nameの参照先が存在するか検証
		if svc.RepoName != "" {
			if _, exists := c.Services[svc.RepoName]; !exists {
				return fmt.Errorf("サービス %q: repo_nameに未定義のサービス %q が指定されています", name, svc.RepoName)
			}
		}

		// DependsOnに存在しないサービス名がないこと
		for _, dep := range svc.DependsOn {
			if _, exists := c.Services[dep]; !exists {
				return fmt.Errorf("サービス %q: depends_onに未定義のサービス %q が指定されています", name, dep)
			}
		}
	}

	// PortConfig.Rangeの検証
	if c.Ports.Range != [2]int{0, 0} && c.Ports.Range[0] >= c.Ports.Range[1] {
		return fmt.Errorf("ports.range: 開始値(%d)は終了値(%d)より小さくなければなりません", c.Ports.Range[0], c.Ports.Range[1])
	}

	return nil
}
