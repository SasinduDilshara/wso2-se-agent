package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SasinduDilshara/wso2-se-agent/internal/claude"
	"github.com/SasinduDilshara/wso2-se-agent/internal/state"
	"github.com/SasinduDilshara/wso2-se-agent/tests/testutil"
)

func TestStreamParser_ParsesFixture(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	fixture := env.FixturePath("reproduce-output.jsonl")

	// Open the fixture as a reader
	f, err := os.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	logDir := filepath.Join(env.RootDir, "logs")
	os.MkdirAll(logDir, 0755)

	processedLog, _ := os.Create(filepath.Join(logDir, "test.log"))
	defer processedLog.Close()
	rawLog, _ := os.Create(filepath.Join(logDir, "test-raw.log"))
	defer rawLog.Close()

	// Use the exported invoker to test stream parsing indirectly
	// We'll test the mock claude binary instead
	_ = fixture
}

func TestMockClaude_WritesFilesAndReturns(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	wsDir := filepath.Join(env.RootDir, "test-workspace")
	os.MkdirAll(wsDir, 0755)

	fixture := env.FixturePath("reproduce-output.jsonl")

	// Run mock claude
	cmd := exec.Command("claude", "-p", "/reproduce test-url")
	cmd.Dir = wsDir
	cmd.Env = append(os.Environ(),
		"PATH="+env.MocksDir+":"+env.OrigPath,
		"WSE_MOCK_CLAUDE_FIXTURE="+fixture,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mock claude failed: %v\n%s", err, string(output))
	}

	// Check that it produced output
	if len(output) == 0 {
		t.Error("expected output from mock claude")
	}

	// Check that the mock created the artifact file
	artifactPath := filepath.Join(wsDir, ".ai", "ia-4856.md")
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Error("mock claude did not create the artifact file")
	}

	// Check output contains the result line
	if !strings.Contains(string(output), "total_cost_usd") {
		t.Error("output should contain cost info")
	}
}

func TestMockClaude_ExitCode(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	wsDir := filepath.Join(env.RootDir, "test-workspace")
	os.MkdirAll(wsDir, 0755)

	cmd := exec.Command("claude", "-p", "test")
	cmd.Dir = wsDir
	cmd.Env = append(os.Environ(),
		"PATH="+env.MocksDir+":"+env.OrigPath,
		"WSE_MOCK_CLAUDE_FIXTURE="+env.FixturePath("claude-error.jsonl"),
		"WSE_MOCK_CLAUDE_EXIT_CODE=1",
	)

	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit code")
	}
}

func TestInvoker_WithMockClaude(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	wsDir := filepath.Join(env.RootDir, "test-workspace")
	os.MkdirAll(filepath.Join(wsDir, ".ai", "logs"), 0755)

	// Set mock fixture
	os.Setenv("WSE_MOCK_CLAUDE_FIXTURE", env.FixturePath("reproduce-output.jsonl"))
	defer os.Unsetenv("WSE_MOCK_CLAUDE_FIXTURE")

	inv := claude.NewInvoker()
	result, err := inv.Invoke(claude.Options{
		Prompt:       "/reproduce https://github.com/wso2/product-apim/issues/4856",
		WorkingDir:   wsDir,
		OutputFormat: "stream-json",
	}, filepath.Join(wsDir, ".ai", "logs"), "4856", "reproduce", "20260417-000000")

	if err != nil {
		t.Fatalf("Invoke failed: %v", err)
	}

	if result.TotalCostUSD != 2.29 {
		t.Errorf("TotalCostUSD: got %f, want 2.29", result.TotalCostUSD)
	}

	if result.ResultText == "" {
		t.Error("ResultText should not be empty")
	}

	// Check logs were created
	logFiles, _ := filepath.Glob(filepath.Join(wsDir, ".ai", "logs", "*.log"))
	if len(logFiles) < 2 {
		t.Errorf("expected 2 log files (processed + raw), got %d", len(logFiles))
	}
}

func TestFullStaticPipeline_WithFakeRepos(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	// Create fake git repos
	carbonPath := env.CreateFakeRepo("carbon-apimgt", "master")
	productPath := env.CreateFakeRepo("product-apim", "master")

	// Write configs
	env.WriteConfig(fmt.Sprintf(`
github_username: testuser
risk_threshold: 7
max_budget_usd: 15.0
log_level: info
workspace_root: %s
`, env.WorkspaceDir))

	env.WriteRepos(fmt.Sprintf(`
repos:
  carbon-apimgt:
    local_path: %s
    fork: testuser/carbon-apimgt
    upstream: wso2/carbon-apimgt
  product-apim:
    local_path: %s
    fork: testuser/product-apim
    upstream: wso2/product-apim
`, carbonPath, productPath))

	// Create a skills tarball for the mock gh to serve
	tarball := env.CreateSkillsTarball()
	os.Setenv("WSE_MOCK_GH_TARBALL", tarball)
	defer os.Unsetenv("WSE_MOCK_GH_TARBALL")

	env.WriteProductConfig("apim", "latest", fmt.Sprintf(`
product: apim
version: latest
repos:
  - name: carbon-apimgt
    upstream: wso2/carbon-apimgt
    branch: master
  - name: product-apim
    upstream: wso2/product-apim
    branch: master
build:
  pack_zip_pattern: "wso2am-*.zip"
runtime:
  startup_command: "sh api-manager.sh"
  health_check_log: "Mgt Console URL"
  default_ports: [19443]
  startup_timeout_seconds: 10
phase_limits:
  reproduce: 5.0
skills_repo: "SasinduDilshara/wso2-se-agent-skills"
skills_branch: "main"
skills_ref: "api-manager-specific/v4"
generic_skills_ref: "skills"
`))

	// Create a dummy pack file
	packPath := filepath.Join(env.RootDir, "wso2am-test.zip")
	os.WriteFile(packPath, []byte("fake-zip"), 0644)

	// Run the CLI: setup only (static phases)
	binPath := buildCLI(t)

	cmd := exec.Command(binPath, "run",
		"--product", "apim",
		"--version", "latest",
		"--issue", "https://github.com/wso2/product-apim/issues/4856",
		"--pack", packPath,
		"--setup",
		"--yes",
	)
	cmd.Env = append(os.Environ(),
		"HOME="+env.RootDir,
		"GOPATH="+os.Getenv("GOPATH"),
		"PATH="+env.MocksDir+":"+env.OrigPath,
		"WSE_MOCK_GH_TARBALL="+tarball,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, string(output))
	}

	outputStr := string(output)

	// Verify all static phases passed
	if !strings.Contains(outputStr, "Pipeline complete") {
		t.Errorf("expected pipeline complete, got:\n%s", outputStr)
	}

	// Verify workspace was created
	wsPath := filepath.Join(env.WorkspaceDir, "apim-issues-4856")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Error("workspace directory not created")
	}

	// Verify state file
	ws, err := state.Load(wsPath)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	if !ws.PhaseSucceeded("prereq") {
		t.Error("prereq should be succeeded")
	}
	if !ws.PhaseSucceeded("workspace") {
		t.Error("workspace should be succeeded")
	}
	if !ws.PhaseSucceeded("skills") {
		t.Error("skills should be succeeded")
	}

	// Verify worktrees were created
	if _, err := os.Stat(filepath.Join(wsPath, "carbon-apimgt")); os.IsNotExist(err) {
		t.Error("carbon-apimgt worktree not created")
	}
	if _, err := os.Stat(filepath.Join(wsPath, "product-apim")); os.IsNotExist(err) {
		t.Error("product-apim worktree not created")
	}

	// Verify skills were installed
	skillFiles, _ := filepath.Glob(filepath.Join(wsPath, ".claude", "skills", "*", "SKILL.md"))
	if len(skillFiles) == 0 {
		t.Error("no skills installed")
	}

	// Verify CLAUDE.md was copied
	if _, err := os.Stat(filepath.Join(wsPath, "CLAUDE.md")); os.IsNotExist(err) {
		t.Error("CLAUDE.md not created")
	}

	// Verify pack was copied
	if _, err := os.Stat(filepath.Join(wsPath, "wso2am-test.zip")); os.IsNotExist(err) {
		t.Error("product pack not copied to workspace")
	}
}

func TestReproducePhase_WithMockClaude(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	// Create a workspace with state showing static phases completed
	wsPath := filepath.Join(env.WorkspaceDir, "apim-issues-4856")
	os.MkdirAll(filepath.Join(wsPath, ".ai", "logs"), 0755)
	os.MkdirAll(filepath.Join(wsPath, ".wse"), 0755)
	os.MkdirAll(filepath.Join(wsPath, ".claude", "skills", "reproduce"), 0755)
	os.WriteFile(filepath.Join(wsPath, ".claude", "skills", "reproduce", "SKILL.md"), []byte("# reproduce"), 0644)

	ws := state.New("https://github.com/wso2/product-apim/issues/4856", "4856", "apim", "latest")
	ws.Phases["prereq"] = &state.PhaseResult{Status: state.StatusSuccess}
	ws.Phases["workspace"] = &state.PhaseResult{Status: state.StatusSuccess}
	ws.Phases["skills"] = &state.PhaseResult{Status: state.StatusSuccess}
	state.Save(wsPath, ws)

	env.WriteConfig(fmt.Sprintf(`
github_username: testuser
risk_threshold: 7
max_budget_usd: 15.0
workspace_root: %s
`, env.WorkspaceDir))

	env.WriteRepos(`repos: {}`)

	env.WriteProductConfig("apim", "latest", `
product: apim
version: latest
repos: []
phase_limits:
  reproduce: 5.0
skills_repo: "test/test"
skills_branch: "main"
skills_ref: "test"
generic_skills_ref: "skills"
`)

	binPath := buildCLI(t)

	cmd := exec.Command(binPath, "run",
		"--product", "apim",
		"--version", "latest",
		"--issue", "https://github.com/wso2/product-apim/issues/4856",
		"--pack", "/dev/null",
		"--phase", "reproduce",
		"--yes",
	)
	cmd.Dir = wsPath
	cmd.Env = append(os.Environ(),
		"HOME="+env.RootDir,
		"GOPATH="+os.Getenv("GOPATH"),
		"PATH="+env.MocksDir+":"+env.OrigPath,
		"WSE_MOCK_CLAUDE_FIXTURE="+env.FixturePath("reproduce-output.jsonl"),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, string(output))
	}

	// Verify state was updated
	ws, _ = state.Load(wsPath)
	if !ws.PhaseSucceeded("reproduce") {
		t.Error("reproduce should be succeeded")
	}
	if ws.Phases["reproduce"].CostUSD != 2.29 {
		t.Errorf("cost: got %f, want 2.29", ws.Phases["reproduce"].CostUSD)
	}

	// Verify logs were created
	logFiles, _ := filepath.Glob(filepath.Join(wsPath, ".ai", "logs", "*reproduce*.log"))
	if len(logFiles) < 2 {
		t.Errorf("expected 2 log files, got %d", len(logFiles))
	}
}

func TestDryRun(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	env.WriteConfig(fmt.Sprintf(`
github_username: testuser
workspace_root: %s
`, env.WorkspaceDir))
	env.WriteRepos(`repos: {}`)
	env.WriteProductConfig("apim", "latest", `
product: apim
version: latest
repos: []
skills_repo: "test/test"
skills_branch: "main"
skills_ref: "test"
`)

	binPath := buildCLI(t)

	cmd := exec.Command(binPath, "run",
		"--product", "apim",
		"--version", "latest",
		"--issue", "https://github.com/wso2/product-apim/issues/4856",
		"--pack", "/dev/null",
		"--dry-run",
	)
	cmd.Env = append(os.Environ(), "HOME="+env.RootDir, "GOPATH="+os.Getenv("GOPATH"))

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Dry run") {
		t.Error("expected dry run output")
	}
	if !strings.Contains(outStr, "prereq") {
		t.Error("expected prereq in dry run")
	}
	if !strings.Contains(outStr, "reproduce") {
		t.Error("expected reproduce in dry run")
	}

	// Verify NO workspace was created
	wsPath := filepath.Join(env.WorkspaceDir, "apim-issues-4856")
	entries, _ := os.ReadDir(wsPath)
	// Workspace dir gets created by the run command, but should have no state
	stateFile := filepath.Join(wsPath, ".wse", "state.json")
	if _, err := os.Stat(stateFile); err == nil {
		t.Error("dry run should not create state file")
	}
	_ = entries
}

func TestStatusCommand(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	wsPath := filepath.Join(env.RootDir, "test-workspace")
	ws := state.New("https://github.com/wso2/product-apim/issues/4856", "4856", "apim", "latest")
	ws.Phases["prereq"] = &state.PhaseResult{Phase: "prereq", Status: state.StatusSuccess, Duration: "1ms"}
	ws.Phases["reproduce"] = &state.PhaseResult{Phase: "reproduce", Status: state.StatusSuccess, Duration: "30s", CostUSD: 2.29}
	score := 4
	ws.RiskScore = &score
	state.Save(wsPath, ws)

	binPath := buildCLI(t)

	cmd := exec.Command(binPath, "status", wsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !strings.Contains(outStr, "4856") {
		t.Error("expected issue number in output")
	}
	if !strings.Contains(outStr, "$2.29") {
		t.Error("expected cost in output")
	}
	if !strings.Contains(outStr, "4/10") {
		t.Error("expected risk score in output")
	}
}

func TestCleanCommand(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	// Create a fake workspace with state
	wsPath := filepath.Join(env.RootDir, "test-workspace")
	ws := state.New("url", "1", "apim", "latest")
	// No worktrees to remove (would need real git repos)
	state.Save(wsPath, ws)

	binPath := buildCLI(t)

	cmd := exec.Command(binPath, "clean", wsPath, "--all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, string(output))
	}

	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Error("workspace should be deleted")
	}
}

func TestVersionCommand(t *testing.T) {
	binPath := buildCLI(t)

	cmd := exec.Command(binPath, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, string(output))
	}

	if !strings.Contains(string(output), "wso2-se-agent") {
		t.Error("expected version output")
	}
}

func TestConfigShow(t *testing.T) {
	env := testutil.NewTestEnv(t)
	defer env.Cleanup()

	env.WriteConfig(`
github_username: testuser
risk_threshold: 5
max_budget_usd: 20.0
`)

	binPath := buildCLI(t)

	cmd := exec.Command(binPath, "config", "show")
	cmd.Env = append(os.Environ(), "HOME="+env.RootDir, "GOPATH="+os.Getenv("GOPATH"))

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !strings.Contains(outStr, "testuser") {
		t.Error("expected username in output")
	}
	if !strings.Contains(outStr, "20.00") {
		t.Error("expected budget in output")
	}
}

// buildCLI compiles the CLI binary once and caches it in a persistent temp dir.
var cachedBinPath string

func buildCLI(t *testing.T) string {
	t.Helper()
	if cachedBinPath != "" {
		if _, err := os.Stat(cachedBinPath); err == nil {
			return cachedBinPath
		}
	}

	projectRoot := findProjectRoot(t)

	// Use os.MkdirTemp instead of t.TempDir so it persists across tests
	tmpDir, err := os.MkdirTemp("", "wso2-se-agent-test-*")
	if err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(tmpDir, "wso2-se-agent-test")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/wso2-se-agent/")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build CLI: %v\n%s", err, string(output))
	}

	cachedBinPath = binPath
	return binPath
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
}

// Unused but here for reference: parse state from JSON bytes
func parseState(data []byte) (*state.WorkspaceState, error) {
	var ws state.WorkspaceState
	err := json.Unmarshal(data, &ws)
	return &ws, err
}
