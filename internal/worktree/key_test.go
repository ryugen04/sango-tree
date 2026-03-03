package worktree

import "testing"

func TestToKey(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"main", "main"},
		{"feature/auth", "feature--auth"},
		{"feature/auth/login", "feature--auth--login"},
		{"hotfix/v1.0", "hotfix--v1.0"},
	}
	for _, tt := range tests {
		got := ToKey(tt.branch)
		if got != tt.want {
			t.Errorf("ToKey(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestFromKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"main", "main"},
		{"feature--auth", "feature/auth"},
		{"feature--auth--login", "feature/auth/login"},
	}
	for _, tt := range tests {
		got := FromKey(tt.key)
		if got != tt.want {
			t.Errorf("FromKey(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	branches := []string{"main", "feature/auth", "release/v2.0/rc1"}
	for _, b := range branches {
		got := FromKey(ToKey(b))
		if got != b {
			t.Errorf("roundtrip %q: got %q", b, got)
		}
	}
}
