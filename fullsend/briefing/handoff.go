package briefing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// SystemSeparator joins the payload's system entries into one agent definition body. It is the only text the
// handoff adds, and Handoff.Attributes records it by hashing what the runtime receives.
const SystemSeparator = "\n\n"

// Handoff is the payload mapped onto the two inputs every fullsend runtime already takes:
//
//   - AgentDefinition is the agent body: claude's --agent file, pi's APPEND_SYSTEM.md, codex's developer_instructions;
//   - Prompt replaces the constant "Run the agent task" (RunParams.Prompt), sent on stdin so a large briefing does
//     not meet the per-argument length limit.
//
// The payload's tools are not handed over. Which tools an agent has stays in its harness and runtime configuration,
// enforced by sandbox hooks outside the model (R-5).
type Handoff struct {
	AgentDefinition string
	Prompt          string
	Attributes      map[string]any
}

type messagesPayload struct {
	System []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"system"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

// HandoffFor maps a result onto the runtime handoff, with the span attributes ADR 0133 adds to the run's root span
// (ADR 0050). A refused result has no definition or prompt, only attributes.
func HandoffFor(s Snapshot, r Result) (Handoff, error) {
	var route struct {
		Route string `json:"route"`
	}
	if err := json.Unmarshal(s.RoutePolicy, &route); err != nil {
		return Handoff{}, err
	}
	profile := field(r.Trace, "profile")
	attrs := map[string]any{
		"cwa.route":           route.Route,
		"cwa.profile.id":      profile["id"],
		"cwa.profile.version": profile["version"],
		"cwa.snapshot_digest": field(r.Trace, "context")["snapshot_digest"],
	}
	refused, reason := r.Refused()
	attrs["cwa.refused"] = refused
	if refused {
		attrs["cwa.refused.reason"] = reason
		if action, ok := field(r.Trace, "recovery")["action"]; ok {
			attrs["cwa.recovery.action"] = action
		}
		return Handoff{Attributes: attrs}, nil
	}

	var p messagesPayload
	if err := json.Unmarshal(r.Payload, &p); err != nil {
		return Handoff{}, fmt.Errorf("reading the %s payload: %w", Renderer, err)
	}
	if len(p.Messages) != 1 || p.Messages[0].Role != "user" {
		return Handoff{}, fmt.Errorf("%s payload has %d messages, want one user message", Renderer, len(p.Messages))
	}
	system := make([]string, len(p.System))
	for i, e := range p.System {
		system[i] = e.Text
	}
	h := Handoff{AgentDefinition: strings.Join(system, SystemSeparator), Prompt: p.Messages[0].Content}
	result := field(r.Trace, "result")
	attrs["cwa.payload.sha256"] = result["hash"]
	attrs["cwa.input_tokens"] = result["input_tokens"]
	attrs["cwa.included"] = len(list(r.Trace, "included"))
	attrs["cwa.excluded"] = len(list(r.Trace, "excluded"))
	attrs["cwa.handoff.agent_definition.sha256"] = digest(h.AgentDefinition)
	attrs["cwa.handoff.prompt.sha256"] = digest(h.Prompt)
	h.Attributes = attrs
	return h, nil
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func field(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func list(m map[string]any, key string) []map[string]any {
	raw, _ := m[key].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		if row, ok := v.(map[string]any); ok {
			out = append(out, row)
		}
	}
	return out
}
