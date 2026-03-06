package runbook

import (
	"strings"

	"github.com/ryugen04/sango-tree/internal/config"
)

// SearchResult は検索結果
type SearchResult struct {
	ServiceName string
	Entry       config.RunbookEntry
	MatchField  string // "title" | "symptoms" | "cause" | "tags"
}

// Search は全サービスのrunbookからキーワードにマッチするエントリを返す
func Search(services map[string]*config.Service, keyword string) []SearchResult {
	var results []SearchResult
	for name, svc := range services {
		for _, entry := range svc.Runbook {
			if matched, field := Match(entry, keyword); matched {
				results = append(results, SearchResult{
					ServiceName: name,
					Entry:       entry,
					MatchField:  field,
				})
			}
		}
	}
	return results
}

// Match はRunbookEntryがキーワードにマッチするか判定する（部分一致、case-insensitive）
func Match(entry config.RunbookEntry, keyword string) (bool, string) {
	kw := strings.ToLower(keyword)

	if strings.Contains(strings.ToLower(entry.Title), kw) {
		return true, "title"
	}
	for _, s := range entry.Symptoms {
		if strings.Contains(strings.ToLower(s), kw) {
			return true, "symptoms"
		}
	}
	if strings.Contains(strings.ToLower(entry.Cause), kw) {
		return true, "cause"
	}
	for _, tag := range entry.Tags {
		if strings.Contains(strings.ToLower(tag), kw) {
			return true, "tags"
		}
	}
	return false, ""
}
