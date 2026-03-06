package process

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"
)

// HealthcheckConfig はヘルスチェックの設定
type HealthcheckConfig struct {
	Command     string
	URL         string
	Interval    time.Duration
	Timeout     time.Duration
	Retries     int
	StartPeriod time.Duration
	WorkingDir  string
}

// RunHealthcheck はヘルスチェックを実行する
func RunHealthcheck(ctx context.Context, serviceName string, cfg HealthcheckConfig) error {
	// チェック方法が指定されていなければ即成功
	if cfg.Command == "" && cfg.URL == "" {
		return nil
	}

	// デフォルト値
	if cfg.Retries <= 0 {
		cfg.Retries = 3
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 5 * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}

	// start_period待機
	if cfg.StartPeriod > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cfg.StartPeriod):
		}
	}

	var lastErr error
	for i := 0; i < cfg.Retries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.Interval):
			}
		}

		if cfg.URL != "" {
			lastErr = checkURL(ctx, cfg.URL, cfg.Timeout)
		} else {
			lastErr = checkCommand(ctx, cfg.Command, cfg.Timeout, cfg.WorkingDir)
		}

		if lastErr == nil {
			return nil
		}
		log.Debug().Str("service", serviceName).Int("attempt", i+1).Err(lastErr).Msg("healthcheck failed")
	}

	return fmt.Errorf("healthcheck failed after %d retries: %w", cfg.Retries, lastErr)
}

// checkURL はHTTP GETでヘルスチェックを行う
func checkURL(ctx context.Context, url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// checkCommand はコマンド実行でヘルスチェックを行う
func checkCommand(ctx context.Context, command string, timeout time.Duration, workingDir string) error {
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	if workingDir != "" {
		cmd.Dir = workingDir
	}
	return cmd.Run()
}
