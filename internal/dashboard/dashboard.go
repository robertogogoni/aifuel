package dashboard

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/robertogogoni/aifuel/internal/installer"
	"github.com/robertogogoni/aifuel/internal/ui"
)

// ── Messages ────────────────────────────────────────────────────────────────

type tickMsg time.Time
type dataMsg struct{ data map[string]interface{} }
type adminCostMsg struct{ report *installer.CostReport }
type errMsg struct{ err error }

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	tabStyle = lipgloss.NewStyle().
			Foreground(ui.Overlay0).
			Padding(0, 2)
	activeTabStyle = lipgloss.NewStyle().
			Foreground(ui.Peach).
			Bold(true).
			Padding(0, 2).
			Underline(true)
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Mauve).
			MarginBottom(1)
	labelStyle = lipgloss.NewStyle().
			Foreground(ui.Subtext0).
			Width(16)
	valueStyle = lipgloss.NewStyle().
			Foreground(ui.Text)
	dimStyle = lipgloss.NewStyle().
			Foreground(ui.Overlay0)
	costStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Yellow)
	borderStyle = lipgloss.NewStyle().
			Foreground(ui.Surface0)
)

// ── Tab definitions ─────────────────────────────────────────────────────────

var tabs = []string{"Rate Limits", "Cost & Usage", "Analytics", "Account"}

// ── Model ───────────────────────────────────────────────────────────────────

type Model struct {
	width     int
	height    int
	activeTab int
	data      map[string]interface{}
	adminCost *installer.CostReport
	lastFetch time.Time
	err       string
	quitting  bool
}

func NewModel() Model {
	return Model{
		width:     80,
		height:    24,
		activeTab: 0,
	}
}

// ── Key bindings ────────────────────────────────────────────────────────────

type keyMap struct {
	Tab      key.Binding
	ShiftTab key.Binding
	Refresh  key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Tab:      key.NewBinding(key.WithKeys("tab", "l", "right"), key.WithHelp("tab", "next tab")),
	ShiftTab: key.NewBinding(key.WithKeys("shift+tab", "h", "left"), key.WithHelp("shift+tab", "prev tab")),
	Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c", "esc"), key.WithHelp("q", "quit")),
}

// ── Commands ────────────────────────────────────────────────────────────────

func tickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchDataCmd() tea.Cmd {
	return func() tea.Msg {
		script := findAifuelScript("aifuel-claude.sh")
		if script == "" {
			return errMsg{fmt.Errorf("aifuel-claude.sh not found")}
		}
		out, err := exec.Command("bash", script).Output()
		if err != nil {
			return errMsg{err}
		}
		var data map[string]interface{}
		if err := json.Unmarshal(out, &data); err != nil {
			return errMsg{err}
		}
		return dataMsg{data}
	}
}

func fetchAdminCostCmd() tea.Cmd {
	return func() tea.Msg {
		client := installer.NewAdminClient()
		if client == nil {
			return adminCostMsg{nil}
		}
		now := time.Now().UTC()
		report, err := client.GetCostReport(
			now.AddDate(0, 0, -7).Format("2006-01-02T15:04:05Z"),
			now.Format("2006-01-02T15:04:05Z"),
			[]string{"description"},
		)
		if err != nil {
			return adminCostMsg{nil}
		}
		return adminCostMsg{report}
	}
}

func findAifuelScript(name string) string {
	_, _, libDir := installer.GetInstallDirs()
	path := libDir + "/" + name
	if _, err := exec.Command("test", "-f", path).CombinedOutput(); err == nil {
		return path
	}
	// Try common locations
	for _, p := range []string{
		libDir + "/" + name,
		"/usr/share/aifuel/scripts/" + name,
	} {
		if out, _ := exec.Command("test", "-f", p).CombinedOutput(); len(out) == 0 {
			return p
		}
	}
	return libDir + "/" + name // best guess
}

// ── Init ────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetchDataCmd(), fetchAdminCostCmd(), tickCmd())
}

// ── Update ──────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, keys.Tab):
			m.activeTab = (m.activeTab + 1) % len(tabs)
			return m, nil
		case key.Matches(msg, keys.ShiftTab):
			m.activeTab = (m.activeTab - 1 + len(tabs)) % len(tabs)
			return m, nil
		case key.Matches(msg, keys.Refresh):
			return m, tea.Batch(fetchDataCmd(), fetchAdminCostCmd())
		}

	case tickMsg:
		return m, tea.Batch(fetchDataCmd(), tickCmd())

	case dataMsg:
		m.data = msg.data
		m.lastFetch = time.Now()
		m.err = ""
		return m, nil

	case adminCostMsg:
		m.adminCost = msg.report
		return m, nil

	case errMsg:
		m.err = msg.err.Error()
		return m, nil
	}

	return m, nil
}

// ── View ────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Header
	fuel := lipgloss.NewStyle().Foreground(ui.Peach).Bold(true).Render("\u26fd")
	title := lipgloss.NewStyle().Bold(true).Foreground(ui.Mauve).Render("AIFuel Dashboard")
	b.WriteString(fmt.Sprintf("\n %s %s", fuel, title))

	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Truncate(time.Second)
		b.WriteString("  " + dimStyle.Render(fmt.Sprintf("(updated %s ago)", ago)))
	}
	b.WriteString("\n")

	// Tab bar
	b.WriteString(" ")
	for i, tab := range tabs {
		if i == m.activeTab {
			b.WriteString(activeTabStyle.Render(tab))
		} else {
			b.WriteString(tabStyle.Render(tab))
		}
	}
	b.WriteString("\n")
	b.WriteString(" " + borderStyle.Render(strings.Repeat("\u2500", min(m.width-2, 70))) + "\n")

	if m.err != "" {
		b.WriteString("\n " + ui.Error.Render("Error: "+m.err) + "\n")
		return b.String()
	}

	if m.data == nil {
		b.WriteString("\n " + dimStyle.Render("Loading...") + "\n")
		return b.String()
	}

	// Render active tab
	switch m.activeTab {
	case 0:
		b.WriteString(m.viewRateLimits())
	case 1:
		b.WriteString(m.viewCost())
	case 2:
		b.WriteString(m.viewAnalytics())
	case 3:
		b.WriteString(m.viewAccount())
	}

	// Footer
	b.WriteString("\n " + dimStyle.Render("tab: switch  r: refresh  q: quit") + "\n")

	return b.String()
}

func (m Model) viewRateLimits() string {
	var b strings.Builder
	b.WriteString("\n")

	renderBar := func(name string, pct float64, reset string) {
		bar := ui.RenderProgressBar(pct, 25)
		pctStr := ui.ColorForPct(pct).Render(fmt.Sprintf("%3.0f%%", pct))
		countdown := ""
		if reset != "" {
			countdown = dimStyle.Render("  resets in " + fmtCountdown(reset))
		}
		b.WriteString(fmt.Sprintf(" %s %s %s%s\n", labelStyle.Render(name), bar, pctStr, countdown))
	}

	renderBar("5-Hour", getF(m.data, "five_hour"), getS(m.data, "five_hour_reset"))
	renderBar("7-Day", getF(m.data, "seven_day"), getS(m.data, "seven_day_reset"))

	if v := getN(m.data, "seven_day_sonnet"); v >= 0 {
		renderBar("7d Sonnet", v, getS(m.data, "seven_day_sonnet_reset"))
	}
	if v := getN(m.data, "seven_day_opus"); v >= 0 {
		renderBar("7d Opus", v, getS(m.data, "seven_day_opus_reset"))
	}

	// Per-model concurrency
	if rl, ok := m.data["rate_limits"].(map[string]interface{}); ok {
		if ml, ok := rl["model_limits"].(map[string]interface{}); ok && len(ml) > 0 {
			b.WriteString("\n " + headerStyle.Render("Per-Model Concurrency") + "\n")
			for group, limits := range ml {
				limMap, ok := limits.(map[string]interface{})
				if !ok {
					continue
				}
				conc := "-"
				if v, ok := limMap["concurrents"].(float64); ok {
					conc = fmt.Sprintf("%.0f", v)
				}
				name := strings.ReplaceAll(group, "_", " ")
				b.WriteString(fmt.Sprintf(" %s %s concurrent\n", labelStyle.Render(name), dimStyle.Render(conc)))
			}
		}
	}

	return b.String()
}

func (m Model) viewCost() string {
	var b strings.Builder
	b.WriteString("\n")

	// Local cost data (from ccusage)
	b.WriteString(" " + headerStyle.Render("Today (local)") + "\n")
	cost := getF(m.data, "daily_cost")
	msgs := getF(m.data, "session_messages")
	burn := getF(m.data, "burn_rate_per_hour")
	proj := getF(m.data, "projected_daily_cost")

	b.WriteString(fmt.Sprintf(" %s %s  %s\n", labelStyle.Render("Daily cost"),
		costStyle.Render(fmt.Sprintf("$%.2f", cost)),
		dimStyle.Render(fmt.Sprintf("(%.0f messages)", msgs))))
	b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Burn rate"),
		dimStyle.Render(fmt.Sprintf("$%.2f/hr", burn))))
	b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Projected"),
		dimStyle.Render(fmt.Sprintf("$%.2f (16hr day)", proj))))

	// Model breakdown
	if models, ok := m.data["models"].([]interface{}); ok && len(models) > 0 {
		b.WriteString("\n " + headerStyle.Render("Models") + "\n")
		for _, mi := range models {
			mm, ok := mi.(map[string]interface{})
			if !ok {
				continue
			}
			name := getS(mm, "model")
			mcost := getF(mm, "cost")
			b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render(name),
				costStyle.Render(fmt.Sprintf("$%.2f", mcost))))
		}
	}

	// Admin API cost (if available)
	if m.adminCost != nil && len(m.adminCost.Data) > 0 {
		b.WriteString("\n " + headerStyle.Render("Official (Admin API, 7 days)") + "\n")
		var total float64
		for _, bucket := range m.adminCost.Data {
			for _, item := range bucket.CostSubitems {
				var c float64
				fmt.Sscanf(item.Cost, "%f", &c)
				total += c
			}
		}
		b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("7-day total"),
			costStyle.Render(fmt.Sprintf("$%.2f", total/100))))
	}

	return b.String()
}

func (m Model) viewAnalytics() string {
	var b strings.Builder
	b.WriteString("\n")

	if analytics, ok := m.data["analytics"].(map[string]interface{}); ok {
		// Peak hours
		if peak, ok := analytics["peak"].(map[string]interface{}); ok {
			isPeak := false
			if v, ok := peak["is_peak"].(bool); ok {
				isPeak = v
			}
			label := "off-peak"
			color := ui.Green
			if isPeak {
				label = "PEAK (2x burn)"
				color = ui.Red
			}
			b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Period"),
				lipgloss.NewStyle().Foreground(color).Bold(true).Render(label)))

			if mins, ok := peak["transition_minutes"].(float64); ok && mins > 0 {
				b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Next transition"),
					dimStyle.Render(fmt.Sprintf("%.0f min", mins))))
			}
		}

		// Depletion
		if dep, ok := analytics["depletion_5h"].(map[string]interface{}); ok {
			if mins, ok := dep["depletes_in_minutes"].(float64); ok {
				status := getS(dep, "status")
				color := ui.Green
				if status == "warning" {
					color = ui.Yellow
				} else if status == "critical" {
					color = ui.Red
				}
				var depStr string
				if mins > 120 {
					depStr = fmt.Sprintf("%.1f hours", mins/60)
				} else {
					depStr = fmt.Sprintf("%.0f min", mins)
				}
				b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("5h depletion"),
					lipgloss.NewStyle().Foreground(color).Render(depStr)))
			}
		}

		// Binding limit
		if bind, ok := analytics["binding"].(map[string]interface{}); ok {
			limit := getS(bind, "binding_limit")
			pct := getF(bind, "binding_pct")
			b.WriteString(fmt.Sprintf(" %s %s at %s\n", labelStyle.Render("Binding limit"),
				dimStyle.Render(limit),
				ui.ColorForPct(pct).Render(fmt.Sprintf("%.0f%%", pct))))
		}

		// Session
		if sess, ok := analytics["session"].(map[string]interface{}); ok {
			if mins, ok := sess["session_minutes"].(float64); ok {
				b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Session"),
					dimStyle.Render(fmt.Sprintf("%.0f min", mins))))
			}
		}

		// Recent prompts
		if prompts, ok := analytics["recent_prompts"].([]interface{}); ok && len(prompts) > 0 {
			b.WriteString("\n " + headerStyle.Render("Recent Prompts") + "\n")
			for i, pi := range prompts {
				if i >= 5 {
					break
				}
				p, ok := pi.(map[string]interface{})
				if !ok {
					continue
				}
				model := getS(p, "model")
				pcost := getF(p, "cost")
				tokens := getF(p, "total_tokens")
				b.WriteString(fmt.Sprintf(" %s  %s  %s\n",
					dimStyle.Render(strings.Replace(model, "claude-", "", 1)),
					costStyle.Render(fmt.Sprintf("$%.3f", pcost)),
					dimStyle.Render(fmtTokensInt(int64(tokens)))))
			}
		}
	} else {
		b.WriteString(" " + dimStyle.Render("No analytics data available yet.") + "\n")
	}

	return b.String()
}

func (m Model) viewAccount() string {
	var b strings.Builder
	b.WriteString("\n")

	plan := getS(m.data, "plan")
	source := getS(m.data, "data_source")
	sessions := getF(m.data, "total_sessions")
	outTok := getF(m.data, "total_output_tokens")
	cacheRead := getF(m.data, "total_cache_read_tokens")

	b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Plan"),
		lipgloss.NewStyle().Foreground(ui.Peach).Bold(true).Render(plan)))
	b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Data source"),
		dimStyle.Render(source)))
	b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Sessions today"),
		dimStyle.Render(fmt.Sprintf("%.0f", sessions))))
	b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Output tokens"),
		dimStyle.Render(fmtTokensInt(int64(outTok)))))
	b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Cache read"),
		dimStyle.Render(fmtTokensInt(int64(cacheRead)))))

	// Account info from org endpoint
	if acct, ok := m.data["account"].(map[string]interface{}); ok {
		if tier := getS(acct, "rate_limit_tier"); tier != "" {
			b.WriteString("\n " + headerStyle.Render("Organization") + "\n")
			b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Rate tier"),
				lipgloss.NewStyle().Foreground(ui.Peach).Render(tier)))
			if billing := getS(acct, "billing_type"); billing != "" {
				b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Billing"),
					dimStyle.Render(billing)))
			}
			if caps, ok := acct["capabilities"].([]interface{}); ok {
				capStrs := make([]string, len(caps))
				for i, c := range caps {
					if s, ok := c.(string); ok {
						capStrs[i] = s
					}
				}
				b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Capabilities"),
					dimStyle.Render(strings.Join(capStrs, ", "))))
			}
		}
	}

	// Admin API status
	b.WriteString("\n " + headerStyle.Render("Admin API") + "\n")
	if installer.GetAdminKey() != "" {
		b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Status"),
			lipgloss.NewStyle().Foreground(ui.Green).Render("configured")))
	} else {
		b.WriteString(fmt.Sprintf(" %s %s\n", labelStyle.Render("Status"),
			dimStyle.Render("not configured (run: aifuel admin setup)")))
	}

	return b.String()
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func getF(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func getN(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return -1
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return -1
}

func getS(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func fmtCountdown(iso string) string {
	if iso == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	diff := int(time.Until(t).Seconds())
	if diff <= 0 {
		return "expired"
	}
	d := diff / 86400
	h := (diff % 86400) / 3600
	m := (diff % 3600) / 60
	switch {
	case d > 0:
		return fmt.Sprintf("%dd %dh", d, h)
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return "< 1m"
	}
}

func fmtTokensInt(n int64) string {
	switch {
	case n >= 1000000:
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Run launches the dashboard TUI
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}
