package port

import (
	"net"
	"testing"
)

func TestIsReserved(t *testing.T) {
	m := NewManager(3000, 9000, []int{80, 443, 5432})

	if !m.IsReserved(80) {
		t.Error("80は予約済みであるべき")
	}
	if !m.IsReserved(443) {
		t.Error("443は予約済みであるべき")
	}
	if m.IsReserved(8080) {
		t.Error("8080は予約済みでないべき")
	}
}

func TestInRange(t *testing.T) {
	m := NewManager(3000, 9000, nil)

	tests := []struct {
		port int
		want bool
	}{
		{2999, false},
		{3000, true},
		{5000, true},
		{9000, true},
		{9001, false},
	}
	for _, tt := range tests {
		if got := m.InRange(tt.port); got != tt.want {
			t.Errorf("InRange(%d) = %v, want %v", tt.port, got, tt.want)
		}
	}
}

func TestIsAvailable(t *testing.T) {
	m := NewManager(3000, 60000, nil)

	// テスト用にリスナーを立ててポートを塞ぐ
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	busyPort := ln.Addr().(*net.TCPAddr).Port

	if m.IsAvailable(busyPort) {
		t.Errorf("ポート %d は使用中なのにAvailableと判定された", busyPort)
	}

	// 閉じたら使用可能になる
	ln.Close()
	if !m.IsAvailable(busyPort) {
		t.Errorf("ポート %d は閉じた後なのにUnavailableと判定された", busyPort)
	}
}

func TestValidateAll_Duplicate(t *testing.T) {
	m := NewManager(3000, 9000, nil)

	ports := map[string]int{
		"api":    3000,
		"worker": 3000,
	}

	errs := m.ValidateAll(ports)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Error("重複ポートでエラーが返されるべき")
	}
}

func TestValidateAll_OutOfRange(t *testing.T) {
	m := NewManager(3000, 9000, nil)

	ports := map[string]int{
		"api": 80,
	}

	errs := m.ValidateAll(ports)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Error("範囲外ポートでエラーが返されるべき")
	}
}

func TestResolvePort(t *testing.T) {
	tests := []struct {
		name     string
		basePort int
		offset   int
		shared   bool
		want     int
	}{
		// 非sharedの場合はオフセットを加算する
		{name: "非shared・オフセット10", basePort: 3000, offset: 10, shared: false, want: 3010},
		// sharedの場合はオフセットを無視してベースポートをそのまま返す
		{name: "shared・オフセット10", basePort: 3000, offset: 10, shared: true, want: 3000},
		// オフセット0の場合は常にベースポートと同じ
		{name: "オフセット0", basePort: 5000, offset: 0, shared: false, want: 5000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePort(tt.basePort, tt.offset, tt.shared)
			if got != tt.want {
				t.Errorf("ResolvePort(%d, %d, %v) = %d, want %d", tt.basePort, tt.offset, tt.shared, got, tt.want)
			}
		})
	}
}

func TestResolveAllPorts(t *testing.T) {
	services := map[string]ServicePortInfo{
		// 非sharedサービス: オフセット分加算される
		"api":    {Port: 3000, Shared: false},
		// sharedサービス: オフセットは無視される
		"db":     {Port: 5432, Shared: true},
		// 非sharedサービス: オフセット分加算される
		"worker": {Port: 4000, Shared: false},
	}
	offset := 100

	result := ResolveAllPorts(services, offset)

	// api: 3000 + 100 = 3100
	if got := result["api"]; got != 3100 {
		t.Errorf("api のポート = %d, want 3100", got)
	}
	// db: shared なので 5432 のまま
	if got := result["db"]; got != 5432 {
		t.Errorf("db のポート = %d, want 5432", got)
	}
	// worker: 4000 + 100 = 4100
	if got := result["worker"]; got != 4100 {
		t.Errorf("worker のポート = %d, want 4100", got)
	}
}

func TestResolveAllPorts_Empty(t *testing.T) {
	// 空のサービスマップを渡した場合、空のマップが返る
	result := ResolveAllPorts(map[string]ServicePortInfo{}, 50)
	if len(result) != 0 {
		t.Errorf("空入力で空マップが返るべきだが len = %d", len(result))
	}
}
