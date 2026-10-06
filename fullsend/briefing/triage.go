package briefing

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Triage freezes the snapshot for a triage run on the route policy at route and profile, with input tokens of
// briefing.
func Triage(f IssueRead, route, profile []byte, input int) Snapshot {
	iss, trig := f.Issue, f.Trigger
	task := Scope{Tenant: f.Repo, Task: fmt.Sprintf("issue:%d", iss.Number)}
	scope := task
	scope.User, scope.Session, scope.Step = trig.Actor, "run:"+f.RunID, "triage"

	batches := append(governance(f.Read),
		triageState(f, task),
		query(f.Read, task, "query:triage/run-"+f.RunID, trig.At, fmt.Sprintf(
			"Triage %s#%d. This run was triggered by @%s adding the label '%s'. "+
				"Decide using the material provided and answer with the triage result object.",
			f.Repo, iss.Number, trig.Actor, trig.Label)))
	batches = append(batches, thread(f, task)...)
	batches = append(batches, duplicates(f), triageMemory(f, task))
	return freeze(f.Read, scope, route, profile, input, batches, nil)
}

// triageState is application-written and current (R-8): the work item's status and labels, and who triggered the
// run with what permission. A cached read older than the route's max_age_seconds is excluded as stale_state.
func triageState(f IssueRead, task Scope) Batch {
	iss, trig := f.Issue, f.Trigger
	user := Scope{Tenant: f.Repo, User: trig.Actor}
	workItem := newItem("state:work-item", "state.task", fmt.Sprintf("forge:%s/issues/%d", f.Repo, iss.Number),
		iss.UpdatedAt, "state", f.ReadAt, fmt.Sprintf(
			"issue %s#%d · %s · labels: %s · title: %s · opened %s by @%s (permission: %s)",
			f.Repo, iss.Number, iss.State, strings.Join(iss.Labels, ", "), iss.Title, iss.CreatedAt, iss.Author,
			iss.AuthorPermission))
	workItem.Scope, workItem.Eligibility = &task, "forge read at freeze time"
	requester := newItem("state:requester", "state.user", fmt.Sprintf("forge:%s/collaborators/%s", f.Repo, trig.Actor),
		trig.At, "state", f.ReadAt, fmt.Sprintf(
			"@%s triggered this run (%s %s) with repository permission '%s', "+
				"resolved by the authorization contract (ADR 0054).",
			trig.Actor, trig.Kind, trig.Label, trig.ActorPermission))
	requester.Scope, requester.Eligibility = &user, "dispatch authorization result"
	items := []Item{workItem, requester}
	for _, s := range f.StaleReads {
		cached := newItem(s.ID, "state.task", fmt.Sprintf("forge:%s/checks", f.Repo), f.BaseSHA[:7], "state",
			s.ReadAt, s.Body)
		cached.Scope, cached.Eligibility = &task, "cached CI summary"
		items = append(items, cached)
	}
	return batch("forge-state", "state", items)
}

// thread splits the issue and its comments by who wrote them, never by what they say (ADR 0098 actor provenance
// becomes CWA authority, R-1, R-7):
//
//   - fullsend's own earlier comments are prior model turns: interaction.history, untrusted, lineage generated;
//   - turns by actors whose current permission is at or above the route's threshold may instruct: history, user;
//   - everything else is material: evidence.tool_results, observation, where the route caps floods per author.
func thread(f IssueRead, task Scope) []Batch {
	iss := f.Issue
	turns := append([]Comment{{Author: iss.Author, Permission: iss.AuthorPermission, CreatedAt: iss.CreatedAt,
		Body: iss.Title + "\n\n" + iss.Body}}, f.Comments...)
	var history, observed []Item
	for _, t := range turns {
		isBody := t.ID == 0
		ref, source := strconv.Itoa(t.ID), fmt.Sprintf("forge:issue/%d/by/%s", iss.Number, t.Author)
		if isBody {
			ref, source = "body", fmt.Sprintf("forge:issue/%d/body", iss.Number)
		}
		var it Item
		switch {
		case t.Bot:
			it = newItem("turn:comment/"+ref, "interaction.history", source, ref, "untrusted", t.CreatedAt, t.Body)
			it.Lineage, it.Eligibility = "generated", "fullsend's own earlier comment"
		case slices.Contains(f.InstructingRoles, t.Permission):
			it = newItem(fmt.Sprintf("turn:comment/%s@%s", ref, t.Author), "interaction.history", source, ref,
				"user", t.CreatedAt, t.Body)
			it.Eligibility = fmt.Sprintf("author permission '%s' >= triage", t.Permission)
		default:
			// The renderer orders a slot's items by id (only history goes by time), so ids that sort in thread
			// order keep the issue body ahead of its comments.
			pos := "c" + ref
			if isBody {
				pos = "0-body"
			}
			it = newItem(fmt.Sprintf("obs:issue/%d/%s@%s", iss.Number, pos, t.Author), "evidence.tool_results",
				source, ref, "observation", t.CreatedAt, t.Body)
			it.Eligibility = fmt.Sprintf("author permission '%s' below triage: data only", t.Permission)
		}
		it.InjectionRisk, it.Scope = "untrusted_content", &task
		if it.Slot == "interaction.history" {
			history = append(history, it)
		} else {
			observed = append(observed, it)
		}
	}
	return []Batch{batch("forge-thread", "interaction", history), batch("forge-observer", "retrieval", observed)}
}

// duplicates is the similarity index's result: one scored item per candidate, never a merged list (R-13).
func duplicates(f IssueRead) Batch {
	var items []Item
	for _, d := range f.DuplicateCandidates {
		it := newItem(fmt.Sprintf("dup:issue/%d", d.Number), "evidence.knowledge",
			fmt.Sprintf("forge:issue/%d", d.Number), d.UpdatedAt, "reference_only", d.UpdatedAt,
			fmt.Sprintf("#%d (%s) %s: %s", d.Number, d.State, d.Title, d.Excerpt))
		it.Relevance, it.InjectionRisk, it.Scope = score(d.Score), "untrusted_content", &Scope{Tenant: f.Repo}
		it.Eligibility = "similarity score from the duplicate index"
		items = append(items, it)
	}
	return batch("duplicate-index", "retrieval", items)
}

// triageMemory is this agent's earlier conclusions on the work item. Each entry expires and names its source run;
// an entry already expired is not emitted but reported, so the trace still shows it (R-9, R-14).
func triageMemory(f IssueRead, task Scope) Batch {
	var items []Item
	var expired []Exclusion
	for _, m := range f.Memory {
		id := fmt.Sprintf("run:triage/%s#summary", m.Run)
		if !after(m.Expires, f.AssemblyTime) {
			expired = append(expired, Exclusion{ItemID: id, Reason: "expired", Stage: "producer"})
			continue
		}
		it := newItem(id, "interaction.memory", "run:triage/"+m.Run, m.Run, "generated", m.At, m.Summary)
		it.Expires, it.Lineage, it.InjectionRisk, it.Scope = m.Expires, "summarised", "untrusted_content", &task
		it.Eligibility = "this agent's own earlier run on this work item"
		items = append(items, it)
	}
	return batch("run-memory", "memory", items, expired...)
}
