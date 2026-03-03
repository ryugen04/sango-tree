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
func ExpandVariablesWithOffset(cfg *Config, offset int) {
	for name, svc := range cfg.Services {
		resolver := buildResolverWithOffset(cfg, name, offset)

		// CommandArgs を展開
		for i, arg := range svc.CommandArgs {
			svc.CommandArgs[i] = expandString(arg, resolver)
		}

		// EnvDynamic を展開
		for key, val := range svc.EnvDynamic {
			svc.EnvDynamic[key] = expandString(val, resolver)
		}

		// Healthcheck.URL を展開
		if svc.Healthcheck != nil && svc.Healthcheck.URL != "" {
			svc.Healthcheck.URL = expandString(svc.Healthcheck.URL, resolver)
		}
	}
}

// buildResolver は変数名を値に解決する関数を生成する（オフセット0）
func buildResolver(cfg *Config, serviceName string) func(string) string {
	return buildResolverWithOffset(cfg, serviceName, 0)
}

// buildResolverWithOffset はオフセットを考慮した変数リゾルバを生成する
func buildResolverWithOffset(cfg *Config, serviceName string, offset int) func(string) string {
	return func(varName string) string {
		// ${port} → 自サービスのPort（オフセット適用）
		if varName == "port" {
			svc := cfg.Services[serviceName]
			p := svc.Port
			if !svc.Shared {
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
					if !target.Shared {
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
