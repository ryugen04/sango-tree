package process

import (
	"context"
	"net"
	"os"
	"testing"
)

func TestRunPostStartVerification_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen error: %v", err)
	}
	defer ln.Close()

	portNumber := ln.Addr().(*net.TCPAddr).Port
	result := RunPostStartVerification(context.Background(), "test-svc", portNumber, os.Getpid())

	if result.PortListening == nil || !*result.PortListening {
		t.Fatalf("PortListening = %v, want true", result.PortListening)
	}
	if result.ProcessAlive == nil || !*result.ProcessAlive {
		t.Fatalf("ProcessAlive = %v, want true", result.ProcessAlive)
	}
	if result.HasErrors() {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
}

func TestRunPostStartVerification_PortNotListening(t *testing.T) {
	portNumber := pickUnusedPort(t)
	result := RunPostStartVerification(context.Background(), "test-svc", portNumber, 0)

	if result.PortListening == nil || *result.PortListening {
		t.Fatalf("PortListening = %v, want false", result.PortListening)
	}
	if result.ProcessAlive != nil {
		t.Fatalf("ProcessAlive = %v, want nil", result.ProcessAlive)
	}
	if !result.HasErrors() {
		t.Fatal("expected errors, got none")
	}
}

func TestRunPostStartVerification_ProcessNotAlive(t *testing.T) {
	pid := pickDeadPID()
	result := RunPostStartVerification(context.Background(), "test-svc", 0, pid)

	if result.PortListening != nil {
		t.Fatalf("PortListening = %v, want nil", result.PortListening)
	}
	if result.ProcessAlive == nil || *result.ProcessAlive {
		t.Fatalf("ProcessAlive = %v, want false", result.ProcessAlive)
	}
	if !result.HasErrors() {
		t.Fatal("expected errors, got none")
	}
}

func TestRunPostStartVerification_SkipsWhenPortAndPIDAreZero(t *testing.T) {
	result := RunPostStartVerification(context.Background(), "test-svc", 0, 0)

	if result.PortListening != nil {
		t.Fatalf("PortListening = %v, want nil", result.PortListening)
	}
	if result.ProcessAlive != nil {
		t.Fatalf("ProcessAlive = %v, want nil", result.ProcessAlive)
	}
	if result.HasErrors() {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
}

func pickUnusedPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve port: %v", err)
	}
	portNumber := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return portNumber
}

func pickDeadPID() int {
	start := os.Getpid() + 10000
	for i := 0; i < 1000; i++ {
		candidate := start + i
		if candidate > 0 && !IsProcessRunning(candidate) {
			return candidate
		}
	}
	return 999999
}
