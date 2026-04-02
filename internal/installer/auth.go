package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AuthResult holds the result of an authentication check or flow
type AuthResult struct {
	Provider  string
	Installed bool
	CliTool   string
	WasAuthed bool  // already had valid credentials before auth attempt
	NowAuthed bool  // credentials valid after auth attempt
	Error     error
}

// AuthAll checks authentication status for all providers without running auth flows
func AuthAll() []AuthResult {
	return []AuthResult{
		checkClaude(),
		checkCodex(),
		checkGemini(),
		checkCopilot(),
		checkCodexBar(),
	}
}

// RunAuthFlow dispatches to the correct auth function by provider name
func RunAuthFlow(provider string) AuthResult {
	switch strings.ToLower(provider) {
	case "claude":
		return AuthClaude()
	case "codex":
		return AuthCodex()
	case "gemini":
		return AuthGemini()
	case "copilot":
		return AuthCopilot()
	case "codexbar":
		return AuthCodexBar()
	default:
		return AuthResult{
			Provider: provider,
			Error:    fmt.Errorf("unknown provider %q; valid providers: claude, codex, gemini, copilot, codexbar", provider),
		}
	}
}

// ── Claude ──────────────────────────────────────────────────────────────────

func claudeCredPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", ".credentials.json")
}

func claudeHasToken() bool {
	data, err := os.ReadFile(claudeCredPath())
	if err != nil {
		return false
	}
	var creds map[string]interface{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return false
	}
	oauth, ok := creds["claudeAiOauth"].(map[string]interface{})
	if !ok {
		return false
	}
	token, ok := oauth["accessToken"].(string)
	return ok && token != ""
}

func checkClaude() AuthResult {
	r := AuthResult{Provider: "claude", CliTool: "claude"}
	_, err := exec.LookPath("claude")
	r.Installed = err == nil
	r.WasAuthed = claudeHasToken()
	r.NowAuthed = r.WasAuthed
	return r
}

// AuthClaude authenticates with the Claude CLI
func AuthClaude() AuthResult {
	r := AuthResult{Provider: "claude", CliTool: "claude"}

	path, err := exec.LookPath("claude")
	if err != nil {
		r.Error = fmt.Errorf("claude CLI not found; install it first: https://docs.anthropic.com/en/docs/claude-code")
		return r
	}
	r.Installed = true
	_ = path

	r.WasAuthed = claudeHasToken()
	if r.WasAuthed {
		r.NowAuthed = true
		return r
	}

	cmd := exec.Command("claude", "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		r.Error = fmt.Errorf("claude auth login failed: %w", err)
		r.NowAuthed = claudeHasToken()
		return r
	}

	r.NowAuthed = claudeHasToken()
	if !r.NowAuthed {
		r.Error = fmt.Errorf("auth flow completed but no token found at %s", claudeCredPath())
	}
	return r
}

// ── Codex (OpenAI) ──────────────────────────────────────────────────────────

func codexCredPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".codex", "auth.json")
}

func codexHasToken() bool {
	// Check environment variable first
	if os.Getenv("OPENAI_API_KEY") != "" {
		return true
	}
	data, err := os.ReadFile(codexCredPath())
	if err != nil {
		return false
	}
	var auth map[string]interface{}
	if err := json.Unmarshal(data, &auth); err != nil {
		return false
	}
	// Check tokens.access_token
	if tokens, ok := auth["tokens"].(map[string]interface{}); ok {
		if token, ok := tokens["access_token"].(string); ok && token != "" {
			return true
		}
	}
	// Check top-level access_token
	if token, ok := auth["access_token"].(string); ok && token != "" {
		return true
	}
	return false
}

func checkCodex() AuthResult {
	r := AuthResult{Provider: "codex", CliTool: "codex"}
	_, err := exec.LookPath("codex")
	r.Installed = err == nil
	r.WasAuthed = codexHasToken()
	r.NowAuthed = r.WasAuthed
	return r
}

// AuthCodex authenticates with the Codex CLI
func AuthCodex() AuthResult {
	r := AuthResult{Provider: "codex", CliTool: "codex"}

	_, err := exec.LookPath("codex")
	if err != nil {
		r.Error = fmt.Errorf("codex CLI not found; install it first: npm install -g @openai/codex")
		return r
	}
	r.Installed = true

	r.WasAuthed = codexHasToken()
	if r.WasAuthed {
		r.NowAuthed = true
		return r
	}

	cmd := exec.Command("codex", "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		r.Error = fmt.Errorf("codex auth login failed: %w", err)
		r.NowAuthed = codexHasToken()
		return r
	}

	r.NowAuthed = codexHasToken()
	if !r.NowAuthed {
		r.Error = fmt.Errorf("auth flow completed but no token found; you can also set OPENAI_API_KEY")
	}
	return r
}

// ── Gemini ──────────────────────────────────────────────────────────────────

func geminiCredPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "oauth_creds.json")
}

func geminiHasToken() bool {
	data, err := os.ReadFile(geminiCredPath())
	if err != nil {
		return false
	}
	var creds map[string]interface{}
	if err := json.Unmarshal(data, &creds); err != nil {
		return false
	}
	token, ok := creds["access_token"].(string)
	return ok && token != ""
}

func checkGemini() AuthResult {
	r := AuthResult{Provider: "gemini", CliTool: "gemini"}
	_, err := exec.LookPath("gemini")
	r.Installed = err == nil
	r.WasAuthed = geminiHasToken()
	r.NowAuthed = r.WasAuthed
	return r
}

// AuthGemini authenticates with the Gemini CLI
func AuthGemini() AuthResult {
	r := AuthResult{Provider: "gemini", CliTool: "gemini"}

	_, err := exec.LookPath("gemini")
	if err != nil {
		r.Error = fmt.Errorf("gemini CLI not found; install it first: npm install -g @google/gemini-cli")
		return r
	}
	r.Installed = true

	r.WasAuthed = geminiHasToken()
	if r.WasAuthed {
		r.NowAuthed = true
		return r
	}

	cmd := exec.Command("gemini", "auth", "login")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		r.Error = fmt.Errorf("gemini auth login failed: %w", err)
		r.NowAuthed = geminiHasToken()
		return r
	}

	r.NowAuthed = geminiHasToken()
	if !r.NowAuthed {
		r.Error = fmt.Errorf("auth flow completed but no token found at %s", geminiCredPath())
	}
	return r
}

// ── Copilot (GitHub) ────────────────────────────────────────────────────────

func copilotIsAuthed() bool {
	cmd := exec.Command("gh", "auth", "status")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func checkCopilot() AuthResult {
	r := AuthResult{Provider: "copilot", CliTool: "gh"}
	_, err := exec.LookPath("gh")
	r.Installed = err == nil
	if r.Installed {
		r.WasAuthed = copilotIsAuthed()
		r.NowAuthed = r.WasAuthed
	}
	return r
}

// AuthCopilot authenticates with GitHub via gh CLI
func AuthCopilot() AuthResult {
	r := AuthResult{Provider: "copilot", CliTool: "gh"}

	_, err := exec.LookPath("gh")
	if err != nil {
		r.Error = fmt.Errorf("gh CLI not found; install it first: https://cli.github.com/")
		return r
	}
	r.Installed = true

	r.WasAuthed = copilotIsAuthed()
	if r.WasAuthed {
		r.NowAuthed = true
		return r
	}

	cmd := exec.Command("gh", "auth", "login", "--web")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		r.Error = fmt.Errorf("gh auth login failed: %w", err)
		r.NowAuthed = copilotIsAuthed()
		return r
	}

	r.NowAuthed = copilotIsAuthed()
	if !r.NowAuthed {
		r.Error = fmt.Errorf("auth flow completed but gh auth status still reports unauthenticated")
	}
	return r
}

// ── CodexBar ────────────────────────────────────────────────────────────────

func codexbarIsAuthed() bool {
	cmd := exec.Command("codexbar", "--provider", "claude", "--format", "json")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func checkCodexBar() AuthResult {
	r := AuthResult{Provider: "codexbar", CliTool: "codexbar"}
	_, err := exec.LookPath("codexbar")
	r.Installed = err == nil
	if r.Installed {
		r.WasAuthed = codexbarIsAuthed()
		r.NowAuthed = r.WasAuthed
	}
	return r
}

// AuthCodexBar authenticates with codexbar
func AuthCodexBar() AuthResult {
	r := AuthResult{Provider: "codexbar", CliTool: "codexbar"}

	_, err := exec.LookPath("codexbar")
	if err != nil {
		r.Error = fmt.Errorf("codexbar CLI not found; install it first: https://github.com/codexbar/codexbar")
		return r
	}
	r.Installed = true

	r.WasAuthed = codexbarIsAuthed()
	if r.WasAuthed {
		r.NowAuthed = true
		return r
	}

	// Try codexbar auth if the subcommand exists
	checkCmd := exec.Command("codexbar", "auth")
	checkCmd.Stdin = os.Stdin
	checkCmd.Stdout = os.Stdout
	checkCmd.Stderr = os.Stderr
	if err := checkCmd.Run(); err != nil {
		// If the auth subcommand doesn't exist, show manual instructions
		r.Error = fmt.Errorf("codexbar auth not available; configure credentials manually per codexbar docs")
		r.NowAuthed = codexbarIsAuthed()
		return r
	}

	r.NowAuthed = codexbarIsAuthed()
	if !r.NowAuthed {
		r.Error = fmt.Errorf("auth flow completed but codexbar still reports unauthenticated")
	}
	return r
}

// CredentialPath returns the human-readable credential path for a provider
func CredentialPath(provider string) string {
	switch strings.ToLower(provider) {
	case "claude":
		return claudeCredPath()
	case "codex":
		p := codexCredPath()
		if os.Getenv("OPENAI_API_KEY") != "" {
			return "$OPENAI_API_KEY"
		}
		return p
	case "gemini":
		return geminiCredPath()
	case "copilot":
		return "gh auth token"
	case "codexbar":
		return "codexbar config"
	default:
		return "unknown"
	}
}
