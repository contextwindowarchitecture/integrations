package briefing

import (
	"fmt"
	"strings"
	"time"
)

// riskyPrefixes are the paths ADR 0089's tier-1 script treats as high risk.
var riskyPrefixes = []string{"internal/security/", "internal/mint", "internal/runtime/", "internal/sandbox/",
	".github/workflows/"}

// pathRisk scores a changed file for the security route. It is the file's rank in evidence.tool_results, so the
// riskiest file is the last to shed under budget pressure (R-16); a file with no score would rank by id instead.
func pathRisk(path string) float64 {
	switch {
	case strings.HasSuffix(path, "_test.go"):
		return 0.5
	case hasAnyPrefix(path, riskyPrefixes):
		return 0.95
	case strings.HasPrefix(path, "docs/"):
		return 0.3
	}
	return 0.6
}

// ReviewSecurity freezes the snapshot for the security dimension of a review. The pull request description and
// comment thread are never produced: they are author-controlled, and this dimension judges the change itself.
func ReviewSecurity(f PullRead, route, profile []byte, input int) Snapshot {
	pr, trig := f.PR, f.Trigger
	task := Scope{Tenant: f.Repo, Task: fmt.Sprintf("pr:%d", pr.Number)}
	scope := task
	scope.User, scope.Session, scope.Step = trig.Actor, "run:"+f.RunID, "review-security"

	memory, conflicts := reviewMemory(f, task)
	batches := append(governance(f.Read),
		reviewState(f, task),
		query(f.Read, task, "query:review-security/run-"+f.RunID, trig.At, fmt.Sprintf(
			"Review the security of %s#%d at head %s. Re-verify each prior finding against the current diff "+
				"and answer with the review dimension result object.", f.Repo, pr.Number, pr.HeadSHA)),
		diff(f, task), checks(f, task), memory)
	return freeze(f.Read, scope, route, profile, input, batches, conflicts)
}

// reviewState carries the change proposal and the pre-script's tier-1 risk score. The score was computed in bash
// before the sandbox existed; as state it reaches the model directly instead of being re-derived (fullsend #5756).
func reviewState(f PullRead, task Scope) Batch {
	pr, trig, risk := f.PR, f.Trigger, f.RiskTier1
	origin := "same-repo"
	if pr.Fork {
		origin = "fork"
	}
	proposal := newItem("state:change-proposal", "state.task", fmt.Sprintf("forge:%s/pulls/%d", f.Repo, pr.Number),
		pr.HeadSHA, "state", f.ReadAt, fmt.Sprintf(
			"PR %s#%d · head %s · base %s · %s · author @%s (permission: %s) · labels: %s · linked issue #%d",
			f.Repo, pr.Number, pr.HeadSHA, pr.BaseRef, origin, pr.Author, pr.AuthorPermission,
			strings.Join(pr.Labels, ", "), pr.LinkedIssue))
	proposal.Scope, proposal.Eligibility = &task, "forge read at freeze time"
	tier1 := newItem("state:risk-tier1", "state.task", "fullsend:risk-tier1.sh", pr.HeadSHA, "state", f.ReadAt,
		fmt.Sprintf("Deterministic tier-1 risk score %d/5: %s", risk.Score, strings.Join(risk.Signals, "; ")))
	tier1.Scope, tier1.Eligibility = &task, "pre-script output (ADR 0089 tier 1)"
	requester := newItem("state:requester", "state.user", fmt.Sprintf("forge:%s/dispatch", f.Repo), trig.At, "state",
		f.ReadAt, fmt.Sprintf("Triggered by %s under the service identity @%s.", trig.Kind, trig.Actor))
	requester.Scope, requester.Eligibility = &Scope{Tenant: f.Repo, User: trig.Actor}, "dispatch authorization result"
	return batch("forge-state", "state", []Item{proposal, tier1, requester})
}

// diff emits one item per changed file at the head (R-13), scored by path risk. A diffstat variant (R-18) lets a
// low-risk file shrink before anything is dropped; a high-risk file gets none, because on a security route a
// diffstat of it is too lossy to review from, so it is kept whole or not at all.
func diff(f PullRead, task Scope) Batch {
	head := f.PR.HeadSHA
	var items []Item
	for _, d := range f.Diff {
		it := newItem(fmt.Sprintf("diff:%s@%s", d.Path, head), "evidence.tool_results",
			fmt.Sprintf("forge:pr/%d/file/%s", f.PR.Number, d.Path), head, "observation", f.ReadAt, d.Hunk)
		it.Relevance, it.InjectionRisk, it.Scope = score(pathRisk(d.Path)), "untrusted_content", &task
		it.Eligibility = "changed file at head, ranked by path risk"
		if !strings.HasSuffix(d.Path, "_test.go") && pathRisk(d.Path) < 0.9 {
			it.Variants = []Variant{{ID: fmt.Sprintf("diffstat:%s@%s", d.Path, head), Body: d.Stat,
				Method: "diffstat", Lineage: "extracted"}}
		}
		items = append(items, it)
	}
	return batch("forge-diff", "retrieval", items)
}

// checks emits each CI check run as an observation. Every run of one check shares a source, so a route that sets
// supersede: source keeps only the latest (R-25).
func checks(f PullRead, task Scope) Batch {
	var items []Item
	for _, c := range f.Checks {
		it := newItem(fmt.Sprintf("ci:%s@%s", c.Name, c.HeadSHA), "evidence.tool_results", "ci:check/"+c.Name,
			c.HeadSHA, "observation", c.CompletedAt, c.Summary)
		it.Relevance, it.InjectionRisk, it.Scope = score(0.6), "untrusted_content", &task
		it.Eligibility = "check run conclusion"
		items = append(items, it)
	}
	return batch("ci-checks", "retrieval", items)
}

// reviewMemory is earlier reviews' findings on this pull request. A finding a human dismissed is revoked: not
// emitted, but reported (R-9, R-14). A finding that is a factual claim joins a declared fact group with the latest
// run of the check it is about, and route precedence decides between them, never the wording (R-11).
func reviewMemory(f PullRead, task Scope) (Batch, []ConflictGroup) {
	var items []Item
	var revoked []Exclusion
	var groups []ConflictGroup
	for _, p := range f.PriorFindings {
		id := fmt.Sprintf("run:review/%s#%s", p.Run, p.ID)
		if p.RevokedBy != "" {
			revoked = append(revoked, Exclusion{ItemID: id, Reason: "revoked", Stage: "producer"})
			continue
		}
		it := newItem(id, "interaction.memory", "run:review/"+p.Run, p.Run, "generated", p.At, p.Body)
		it.Expires, it.Lineage, it.InjectionRisk, it.Scope = p.Expires, "summarised", "untrusted_content", &task
		it.Eligibility = "security finding from an earlier review of this PR"
		if p.Fact != "" {
			// A factual claim, not a finding. The security route raises memory to protected so findings are never
			// shed; in a slot only the route raised, an item may lower its own tier (R-16), so this claim can
			// still lose its fact group to fresher CI.
			it.Tier = "compressible"
			if latest, ok := latestCheck(f.Checks, "unit-tests"); ok {
				groups = append(groups, ConflictGroup{ID: "fact:" + p.Fact, Kind: "fact", Fact: p.Fact,
					Items: []string{id, fmt.Sprintf("ci:%s@%s", latest.Name, latest.HeadSHA)}})
			}
		}
		items = append(items, it)
	}
	return batch("review-memory", "memory", items, revoked...), groups
}

func latestCheck(checks []Check, name string) (Check, bool) {
	var latest Check
	found := false
	for _, c := range checks {
		if c.Name == name && (!found || after(c.CompletedAt, latest.CompletedAt)) {
			latest, found = c, true
		}
	}
	return latest, found
}

// after reports whether instant a is later than b. A timestamp that does not parse counts as later, so the item
// carrying it is emitted and the assembler, not the producer, records it as invalid (R-2).
func after(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	if errA != nil || errB != nil {
		return true
	}
	return ta.After(tb)
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
