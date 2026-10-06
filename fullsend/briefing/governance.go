package briefing

import (
	"fmt"
	"slices"
	"strings"
)

// governance emits the two governing batches every route admits. Each source is pinned and verified: the agent
// definition and its output contract at the agents repository SHA the config pins, and AGENTS.md at the protected
// base branch's SHA. AGENTS.md is never read from a pull request's head, so a fork that edits it changes the diff
// the reviewer sees, not the reviewer's instructions.
func governance(r Read) []Batch {
	gov := func(it Item, eligibility string) Item {
		it.Trust, it.InjectionRisk, it.Lineage, it.Eligibility = "verified", "none", "verbatim", eligibility
		return it
	}
	a, c := r.Agent, r.OutputContract
	registry := []Item{
		gov(newItem(fmt.Sprintf("agent:%s@%s", a.Name, a.SHA), "governance.instructions", a.Source, a.SHA,
			"governing", r.ReadAt, a.Body), "agent definition pinned by sha in .fullsend/config.yaml"),
		gov(newItem(fmt.Sprintf("contract:%s/%s", c.ID, c.Version), "governance.output_contract",
			fmt.Sprintf("fullsend-ai/agents/schemas/%s.json", c.ID), c.Version, "governing", r.ReadAt, c.Body),
			"harness validation_loop.schema (ADR 0022)"),
	}
	if e := r.Example; e != nil {
		registry = append(registry, gov(newItem(e.ID, "governance.examples", "fullsend-ai/agents/examples/triage",
			a.SHA, "governing", r.ReadAt, e.Body), "steering example for the decision vocabulary"))
	}
	md := r.RepoAgentsMD
	repo := []Item{gov(newItem(fmt.Sprintf("repo:%s@%s", md.Path, r.BaseSHA), "governance.instructions",
		fmt.Sprintf("%s/%s", r.Repo, md.Path), r.BaseSHA, "governing", r.ReadAt, md.Body),
		"read from the protected base branch at a pinned SHA, never from the PR head")}
	return []Batch{batch("agent-registry", "policy", registry), batch("repo-policy", "policy", repo)}
}

// query is the live task statement. Dispatch writes it from the normalized event; it is never copied from issue or
// pull request text, so a work item's author cannot become the user the agent answers to (R-7).
func query(r Read, task Scope, id, at, body string) Batch {
	it := newItem(id, "interaction.query", fmt.Sprintf("dispatch:%s/runs/%s", r.Repo, r.RunID), "normevent/v1",
		"user", at, body)
	it.Trust, it.InjectionRisk, it.Scope, it.Eligibility = "verified", "untrusted_content", &task,
		"templated from the normalized event"
	return batch("dispatch", "interaction", []Item{it})
}

// freeze builds the snapshot. The budget is the briefing's share of the window; the rest is reserved for what the
// runtime adds after the handoff (R-16).
func freeze(r Read, scope Scope, route, profile []byte, input int, batches []Batch, conflicts []ConflictGroup) Snapshot {
	sortBatches(batches)
	if conflicts == nil {
		conflicts = []ConflictGroup{}
	}
	return Snapshot{
		AssemblyTime: r.AssemblyTime,
		Scope:        scope,
		Budget:       Budget{Input: input, ReservedOutput: Window - input, MarginPercent: MarginPercent},
		Profile:      profile,
		RoutePolicy:  route,
		Tokenizer:    Tokenizer,
		Renderer:     Renderer,
		Batches:      batches,
		Conflicts:    conflicts,
	}
}

func sortBatches(b []Batch) {
	slices.SortFunc(b, func(x, y Batch) int { return strings.Compare(x.Producer.ID, y.Producer.ID) })
}
