package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const adminAPIBase = "https://api.anthropic.com"

// AdminClient wraps Anthropic Admin API calls
type AdminClient struct {
	Key     string
	Version string
}

// GetAdminKey finds the Admin API key from environment or config
func GetAdminKey() string {
	// 1. Environment variable (highest priority)
	if key := os.Getenv("ANTHROPIC_ADMIN_KEY"); key != "" {
		return key
	}

	// 2. Config file
	configDir, _, _ := GetInstallDirs()
	configPath := filepath.Join(configDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	var cfg map[string]interface{}
	if json.Unmarshal(data, &cfg) != nil {
		return ""
	}
	if key, ok := cfg["admin_api_key"].(string); ok && key != "" {
		return key
	}
	return ""
}

// SaveAdminKey stores the admin key in config.json
func SaveAdminKey(key string) error {
	configDir, _, _ := GetInstallDirs()
	configPath := filepath.Join(configDir, "config.json")

	var cfg map[string]interface{}
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	cfg["admin_api_key"] = key

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0600)
}

// NewAdminClient creates a client if an admin key is available
func NewAdminClient() *AdminClient {
	key := GetAdminKey()
	if key == "" {
		return nil
	}
	return &AdminClient{Key: key, Version: "2023-06-01"}
}

func (c *AdminClient) get(path string, params map[string]string) ([]byte, error) {
	url := adminAPIBase + path
	if len(params) > 0 {
		parts := make([]string, 0, len(params))
		for k, v := range params {
			parts = append(parts, k+"="+v)
		}
		url += "?" + strings.Join(parts, "&")
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.Key)
	req.Header.Set("anthropic-version", c.Version)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	return body, nil
}

// CostReport represents the cost report response
type CostReport struct {
	Data    []CostBucket `json:"data"`
	HasMore bool         `json:"has_more"`
}

type CostBucket struct {
	StartAt      string     `json:"bucket_start_time"`
	EndAt        string     `json:"bucket_end_time"`
	CostSubitems []CostItem `json:"cost_subitems"`
}

type CostItem struct {
	Description string  `json:"description"`
	Model       string  `json:"model,omitempty"`
	InferenceGeo string `json:"inference_geo,omitempty"`
	Cost        string  `json:"cost"`
}

// UsageReport represents the usage report response
type UsageReport struct {
	Data    []UsageBucket `json:"data"`
	HasMore bool          `json:"has_more"`
}

type UsageBucket struct {
	StartAt           string `json:"bucket_start_time"`
	EndAt             string `json:"bucket_end_time"`
	Model             string `json:"model,omitempty"`
	ServiceTier       string `json:"service_tier,omitempty"`
	Speed             string `json:"speed,omitempty"`
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	CacheReadTokens   int64  `json:"cache_read_input_tokens"`
	CacheCreateTokens int64  `json:"cache_creation_input_tokens"`
	WebSearchRequests int64  `json:"web_search_requests,omitempty"`
}

// ClaudeCodeReport represents the Claude Code analytics response
type ClaudeCodeReport struct {
	Data    []ClaudeCodeEntry `json:"data"`
	HasMore bool              `json:"has_more"`
}

type ClaudeCodeEntry struct {
	Date           string         `json:"date"`
	Actor          map[string]interface{} `json:"actor"`
	CustomerType   string         `json:"customer_type"`
	TerminalType   string         `json:"terminal_type"`
	CoreMetrics    CoreMetrics    `json:"core_metrics"`
	ToolActions    ToolActions    `json:"tool_actions"`
	ModelBreakdown []ModelEntry   `json:"model_breakdown"`
}

type CoreMetrics struct {
	NumSessions  int        `json:"num_sessions"`
	LinesOfCode  LOCMetrics `json:"lines_of_code"`
	Commits      int        `json:"commits_by_claude_code"`
	PullRequests int        `json:"pull_requests_by_claude_code"`
}

type LOCMetrics struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

type ToolActions struct {
	EditTool     ToolAction `json:"edit_tool"`
	MultiEdit    ToolAction `json:"multi_edit_tool"`
	WriteTool    ToolAction `json:"write_tool"`
	NotebookEdit ToolAction `json:"notebook_edit_tool"`
}

type ToolAction struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}

type ModelEntry struct {
	Model         string         `json:"model"`
	Tokens        TokenBreakdown `json:"tokens"`
	EstimatedCost EstimatedCost  `json:"estimated_cost"`
}

type TokenBreakdown struct {
	Input       int64 `json:"input"`
	Output      int64 `json:"output"`
	CacheRead   int64 `json:"cache_read"`
	CacheCreate int64 `json:"cache_creation"`
}

type EstimatedCost struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

// OrgInfo from /v1/organizations/me
type OrgInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// GetOrgInfo fetches the organization identity
func (c *AdminClient) GetOrgInfo() (*OrgInfo, error) {
	data, err := c.get("/v1/organizations/me", nil)
	if err != nil {
		return nil, err
	}
	var org OrgInfo
	if err := json.Unmarshal(data, &org); err != nil {
		return nil, err
	}
	return &org, nil
}

// GetCostReport fetches cost data for a date range
func (c *AdminClient) GetCostReport(startingAt, endingAt string, groupBy []string) (*CostReport, error) {
	params := map[string]string{
		"starting_at":  startingAt,
		"ending_at":    endingAt,
		"bucket_width": "1d",
	}
	path := "/v1/organizations/cost_report"
	url := path
	if len(groupBy) > 0 {
		for _, g := range groupBy {
			url += "&group_by[]=" + g
		}
	}
	data, err := c.get(path, params)
	if err != nil {
		return nil, err
	}
	var report CostReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// GetUsageReport fetches token usage for a date range
func (c *AdminClient) GetUsageReport(startingAt, endingAt string, groupBy []string, bucketWidth string) (*UsageReport, error) {
	params := map[string]string{
		"starting_at":  startingAt,
		"ending_at":    endingAt,
		"bucket_width": bucketWidth,
	}
	for _, g := range groupBy {
		params["group_by[]"] = g
	}
	data, err := c.get("/v1/organizations/usage_report/messages", params)
	if err != nil {
		return nil, err
	}
	var report UsageReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

// GetClaudeCodeAnalytics fetches Claude Code productivity metrics for a day
func (c *AdminClient) GetClaudeCodeAnalytics(date string) (*ClaudeCodeReport, error) {
	params := map[string]string{
		"starting_at": date,
		"limit":       "100",
	}
	data, err := c.get("/v1/organizations/usage_report/claude_code", params)
	if err != nil {
		return nil, err
	}
	var report ClaudeCodeReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
