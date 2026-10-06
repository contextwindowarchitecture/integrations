package briefing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The two pages each scenario commits for people: before.md, what fullsend's runtime receives today, and
// briefing.md, what it would receive with CWA. Both are generated, so they cannot drift from the code.

// defaultAgentPrompt is fullsend's runtime.DefaultAgentPrompt (internal/runtime/runtime.go).
const defaultAgentPrompt = "Run the agent task"

// Before renders what fullsend hands the runtime today for the scenario's agent, and what the agent then reads
// for itself. The fetched text is approximately what gh prints for the fixture: one tool result, every author
// alike, nothing escaped.
func (in *Integration) Before(name string) (string, error) {
	sc, err := in.Load(name)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(in.Root, "fixtures", sc.Fixture+".json"))
	if err != nil {
		return "", err
	}
	var r Read
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: what the %s agent gets today\n\n", name, sc.Agent)
	fmt.Fprintf(&b, "fullsend starts the runtime with two things:\n\n")
	fmt.Fprintf(&b, "- **Agent definition:** `%s` at `%s`, pinned in `.fullsend/config.yaml`. It tells the agent what to go and read.\n", r.Agent.Source, r.Agent.SHA)
	fmt.Fprintf(&b, "- **Prompt:** `%s`\n\n", defaultAgentPrompt)
	b.WriteString("Everything else the agent reads for itself during its loop, with its own tools. ")
	switch sc.Agent {
	case "triage":
		var f IssueRead
		if err := json.Unmarshal(raw, &f); err != nil {
			return "", err
		}
		b.WriteString("The issue and all its comments come back as one tool result. Each comment shows its author's " +
			"name, but nothing marks whose words may direct the agent and whose are only material, and markup inside a " +
			"body reaches the model as written.\n\n")
		beforeTriage(&b, f)
		notToday(&b, "Duplicate candidates: fullsend has no similarity index, so the agent searches issues itself, if it does.",
			"Earlier triage runs on this issue: nothing carries them forward.",
			"Whether the cached CI summary is current: nothing dates it.")
	case "review-security":
		var f PullRead
		if err := json.Unmarshal(raw, &f); err != nil {
			return "", err
		}
		b.WriteString("Each read below is one tool result. Nothing marks the author-written description as " +
			"persuasion rather than evidence, or says which earlier finding is still current.\n\n")
		beforeReview(&b, f)
	case "fix":
		var f FixRead
		if err := json.Unmarshal(raw, &f); err != nil {
			return "", err
		}
		b.WriteString("Each read below is one tool result. Nothing ranks the maintainer's request against the " +
			"repository's rules, or says which earlier finding is still current.\n\n")
		beforeFix(&b, f)
	}
	b.WriteString("Nothing records which of these the model read, in what order, or what it skipped. " +
		"[briefing.md](briefing.md) is the same run with CWA.\n")
	return b.String(), nil
}

func beforeTriage(b *strings.Builder, f IssueRead) {
	iss := f.Issue
	fmt.Fprintf(b, "`gh issue view %d --comments`\n\n~~~text\n", iss.Number)
	fmt.Fprintf(b, "title:\t%s\nstate:\t%s\nauthor:\t%s\nlabels:\t%s\n--\n%s\n", iss.Title, iss.State, iss.Author,
		strings.Join(iss.Labels, ", "), iss.Body)
	for _, c := range f.Comments {
		fmt.Fprintf(b, "--\n%s • %s\n%s\n", c.Author, c.CreatedAt, c.Body)
	}
	b.WriteString("~~~\n\n")
}

func beforeReview(b *strings.Builder, f PullRead) {
	pr := f.PR
	fmt.Fprintf(b, "`gh pr view %d`\n\n~~~text\ntitle:\t%s\nauthor:\t%s\nlabels:\t%s\n--\n%s\n~~~\n\n", pr.Number,
		pr.Title, pr.Author, strings.Join(pr.Labels, ", "), pr.Description)
	beforeDiff(b, pr.Number, f.Diff)
	beforeChecks(b, pr.Number, f.Checks)
	beforeFindings(b, "$PRIOR_REVIEW_FILE", "prior-review.txt, the review bot's last review comment", f.PriorFindings)
	notToday(b, "The tier-1 risk score: the harness pre-script computes it before the sandbox exists, but nothing "+
		"passes it to the agent (#5756).")
}

func beforeFix(b *strings.Builder, f FixRead) {
	fmt.Fprintf(b, "`echo \"$HUMAN_INSTRUCTION\"`\n\n~~~text\n%s\n~~~\n\n", f.HumanInstruction.Body)
	beforeFindings(b, "review-body.txt", "the latest review, which the fix workflow prefetches", f.PriorFindings)
	beforeDiff(b, f.PR.Number, f.Diff)
}

func beforeChecks(b *strings.Builder, number int, checks []Check) {
	fmt.Fprintf(b, "`gh pr checks %d`, the latest run of each check\n\n~~~text\n", number)
	latest := map[string]Check{}
	var names []string
	for _, c := range checks {
		if prev, ok := latest[c.Name]; !ok || after(c.CompletedAt, prev.CompletedAt) {
			if !ok {
				names = append(names, c.Name)
			}
			latest[c.Name] = c
		}
	}
	for _, n := range names {
		fmt.Fprintf(b, "%s\t%s\t%s\n", n, latest[n].Conclusion, latest[n].HeadSHA)
	}
	b.WriteString("~~~\n\n")
}

// notToday lists what the briefing carries that the agent has no source for today. These come from producers, which
// fullsend would write whatever assembler it used; the assembler's part is labeling, ordering, capping, deciding
// conflicts, fitting, refusing and tracing.
func notToday(b *strings.Builder, lines ...string) {
	b.WriteString("Not available to the agent today, and in the briefing only because a producer supplies it:\n\n")
	for _, l := range lines {
		fmt.Fprintf(b, "- %s\n", l)
	}
	b.WriteString("\n")
}

func beforeDiff(b *strings.Builder, number int, files []FileDiff) {
	fmt.Fprintf(b, "`gh pr diff %d`\n\n~~~diff\n", number)
	for _, d := range files {
		fmt.Fprintf(b, "--- a/%s\n+++ b/%s\n%s", d.Path, d.Path, d.Hunk)
	}
	b.WriteString("~~~\n\n")
}

func beforeFindings(b *strings.Builder, file, what string, prior []PriorFinding) {
	fmt.Fprintf(b, "`cat %s`: %s, passed in whole. A finding a maintainer has since rejected, by replying or resolving "+
		"the thread, is still in it.\n\n~~~text\n", file, what)
	for _, p := range prior {
		fmt.Fprintf(b, "%s: %s\n", p.ID, p.Body)
	}
	b.WriteString("~~~\n\n")
}

// BriefingPage renders the assembled briefing: what the runtime would receive, then every item left out and every
// declared conflict, from the trace.
func BriefingPage(name string, s Snapshot, r Result) (string, error) {
	h, err := HandoffFor(s, r)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	p := field(r.Trace, "profile")
	fmt.Fprintf(&b, "# %s: the assembled briefing\n\n", name)
	fmt.Fprintf(&b, "Profile `%v` v%v, snapshot `%v`.", p["id"], p["version"], field(r.Trace, "context")["snapshot_digest"])
	if refused, reason := r.Refused(); refused {
		outcome := fmt.Sprintf("`%s`", reason)
		hint := ""
		if action, ok := field(r.Trace, "recovery")["action"]; ok {
			outcome += fmt.Sprintf(", recovery `%v`", action)
			hint = " " + recoveryHints[fmt.Sprint(action)]
		}
		fmt.Fprintf(&b, "\n\n**Refused: %s.** No sandbox starts and no model is called.%s fullsend maps the "+
			"refusal to an outcome instead: see [refusals](../../README.md#46-refusals-become-fullsend-outcomes-with-no-model-tokens-spent)."+
			"\n\n", outcome, hint)
	} else {
		res, budget := field(r.Trace, "result"), field(r.Trace, "budget")
		fmt.Fprintf(&b, " %v of %v input tokens, payload SHA-256 `%v`. Compare [before.md](before.md).\n\n",
			res["input_tokens"], budget["input"], res["hash"])
		b.WriteString("## Agent definition\n\nThe system entries, joined by a blank line. fullsend writes this where " +
			"it writes the agent body today: `claude --agent`, pi's `APPEND_SYSTEM.md`, codex's `developer_instructions`.\n\n")
		fmt.Fprintf(&b, "~~~text\n%s\n~~~\n\n", h.AgentDefinition)
		fmt.Fprintf(&b, "## Prompt\n\nThe one user message, sent on stdin in place of `%s`. Bodies are escaped, "+
			"and each wrapper names the item it holds, so text cannot forge a wrapper or another author.\n\n", defaultAgentPrompt)
		fmt.Fprintf(&b, "~~~xml\n%s~~~\n\n", h.Prompt)
	}

	if rows := list(r.Trace, "excluded"); len(rows) > 0 {
		b.WriteString("## Left out, and why\n\n| Item | Slot | Decided by | Reason |\n| --- | --- | --- | --- |\n")
		for _, row := range rows {
			slot, _ := row["slot"].(string)
			reason := fmt.Sprintf("`%v`", row["reason"])
			for _, key := range []string{"duplicate_of", "superseded_by"} {
				if kept, ok := row[key]; ok {
					reason += fmt.Sprintf(", keeps `%v`", kept)
				}
			}
			decided := "the assembler"
			if row["stage"] == "producer" {
				decided = "its producer"
			}
			fmt.Fprintf(&b, "| `%v` | %s | %s | %s |\n", row["item_id"], code(slot), decided, reason)
		}
		b.WriteString("\n")
	}
	if rows := list(r.Trace, "compressed"); len(rows) > 0 {
		b.WriteString("## Shortened\n\n| Item | Replaced by | Tokens |\n| --- | --- | --- |\n")
		for _, row := range rows {
			fmt.Fprintf(&b, "| `%v` | `%v` | %v → %v |\n", row["item_id"], row["variant_id"], row["from"], row["to"])
		}
		b.WriteString("\n")
	}
	if rows := list(r.Trace, "conflicts"); len(rows) > 0 {
		b.WriteString("## Declared conflicts\n\nThe run declares these from structured data before assembly; the " +
			"assembler never looks for contradictions in text. It decides each one before fitting the budget.\n\n" +
			"| Group | Kind | Outcome | Winner |\n| --- | --- | --- | --- |\n")
		shedWinner := false
		for _, c := range rows {
			winner := "-"
			if w, ok := c["winner"]; ok {
				winner = fmt.Sprintf("`%v`", w)
				shedWinner = shedWinner || outcomeOf(r.Trace, fmt.Sprint(w)) == "over_budget"
			}
			fmt.Fprintf(&b, "| `%v` | %v | %v by %v | %s |\n", c["group_id"], c["kind"], c["resolution"], c["decided_by"], winner)
		}
		if shedWinner {
			b.WriteString("\nA winner can still be left out afterwards for budget, as here: winning a conflict decides " +
				"which claim is current, not that it fits.\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("[trace.json](trace.json) holds every decision, including each included item's token count; " +
		"[snapshot.json](snapshot.json) is the frozen input it was assembled from.\n")
	return b.String(), nil
}

func code(s string) string {
	if s == "" {
		return "-"
	}
	return "`" + s + "`"
}

// recoveryHints say in plain words what each recovery action asks fullsend to do before it tries again (R-12).
var recoveryHints = map[string]string{
	"request_context":    "The assembler asks for more context: the evidence the route requires was never supplied.",
	"precompute_summary": "The assembler asks for shorter forms computed ahead of time, such as a review split per file, then a new snapshot.",
	"retrieve_narrower":  "The assembler asks for narrower retrieval: the shorter forms supplied still did not fit.",
}

// outcomeOf finds an item's row in a trace: "included", or its exclusion reason.
func outcomeOf(trace map[string]any, id string) string {
	for _, row := range list(trace, "included") {
		if row["item_id"] == id {
			return "included"
		}
	}
	for _, row := range list(trace, "excluded") {
		if row["item_id"] == id {
			reason, _ := row["reason"].(string)
			return reason
		}
	}
	return "absent"
}
