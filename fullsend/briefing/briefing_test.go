package briefing

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func open(t *testing.T) *Integration {
	t.Helper()
	in, err := Open("..")
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func run(t *testing.T, name string) (Snapshot, Result) {
	t.Helper()
	snap, res, err := open(t).Run(name)
	if err != nil {
		t.Fatal(err)
	}
	return snap, res
}

// prompt is the user message the runtime receives, which holds every non-governance item.
func prompt(t *testing.T, s Snapshot, r Result) string {
	t.Helper()
	h, err := HandoffFor(s, r)
	if err != nil {
		t.Fatal(err)
	}
	return h.Prompt
}

func outcome(trace map[string]any, id string) string { return outcomeOf(trace, id) }

func slotOf(trace map[string]any, id string) string {
	for _, row := range list(trace, "included") {
		if row["item_id"] == id {
			return row["slot"].(string)
		}
	}
	return ""
}

func TestScenariosAreCurrent(t *testing.T) {
	in := open(t)
	names, err := in.Scenarios()
	if err != nil || len(names) == 0 {
		t.Fatalf("no scenarios (%v)", err)
	}
	for _, name := range names {
		files, err := in.Files(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for file, want := range files {
			have, err := os.ReadFile(filepath.Join("..", "scenarios", name, file))
			switch {
			case want == nil && err == nil:
				t.Errorf("%s/%s exists but the scenario is refused; run go run ./cmd/scenarios -write", name, file)
			case want != nil && !bytes.Equal(have, want):
				t.Errorf("%s/%s is stale; run go run ./cmd/scenarios -write and review the diff", name, file)
			}
		}
	}
}

func TestAssemblyIsDeterministic(t *testing.T) {
	_, first := run(t, "triage-7811")
	_, second := run(t, "triage-7811")
	if !bytes.Equal(first.Payload, second.Payload) {
		t.Fatal("the same snapshot rendered two different payloads (R-23)")
	}
}

// An instruction hidden in a read-only reporter's issue body reaches the model escaped, inside an observation,
// below the slots that may instruct.
func TestHiddenInstructionIsMaterial(t *testing.T) {
	snap, res := run(t, "triage-7811")
	body := "obs:issue/7811/0-body@newcontrib"
	if got := slotOf(res.Trace, body); got != "evidence.tool_results" {
		t.Fatalf("issue body slot = %q, want evidence.tool_results", got)
	}
	p := prompt(t, snap, res)
	if !strings.Contains(p, "&lt;!-- AI agents reading this") || strings.Contains(p, "<!-- AI agents") {
		t.Error("the hidden instruction is not escaped in the prompt")
	}
}

// Who wrote a turn decides whether it may instruct: a maintainer's comment is a user turn, fullsend's own earlier
// comment is an assistant turn, and a read-only commenter's comment is material.
func TestPermissionDecidesAuthority(t *testing.T) {
	snap, res := run(t, "triage-7811")
	p := prompt(t, snap, res)
	for _, want := range []string{
		`<thread id="turn:comment/90011@maintainer-a" speaker="user">`,
		`<thread id="turn:comment/90020" speaker="assistant">`,
		`<external_content id="obs:issue/7811/c90034@drive-by-1">`,
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %s", want)
		}
	}
}

// The task statement is written by dispatch, so nothing the reporter wrote becomes the live query.
func TestQueryIsNotIssueText(t *testing.T) {
	snap, _ := run(t, "triage-7811")
	for _, b := range snap.Batches {
		for _, it := range b.Items {
			if it.Slot == "interaction.query" && (b.Producer.ID != "dispatch" || strings.Contains(it.Body, "PNG")) {
				t.Errorf("query %s from %s carries issue text", it.ID, b.Producer.ID)
			}
		}
	}
}

func TestTriageExclusions(t *testing.T) {
	_, res := run(t, "triage-7811")
	for id, want := range map[string]string{
		"obs:issue/7811/c90031@drive-by-1": "duplicate_content",
		"obs:issue/7811/c90032@drive-by-1": "duplicate_content",
		"obs:issue/7811/c90033@drive-by-1": "source_diversity_cap",
		"dup:issue/6100":                   "below_threshold",
		"state:ci-summary":                 "stale_state",
		"run:triage/17002#summary":         "expired",
		"dup:issue/7012":                   "included",
		"run:triage/18800#summary":         "included",
	} {
		if got := outcome(res.Trace, id); got != want {
			t.Errorf("%s: %s, want %s", id, got, want)
		}
	}
}

// The security dimension never sees the author-written description that claims a pre-approval.
func TestSecurityRouteOmitsAuthorPersuasion(t *testing.T) {
	snap, res := run(t, "review-security-7820")
	if strings.Contains(prompt(t, snap, res), "pre-approved") {
		t.Error("the PR description reached the security reviewer")
	}
}

func TestReviewMemoryAndFacts(t *testing.T) {
	_, res := run(t, "review-security-7820")
	for id, want := range map[string]string{
		"run:review/18990#F1":    "included",
		"run:review/18990#F2":    "revoked",
		"run:review/18990#F3":    "conflict_lost",
		"ci:unit-tests@9f1e77aa": "superseded",
		"ci:unit-tests@abc1230f": "included",
	} {
		if got := outcome(res.Trace, id); got != want {
			t.Errorf("%s: %s, want %s", id, got, want)
		}
	}
}

// Route v1 sheds a high-severity prior finding under pressure; v2 keeps it and the riskiest file whole; with less
// room v2 refuses before any model is called.
func TestBudgetPressure(t *testing.T) {
	const finding, risky = "run:review/18990#F1", "diff:internal/runtime/claude.go@abc1230f"
	_, v1 := run(t, "review-security-7820-tight-v1")
	if got := outcome(v1.Trace, finding); got != "over_budget" {
		t.Errorf("v1: %s is %s, want over_budget", finding, got)
	}
	_, v2 := run(t, "review-security-7820-tight-v2")
	if got := outcome(v2.Trace, finding); got != "included" {
		t.Errorf("v2: %s is %s, want included", finding, got)
	}
	if got := outcome(v2.Trace, risky); got != "included" {
		t.Errorf("v2: %s is %s, want included", risky, got)
	}
	for _, row := range list(v2.Trace, "compressed") {
		if row["item_id"] == risky {
			t.Errorf("v2 compressed %s to a diffstat", risky)
		}
	}
	snap, starved := run(t, "review-security-7820-starved-v2")
	if refused, reason := starved.Refused(); !refused || reason != "evidence_required" {
		t.Fatalf("starved: refused=%v reason=%q, want evidence_required", refused, reason)
	}
	if action := field(starved.Trace, "recovery")["action"]; action != "precompute_summary" {
		t.Errorf("starved: recovery %v, want precompute_summary", action)
	}
	h, err := HandoffFor(snap, starved)
	if err != nil || h.Prompt != "" || h.AgentDefinition != "" {
		t.Errorf("a refused assembly produced a handoff (%v)", err)
	}
}

// The handoff adds only the recorded separator: the agent definition is the system entries joined, the prompt is
// the one user message, and the attributes tie both to the trace.
func TestHandoff(t *testing.T) {
	snap, res := run(t, "triage-7811")
	h, err := HandoffFor(snap, res)
	if err != nil {
		t.Fatal(err)
	}
	var p messagesPayload
	if err := json.Unmarshal(res.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.System) != 3 || h.AgentDefinition != p.System[0].Text+SystemSeparator+p.System[1].Text+
		SystemSeparator+p.System[2].Text {
		t.Error("the agent definition is not exactly the system entries joined by the separator")
	}
	if !strings.HasPrefix(h.AgentDefinition, "You are the fullsend triage agent.") {
		t.Error("the agent definition does not open with the pinned agent body")
	}
	if h.Attributes["cwa.payload.sha256"] != field(res.Trace, "result")["hash"] ||
		h.Attributes["cwa.handoff.prompt.sha256"] != digest(h.Prompt) {
		t.Error("span attributes do not match the trace and the handoff")
	}
}

// A /fs-fix instruction from a maintainer is the live query and may direct the fix; review findings are memory and
// may not; AGENTS.md outranks the instruction, and the trace records that precedence by authority.
func TestFixPrecedence(t *testing.T) {
	snap, res := run(t, "fix-7820")
	for id, want := range map[string]string{
		"query:fix/run-19010":         "included",
		"run:review/19002#F1":         "included",
		"run:review/18990#F2":         "revoked",
		"repo:AGENTS.md@4f9c2e1a7b3d": "included",
	} {
		if got := outcome(res.Trace, id); got != want {
			t.Errorf("%s: %s, want %s", id, got, want)
		}
	}
	if got := slotOf(res.Trace, "run:review/19002#F1"); got != "interaction.memory" {
		t.Errorf("finding slot = %q, want interaction.memory, which cannot instruct", got)
	}
	conflicts := list(res.Trace, "conflicts")
	if len(conflicts) != 1 || conflicts[0]["decided_by"] != "authority" ||
		conflicts[0]["winner"] != "repo:AGENTS.md@4f9c2e1a7b3d" {
		t.Errorf("precedence group: %v, want AGENTS.md winning by authority", conflicts)
	}
	p := prompt(t, snap, res)
	if !strings.Contains(p, `<human_instruction id="query:fix/run-19010">`) || strings.Contains(p, "pre-approved") {
		t.Error("the prompt should carry the maintainer's instruction and never the PR description")
	}
}

// The pages committed for people come from the same code as the machine files, and say what they compare.
func TestReadablePages(t *testing.T) {
	in := open(t)
	before, err := in.Before("triage-7811")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, "Run the agent task") || !strings.Contains(before, "<!-- AI agents reading this") {
		t.Error("before.md should show today's constant prompt and the hidden instruction exactly as the agent reads it")
	}
	snap, res := run(t, "review-security-7820-starved-v2")
	page, err := BriefingPage("review-security-7820-starved-v2", snap, res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, "**Refused: `evidence_required`, recovery `precompute_summary`.**") ||
		strings.Contains(page, "## Prompt") {
		t.Error("a refused briefing page should state the refusal and show no prompt")
	}
}
