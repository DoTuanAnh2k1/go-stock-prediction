package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolveJSDir locates web/static/js relative to the test binary or project root.
func resolveJSDir(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "web", "static", "js"),
		filepath.Join("web", "static", "js"),
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	t.Skip("web/static/js directory not found, skipping")
	return ""
}

// TestJSNoMockData verifies that JavaScript files no longer contain hardcoded
// mock/sample data that was replaced by real API calls in Phase 1/2.
func TestJSNoMockData(t *testing.T) {
	jsDir := resolveJSDir(t)

	t.Run("dashboard_js_no_mock_stats", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(jsDir, "dashboard.js"))
		if err != nil {
			t.Fatalf("failed to read dashboard.js: %v", err)
		}
		src := string(content)
		if strings.Contains(src, "updateMockStats") {
			t.Error("dashboard.js still contains updateMockStats function")
		}
		if strings.Contains(src, "Coming soon!") {
			t.Error("dashboard.js still contains 'Coming soon!' alert placeholder")
		}
	})

	t.Run("stock_js_no_hardcoded_sample_data", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(jsDir, "stock.js"))
		if err != nil {
			t.Fatalf("failed to read stock.js: %v", err)
		}
		src := string(content)
		// The old hardcoded Vietcombank entry at price 87500 was sample data.
		if strings.Contains(src, "87500") && strings.Contains(src, "Vietcombank") {
			t.Error("stock.js still contains hardcoded sample stock data (87500/Vietcombank)")
		}
		if strings.Contains(src, "Coming soon!") {
			t.Error("stock.js still contains 'Coming soon!' alert placeholder")
		}
	})
}

// TestJSNoUndiacritizedVietnamese verifies that JavaScript files use proper
// Vietnamese diacritics rather than the ASCII approximations from before Phase 2.
func TestJSNoUndiacritizedVietnamese(t *testing.T) {
	jsDir := resolveJSDir(t)

	badStrings := []string{
		"Dang thu thap",
		"Hoan thanh",
		"That bai",
		"Loi ket noi",
		"Chua co du lieu",
		"Khong the tai",
	}

	t.Run("gold_js", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(jsDir, "gold.js"))
		if err != nil {
			t.Fatalf("failed to read gold.js: %v", err)
		}
		src := string(content)
		for _, bad := range badStrings {
			if strings.Contains(src, bad) {
				t.Errorf("gold.js still contains undiacritized Vietnamese: %q", bad)
			}
		}
	})
}
