package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	ws := New("https://github.com/wso2/product-apim/issues/4856", "4856", "apim", "latest")
	ws.Phases["reproduce"] = &PhaseResult{
		Phase:   "reproduce",
		Status:  StatusSuccess,
		CostUSD: 2.29,
	}
	score := 4
	ws.RiskScore = &score

	if err := Save(dir, ws); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, ".wse", "state.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file not found: %v", err)
	}

	// Load it back
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.IssueNumber != "4856" {
		t.Errorf("IssueNumber: got %q, want %q", loaded.IssueNumber, "4856")
	}
	if loaded.Product != "apim" {
		t.Errorf("Product: got %q, want %q", loaded.Product, "apim")
	}
	if !loaded.PhaseSucceeded("reproduce") {
		t.Error("expected reproduce to be succeeded")
	}
	if loaded.PhaseSucceeded("verify") {
		t.Error("expected verify to not be succeeded")
	}
	if loaded.RiskScore == nil || *loaded.RiskScore != 4 {
		t.Errorf("RiskScore: got %v, want 4", loaded.RiskScore)
	}
	if loaded.Phases["reproduce"].CostUSD != 2.29 {
		t.Errorf("CostUSD: got %f, want 2.29", loaded.Phases["reproduce"].CostUSD)
	}
}

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()

	ws, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ws.Phases == nil {
		t.Error("Phases should be initialized")
	}
	if ws.IssueURL != "" {
		t.Error("should be empty state")
	}
}

func TestPhaseSucceeded(t *testing.T) {
	ws := New("url", "1", "apim", "latest")
	ws.Phases["reproduce"] = &PhaseResult{Status: StatusSuccess}
	ws.Phases["verify"] = &PhaseResult{Status: StatusFailed}

	if !ws.PhaseSucceeded("reproduce") {
		t.Error("reproduce should be succeeded")
	}
	if ws.PhaseSucceeded("verify") {
		t.Error("verify should not be succeeded")
	}
	if ws.PhaseSucceeded("nonexistent") {
		t.Error("nonexistent should not be succeeded")
	}
}
