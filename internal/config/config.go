package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config はgrove.yamlのルート構造体
type Config struct {
	Name     string              `yaml:"name"`
	Version  string              `yaml:"version"`
	Services map[string]*Service `yaml:"services"`
	Ports    PortConfig          `yaml:"ports"`
	Profiles map[string]Profile  `yaml:"profiles"`
	Doctor   DoctorConfig        `yaml:"doctor"`
	Worktree WorktreeConfig      `yaml:"worktree"`
}

// WorktreeConfig はワークツリー管理の設定
type WorktreeConfig struct {
	BaseDir       string        `yaml:"base_dir"`
	AutoSetup     bool          `yaml:"auto_setup"`
	DefaultBranch string        `yaml:"default_branch"`
	Include       IncludeConfig `yaml:"include"`
	Hooks         HooksConfig   `yaml:"hooks"`
}

// IncludeConfig はworktree作成時のファイル配置設定
type IncludeConfig struct {
	Common     []IncludeEntry            `yaml:"common"`
	PerService map[string][]IncludeEntry `yaml:"per_service"`
}

// IncludeEntry はinclude対象のファイル定義
type IncludeEntry struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	Strategy string `yaml:"strategy"` // copy | symlink | template
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
	RepoPath     string            `yaml:"repo_path"`
	RunOn        []string          `yaml:"run_on"`
	Troubleshoot []TroubleshootCheck `yaml:"troubleshoot"`
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

// TroubleshootCheck はトラブルシュート用チェック項目
type TroubleshootCheck struct {
	Name        string `yaml:"name"`
	Command     string `yaml:"command"`
	Description string `yaml:"description"`
}

// 許可されるサービスタイプ
var validServiceTypes = map[string]bool{
	"docker":  true,
	"process": true,
	"script":  true,
}

// Load はgrove.yamlを読み込んでConfigを返す
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

		// process/scriptタイプの場合Commandが必須
		if (svc.Type == "process" || svc.Type == "script") && svc.Command == "" {
			return fmt.Errorf("サービス %q: %sタイプにはcommandが必須です", name, svc.Type)
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
