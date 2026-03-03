package port

import (
	"fmt"
	"net"
)

// Manager はポートの割り当てと衝突チェックを管理する
type Manager struct {
	reserved map[int]bool
	rangeMin int
	rangeMax int
}

// NewManager はPortManagerを生成する
func NewManager(rangeMin, rangeMax int, reserved []int) *Manager {
	r := make(map[int]bool)
	for _, p := range reserved {
		r[p] = true
	}
	return &Manager{
		reserved: r,
		rangeMin: rangeMin,
		rangeMax: rangeMax,
	}
}

// IsAvailable は指定ポートが使用可能かチェックする（net.Listenで確認）
func (m *Manager) IsAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// IsReserved は指定ポートが予約済みかチェックする
func (m *Manager) IsReserved(port int) bool {
	return m.reserved[port]
}

// InRange は指定ポートが許可範囲内かチェックする
func (m *Manager) InRange(port int) bool {
	return port >= m.rangeMin && port <= m.rangeMax
}

// ValidateAll はサービスのポートマップを検証する
// ポートの重複、範囲外、使用中をチェック
func (m *Manager) ValidateAll(ports map[string]int) []error {
	var errs []error

	// 重複チェック
	seen := make(map[int]string)
	for svc, p := range ports {
		if prev, ok := seen[p]; ok {
			errs = append(errs, fmt.Errorf("ポート %d が重複しています: %s と %s", p, prev, svc))
		}
		seen[p] = svc
	}

	for svc, p := range ports {
		// 範囲チェック
		if !m.InRange(p) {
			errs = append(errs, fmt.Errorf("ポート %d (%s) は許可範囲 %d-%d の外です", p, svc, m.rangeMin, m.rangeMax))
			continue
		}
		// 予約チェック
		if m.IsReserved(p) {
			errs = append(errs, fmt.Errorf("ポート %d (%s) は予約済みです", p, svc))
			continue
		}
		// 使用中チェック
		if !m.IsAvailable(p) {
			errs = append(errs, fmt.Errorf("ポート %d (%s) は既に使用中です", p, svc))
		}
	}

	return errs
}

// ServicePortInfo はポート解決に必要なサービス情報
type ServicePortInfo struct {
	Port   int
	Shared bool
}

// ResolvePort はベースポートにオフセットを加算する。sharedならオフセット無し
func ResolvePort(basePort, offset int, shared bool) int {
	if shared {
		return basePort
	}
	return basePort + offset
}

// ResolveAllPorts はサービス群のポートにオフセットを適用したマップを返す
// configパッケージとの循環インポートを避けるため、スタンドアロン関数として実装
func ResolveAllPorts(services map[string]ServicePortInfo, offset int) map[string]int {
	result := make(map[string]int, len(services))
	for name, svc := range services {
		result[name] = ResolvePort(svc.Port, offset, svc.Shared)
	}
	return result
}
