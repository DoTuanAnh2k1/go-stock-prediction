package testutil

import (
	"testing"
)

func TestLoadAllFixtures(t *testing.T) {
	_, err := LoadAllFixtures()
	if err != nil {
		t.Skipf("testdata not available: %v", err)
	}
}

func TestLoadFixtures_FileNotFound(t *testing.T) {
	var data []interface{}
	err := LoadFixtures("nonexistent.json", &data)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
