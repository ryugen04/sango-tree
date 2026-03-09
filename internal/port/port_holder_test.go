package port

import (
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestGetPortHolder_UnusedPort(t *testing.T) {
	// 未使用ポートの場合、PID 0 を返すべき
	pid, err := GetPortHolder(19999) // 使われていない可能性の高いポート
	if err != nil {
		t.Fatalf("GetPortHolder error: %v", err)
	}
	if pid != 0 {
		t.Logf("ポート 19999 は PID %d に占有されている（スキップ）", pid)
		t.Skip("テスト用ポートが使用中")
	}
}

func TestGetPortHolder_UsedPort(t *testing.T) {
	// テスト用にポートをリッスン
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("net.Listen error: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	parts := strings.Split(addr, ":")
	portStr := parts[len(parts)-1]
	portNum, _ := strconv.Atoi(portStr)

	pid, err := GetPortHolder(portNum)
	if err != nil {
		t.Fatalf("GetPortHolder error: %v", err)
	}
	if pid == 0 {
		t.Error("リッスン中のポートに対してPID 0が返された")
	}
}
