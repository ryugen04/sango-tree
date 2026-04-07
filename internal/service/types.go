package service

// ServiceInfo はサービスの状態情報
type ServiceInfo struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	Port          int    `json:"port"`
	Status        string `json:"status"`
	Health        string `json:"health,omitempty"`
	PID           int    `json:"pid,omitempty"`
	RestartCount  int    `json:"restart_count,omitempty"`
	PortListening *bool  `json:"port_listening,omitempty"`
	IsRepoOnly    bool   `json:"is_repo_only,omitempty"`
	IsShared      bool   `json:"is_shared,omitempty"`
	OpenURL       string `json:"open_url,omitempty"`
}

// WorktreeInfo はワークツリーの概要情報
type WorktreeInfo struct {
	Name            string        `json:"name"`
	Offset          int           `json:"offset"`
	WebPort         int           `json:"web_port"`
	RunningServices int           `json:"running_services"`
	TotalServices   int           `json:"total_services"`
	Repos           []string      `json:"repos,omitempty"`
	Services        []ServiceInfo `json:"services,omitempty"`
}

// UpResult はサービス起動の結果
type UpResult struct {
	Started []ServiceInfo `json:"started"`
	Errors  []string      `json:"errors,omitempty"`
}

// DownResult はサービス停止の結果
type DownResult struct {
	Stopped []string `json:"stopped"`
	Errors  []string `json:"errors,omitempty"`
}

// StatusResult はサービス状態の結果
type StatusResult struct {
	Worktree  string         `json:"worktree"`
	Services  []ServiceInfo  `json:"services"`
	Worktrees []WorktreeInfo `json:"worktrees,omitempty"`
}
