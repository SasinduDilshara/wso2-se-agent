package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type PhaseStatus string

const (
	StatusSuccess   PhaseStatus = "success"
	StatusFailed    PhaseStatus = "failed"
	StatusSkipped   PhaseStatus = "skipped"
	StatusGated     PhaseStatus = "gated"
	StatusRunning   PhaseStatus = "running"
	StatusTurnLimit PhaseStatus = "failed:turn_limit"
)

type PhaseResult struct {
	Phase     string            `json:"phase"`
	Status    PhaseStatus       `json:"status"`
	StartedAt time.Time         `json:"started_at"`
	EndedAt   time.Time         `json:"ended_at"`
	Duration  string            `json:"duration"`
	Artifacts []string          `json:"artifacts,omitempty"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
	CostUSD   float64           `json:"cost_usd"`
	Error     string            `json:"error,omitempty"`
}

type WorktreeEntry struct {
	RepoName  string `json:"repo_name"`
	LocalPath string `json:"local_path"`
	Branch    string `json:"branch"`
	BasePath  string `json:"base_path"`
}

type WorkspaceState struct {
	IssueURL    string                  `json:"issue_url"`
	IssueNumber string                  `json:"issue_number"`
	Product     string                  `json:"product"`
	Version     string                  `json:"version"`
	CreatedAt   time.Time               `json:"created_at"`
	Worktrees   []WorktreeEntry         `json:"worktrees"`
	Phases      map[string]*PhaseResult `json:"phases"`
	RiskScore   *int                    `json:"risk_score,omitempty"`
	PRURL       string                  `json:"pr_url,omitempty"`
}

func New(issueURL, issueNumber, product, version string) *WorkspaceState {
	return &WorkspaceState{
		IssueURL:    issueURL,
		IssueNumber: issueNumber,
		Product:     product,
		Version:     version,
		CreatedAt:   time.Now(),
		Phases:      make(map[string]*PhaseResult),
	}
}

func Load(workspaceDir string) (*WorkspaceState, error) {
	path := filepath.Join(workspaceDir, ".wse", "state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &WorkspaceState{Phases: make(map[string]*PhaseResult)}, nil
		}
		return nil, err
	}

	var s WorkspaceState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.Phases == nil {
		s.Phases = make(map[string]*PhaseResult)
	}
	return &s, nil
}

func Save(workspaceDir string, s *WorkspaceState) error {
	dir := filepath.Join(workspaceDir, ".wse")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file then rename
	tmpPath := filepath.Join(dir, "state.json.tmp")
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, filepath.Join(dir, "state.json"))
}

func (s *WorkspaceState) PhaseSucceeded(name string) bool {
	if r, ok := s.Phases[name]; ok {
		return r.Status == StatusSuccess
	}
	return false
}
