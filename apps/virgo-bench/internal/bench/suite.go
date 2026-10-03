package bench

import (
	"encoding/json"
	"fmt"
	"strings"

	"virgo-bench/internal/ollama"
)

// Config selects what a run measures.
type Config struct {
	Model      string `json:"model"`
	WarmRuns   int    `json:"warm_runs"`
	NumPredict int    `json:"num_predict"`
	Cold       bool   `json:"cold"`
	MultiTurn  bool   `json:"multi_turn"`
	Tools      bool   `json:"tools"`
}

// Normalize fills defaults and validates.
func (c *Config) Normalize() error {
	c.Model = strings.TrimSpace(c.Model)
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}
	if c.WarmRuns <= 0 {
		c.WarmRuns = 5
	}
	if c.WarmRuns > 50 {
		return fmt.Errorf("warm_runs must be at most 50")
	}
	if c.NumPredict <= 0 {
		c.NumPredict = 64
	}
	if c.NumPredict > 1024 {
		return fmt.Errorf("num_predict must be at most 1024")
	}
	return nil
}

// WarmPrompt is the fixed single-turn prompt used for cold and warm runs.
const WarmPrompt = "Explain in a short paragraph why the sky is blue."

// MultiTurnPrompts is a scripted conversation. The last turn checks that
// earlier history was actually applied.
var MultiTurnPrompts = []string{
	"Hi! My name is Calum and I'm benchmarking small language models. Reply in one sentence.",
	"Give me three short tips for writing fast Go code.",
	"Summarize your previous answer in one sentence.",
	"What is my name? Answer with just the name.",
}

// MultiTurnRecall is the answer expected from the last multi-turn prompt.
const MultiTurnRecall = "calum"

// ToolCase is one tool-calling prompt and the call it should produce.
type ToolCase struct {
	Prompt       string
	ExpectedTool string
	RequiredArgs []string
}

func fn(name, desc string, props map[string]string, required ...string) ollama.Tool {
	p := map[string]any{}
	for k, d := range props {
		p[k] = map[string]any{"type": "string", "description": d}
	}
	return ollama.Tool{Type: "function", Function: ollama.ToolFunction{
		Name: name, Description: desc,
		Parameters: map[string]any{"type": "object", "properties": p, "required": required},
	}}
}

// Tools are offered on every tool-calling prompt, so the model has to pick
// the right one.
var Tools = []ollama.Tool{
	fn("get_weather", "Get the current weather for a city.", map[string]string{"city": "City name, e.g. Paris"}, "city"),
	fn("calculate", "Evaluate an arithmetic expression.", map[string]string{"expression": "Expression, e.g. 2*(3+4)"}, "expression"),
	fn("get_time", "Get the current local time in a timezone.", map[string]string{"timezone": "IANA timezone, e.g. Asia/Tokyo"}, "timezone"),
	fn("search_web", "Search the web.", map[string]string{"query": "Search query"}, "query"),
	fn("send_email", "Send an email.", map[string]string{"to": "Recipient address", "subject": "Subject line", "body": "Message body"}, "to", "subject", "body"),
}

// ToolCases is the tool-calling suite.
var ToolCases = []ToolCase{
	{"What's the weather like in San Francisco right now?", "get_weather", []string{"city"}},
	{"What is 1234 multiplied by 5678? Use the calculator.", "calculate", []string{"expression"}},
	{"What time is it in Tokyo right now?", "get_time", []string{"timezone"}},
	{"Search the web for the latest Raspberry Pi 5 news.", "search_web", []string{"query"}},
	{"Email alice@example.com with the subject 'Lunch' and tell her I'll be 10 minutes late.", "send_email", []string{"to", "subject", "body"}},
}

// toolNumPredict caps tool-call responses; a call needs more room than the
// default warm-run budget.
const toolNumPredict = 256

// CheckToolCall reports whether calls contains the expected function with
// every required argument present and non-empty. Arguments may arrive as a
// JSON object or as a JSON-encoded string.
func CheckToolCall(calls []ollama.ToolCall, tc ToolCase) bool {
	for _, c := range calls {
		if c.Function.Name != tc.ExpectedTool {
			continue
		}
		args := map[string]any{}
		raw := c.Function.Arguments
		var s string
		if json.Unmarshal(raw, &s) == nil {
			raw = json.RawMessage(s)
		}
		if json.Unmarshal(raw, &args) != nil {
			continue
		}
		ok := true
		for _, k := range tc.RequiredArgs {
			if v, found := args[k]; !found || fmt.Sprint(v) == "" {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
