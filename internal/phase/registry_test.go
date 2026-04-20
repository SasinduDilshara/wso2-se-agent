package phase

import (
	"testing"

	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
)

// stubPhase is a minimal Phase implementation for testing.
type stubPhase struct {
	name      string
	phaseType PhaseType
}

func (s *stubPhase) Name() string                                        { return s.name }
func (s *stubPhase) Type() PhaseType                                     { return s.phaseType }
func (s *stubPhase) Preconditions(ctx *PhaseContext) error                { return nil }
func (s *stubPhase) Execute(ctx *PhaseContext) (*state.PhaseResult, error) {
	return &state.PhaseResult{Phase: s.name, Status: state.StatusSuccess}, nil
}
func (s *stubPhase) ExpectedArtifacts() []string { return nil }

func newStub(name string, t PhaseType) *stubPhase {
	return &stubPhase{name: name, phaseType: t}
}

func TestPipeline_AllPhases(t *testing.T) {
	r := NewRegistry()
	for _, name := range DefaultPipeline {
		r.Register(newStub(name, PhaseTypeStatic))
	}

	phases, err := r.Pipeline("", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) != len(DefaultPipeline) {
		t.Errorf("got %d phases, want %d", len(phases), len(DefaultPipeline))
	}
}

func TestPipeline_SinglePhase(t *testing.T) {
	r := NewRegistry()
	for _, name := range DefaultPipeline {
		r.Register(newStub(name, PhaseTypeStatic))
	}

	phases, err := r.Pipeline("", "", "reproduce", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) != 1 {
		t.Fatalf("got %d phases, want 1", len(phases))
	}
	if phases[0].Name() != "reproduce" {
		t.Errorf("got %q, want %q", phases[0].Name(), "reproduce")
	}
}

func TestPipeline_FromTo(t *testing.T) {
	r := NewRegistry()
	for _, name := range DefaultPipeline {
		r.Register(newStub(name, PhaseTypeStatic))
	}

	phases, err := r.Pipeline("reproduce", "verify", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"reproduce", "risk-assessment", "plan-and-fix", "verify"}
	if len(phases) != len(expected) {
		t.Fatalf("got %d phases, want %d", len(phases), len(expected))
	}
	for i, p := range phases {
		if p.Name() != expected[i] {
			t.Errorf("phase[%d]: got %q, want %q", i, p.Name(), expected[i])
		}
	}
}

func TestPipeline_Setup(t *testing.T) {
	r := NewRegistry()
	for _, name := range DefaultPipeline {
		r.Register(newStub(name, PhaseTypeStatic))
	}

	phases, err := r.Pipeline("prereq", "skills", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"prereq", "workspace", "skills"}
	if len(phases) != len(expected) {
		t.Fatalf("got %d phases, want %d", len(phases), len(expected))
	}
}

func TestPipeline_AutoFix(t *testing.T) {
	r := NewRegistry()
	for _, name := range DefaultPipeline {
		r.Register(newStub(name, PhaseTypeStatic))
	}

	phases, err := r.Pipeline("reproduce", "pr", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"reproduce", "risk-assessment", "plan-and-fix", "verify", "test-coverage", "pr"}
	if len(phases) != len(expected) {
		t.Fatalf("got %d phases, want %d", len(phases), len(expected))
	}
}

func TestPipeline_SkipPhases(t *testing.T) {
	r := NewRegistry()
	for _, name := range DefaultPipeline {
		r.Register(newStub(name, PhaseTypeStatic))
	}

	phases, err := r.Pipeline("reproduce", "pr", "", []string{"test-coverage", "risk-assessment"})
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{"reproduce", "plan-and-fix", "verify", "pr"}
	if len(phases) != len(expected) {
		t.Fatalf("got %d phases, want %d", len(phases), len(expected))
	}
	for i, p := range phases {
		if p.Name() != expected[i] {
			t.Errorf("phase[%d]: got %q, want %q", i, p.Name(), expected[i])
		}
	}
}

func TestPipeline_UnknownPhase(t *testing.T) {
	r := NewRegistry()

	_, err := r.Pipeline("", "", "nonexistent", nil)
	if err == nil {
		t.Error("expected error for unknown phase")
	}
}

func TestPipeline_FromAfterTo(t *testing.T) {
	r := NewRegistry()
	for _, name := range DefaultPipeline {
		r.Register(newStub(name, PhaseTypeStatic))
	}

	_, err := r.Pipeline("verify", "reproduce", "", nil)
	if err == nil {
		t.Error("expected error when from is after to")
	}
}
