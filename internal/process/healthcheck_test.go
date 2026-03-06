package process

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunHealthcheckURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := RunHealthcheck(context.Background(), "test-svc", HealthcheckConfig{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Timeout:  1 * time.Second,
		Retries:  3,
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestRunHealthcheckURLFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := RunHealthcheck(context.Background(), "test-svc", HealthcheckConfig{
		URL:      server.URL,
		Interval: 50 * time.Millisecond,
		Timeout:  1 * time.Second,
		Retries:  2,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunHealthcheckCommand(t *testing.T) {
	// 成功ケース
	err := RunHealthcheck(context.Background(), "test-svc", HealthcheckConfig{
		Command:  "true",
		Interval: 100 * time.Millisecond,
		Timeout:  1 * time.Second,
		Retries:  3,
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	// 失敗ケース
	err = RunHealthcheck(context.Background(), "test-svc", HealthcheckConfig{
		Command:  "false",
		Interval: 50 * time.Millisecond,
		Timeout:  1 * time.Second,
		Retries:  2,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunHealthcheckNoop(t *testing.T) {
	err := RunHealthcheck(context.Background(), "test-svc", HealthcheckConfig{
		Interval: 100 * time.Millisecond,
		Timeout:  1 * time.Second,
		Retries:  3,
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestRunHealthcheckRetries(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := RunHealthcheck(context.Background(), "test-svc", HealthcheckConfig{
		URL:      server.URL,
		Interval: 50 * time.Millisecond,
		Timeout:  1 * time.Second,
		Retries:  5,
	})
	if err != nil {
		t.Fatalf("expected nil after retries, got %v", err)
	}
	if got := count.Load(); got < 3 {
		t.Errorf("expected at least 3 attempts, got %d", got)
	}
}
