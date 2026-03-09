package config

import (
	"fmt"
	"regexp"
	"strings"
)

// 変数展開用の正規表現
var varPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// ExpandVariables はConfig内の変数参照を展開する（オフセット0）
// 対象: CommandArgs, EnvDynamic, Healthcheck.URL
func ExpandVariables(cfg *Config) {
	ExpandVariablesWithOffset(cfg, 0)
}

// ExpandVariablesWithOffset はポートオフセットを考慮して変数参照を展開する
// worktreeServicesが指定された場合、含まれないサービスにはoffset=0を適用する
func ExpandVariablesWithOffset(cfg *Config, offset int, worktreeServices ...[]string) {
	var wtSet map[string]bool
	if len(worktreeServices) > 0 && worktreeServices[0] != nil {
		wtSet = make(map[string]bool, len(worktreeServices[0]))
		for _, s := range worktreeServices[0] {
			wtSet[s] = true
		}
	}

	for name, svc := range cfg.Services {
		resolver := buildResolverWithOffset(cfg, name, offset, wtSet)

		// CommandArgs を展開
		for i, arg := range svc.CommandArgs {
			svc.CommandArgs[i] = expandString(arg, resolver)
		}

		// Env を展開
		for key, val := range svc.Env {
			svc.Env[key] = expandString(val, resolver)
		}

		// EnvDynamic を展開
		for key, val := range svc.EnvDynamic {
			svc.EnvDynamic[key] = expandString(val, resolver)
		}

		// Healthcheck.URL を展開
		if svc.Healthcheck != nil && svc.Healthcheck.URL != "" {
			svc.Healthcheck.URL = expandString(svc.Healthcheck.URL, resolver)
		}

		// Healthcheck.Command を展開
		if svc.Healthcheck != nil && svc.Healthcheck.Command != "" {
			svc.Healthcheck.Command = expandString(svc.Healthcheck.Command, resolver)
		}
	}
}

// buildResolverWithOffset はオフセットを考慮した変数リゾルバを生成する
// wtSetが非nilの場合、含まれるサービスのみにoffsetを適用する
func buildResolverWithOffset(cfg *Config, serviceName string, offset int, wtSet map[string]bool) func(string) string {
	// サービスにオフセットを適用すべきか判定する
	shouldApplyOffset := func(svcName string, svc *Service) bool {
		if svc.Shared {
			return false
		}
		// wtSetが未指定ならば全サービスにoffset適用（従来動作）
		if wtSet == nil {
			return true
		}
		return wtSet[svcName]
	}

	return func(varName string) string {
		// ${port} → 自サービスのPort（オフセット適用）
		if varName == "port" {
			svc := cfg.Services[serviceName]
			p := svc.Port
			if shouldApplyOffset(serviceName, svc) {
				p += offset
			}
			return fmt.Sprintf("%d", p)
		}

		// ${services.<name>.port} → 他サービスのPort（オフセット適用）
		if strings.HasPrefix(varName, "services.") {
			parts := strings.Split(varName, ".")
			if len(parts) == 3 && parts[2] == "port" {
				targetName := parts[1]
				if target, ok := cfg.Services[targetName]; ok {
					p := target.Port
					if shouldApplyOffset(targetName, target) {
						p += offset
					}
					return fmt.Sprintf("%d", p)
				}
			}
		}

		// 解決できない変数はそのまま返す
		return "${" + varName + "}"
	}
}

// expandString は文字列内の変数参照を展開する
func expandString(s string, resolver func(string) string) string {
	return varPattern.ReplaceAllStringFunc(s, func(match string) string {
		// ${...} から変数名を取り出す
		varName := match[2 : len(match)-1]
		return resolver(varName)
	})
}
