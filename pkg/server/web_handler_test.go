package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// templateDir resolves the web/templates directory relative to the test binary's
// working directory (pkg/server/) or from the project root, and skips if missing.
func resolveTemplateDir(t *testing.T) string {
	t.Helper()
	// When tests run from pkg/server/ the project root is two levels up.
	candidates := []string{
		filepath.Join("..", "..", "web", "templates"),
		filepath.Join("web", "templates"),
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	t.Skip("web/templates directory not found, skipping")
	return ""
}

// resolveWebHandlerFile returns the path to web_handler.go, skipping if absent.
func resolveWebHandlerFile(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"web_handler.go",
		filepath.Join("pkg", "server", "web_handler.go"),
	}
	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}
	t.Skip("web_handler.go not found, skipping")
	return ""
}

// TestTemplatesNoUndiacritizedVietnamese verifies Phase 2 regression:
// undiacritized Vietnamese strings that were present before the fix must not
// appear in any HTML template.
func TestTemplatesNoUndiacritizedVietnamese(t *testing.T) {
	templateDir := resolveTemplateDir(t)

	badStrings := []string{
		"Co phieu",
		"Du doan",
		"Gia Vang",
		"Lam moi",
		"Thu thap du lieu",
		"Chua co du lieu",
		"Dang tai",
		"Bieu do",
		"Bang gia",
		"Chenh lech",
		"Cap nhat luc",
		"Gia Mua",
		"Gia Ban",
	}

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatalf("failed to read templates dir %q: %v", templateDir, err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".html") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(templateDir, entry.Name()))
		if err != nil {
			t.Errorf("failed to read %s: %v", entry.Name(), err)
			continue
		}
		for _, bad := range badStrings {
			if strings.Contains(string(content), bad) {
				t.Errorf("template %s still contains undiacritized Vietnamese text: %q", entry.Name(), bad)
			}
		}
	}
}

// TestTemplatesContainVietnameseDiacritics verifies that key pages contain
// properly diacritized Vietnamese text after the Phase 2 fix.
func TestTemplatesContainVietnameseDiacritics(t *testing.T) {
	templateDir := resolveTemplateDir(t)

	// Map of filename → required Vietnamese strings with diacritics.
	// Only check files that are confirmed to exist.
	goodStrings := map[string][]string{
		"dashboard.html": {"Tổng quan", "Cổ phiếu", "Dự đoán"},
		"gold.html":      {"Giá Vàng"},
		"training.html":  {"Huấn luyện"},
	}

	for file, expected := range goodStrings {
		fullPath := filepath.Join(templateDir, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			// Skip individual files that may not exist yet rather than failing.
			t.Logf("skipping %s: %v", file, err)
			continue
		}
		for _, good := range expected {
			if !strings.Contains(string(content), good) {
				t.Errorf("template %s should contain Vietnamese text %q but doesn't", file, good)
			}
		}
	}
}

// TestWebHandlerPageTitlesVietnamese verifies that the old English page titles
// are not present in web_handler.go.
// Note: Vietnamese page-title constants were removed when HTML serving was moved
// to a dedicated frontend container; this test now only checks for the absence
// of the old English titles.
func TestWebHandlerPageTitlesVietnamese(t *testing.T) {
	handlerFile := resolveWebHandlerFile(t)

	content, err := os.ReadFile(handlerFile)
	if err != nil {
		t.Fatalf("failed to read %s: %v", handlerFile, err)
	}

	src := string(content)

	// Old English titles that should have been removed.
	oldTitles := []string{
		"VN Stock Prediction Dashboard",
		"Stocks - VN Stock Dashboard",
		"Predictions - VN Stock Dashboard",
		"ML Training - VN Stock Dashboard",
	}
	for _, old := range oldTitles {
		if strings.Contains(src, old) {
			t.Errorf("web_handler.go still contains old English title: %q", old)
		}
	}
}
