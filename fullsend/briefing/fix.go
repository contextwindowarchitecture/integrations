package briefing

import (
	"fmt"
	"strings"
)

// Fix freezes the snapshot for a human-triggered fix run (/fs-fix). fullsend's fix agent documents that the human
// instruction "takes precedence" over the review body. Here that is structural: the instruction is the live query,
// so it may direct the agent, while the review's findings are generated memory, which may not (R-6). The
// repository's AGENTS.md still outranks the instruction, and the run declares that precedence as an instruction
// group so the trace records it on every fix (R-11).
func Fix(f FixRead, route, profile []byte, input int) Snapshot {
	pr, trig, hi := f.PR, f.Trigger, f.HumanInstruction
	task := Scope{Tenant: f.Repo, Task: fmt.Sprintf("pr:%d", pr.Number)}
	scope := task
	scope.User, scope.Session, scope.Step = trig.Actor, "run:"+f.RunID, "fix"

	// The query is the instruction itself: its author's write permission was checked at dispatch (ADR 0054), and
	// the eligibility names the comment it came from. It stays marked as untrusted content, as every query is (R-10).
	queryID := "query:fix/run-" + f.RunID
	q := query(f.Read, task, queryID, hi.At, hi.Body)
	q.Items[0].Eligibility = fmt.Sprintf("HUMAN_INSTRUCTION from /fs-fix comment %d by @%s (permission: %s)",
		hi.CommentID, hi.Author, hi.Permission)

	memory, _ := findings(f.PriorFindings, nil, task, "review finding this fix addresses")
	batches := append(governance(f.Read), fixState(f, task), q, diff(pr, f.Diff, f.ReadAt, task), memory)
	precedence := ConflictGroup{ID: "instruction:repo-rules-over-request", Kind: "instruction",
		Items: []string{fmt.Sprintf("repo:%s@%s", f.RepoAgentsMD.Path, f.BaseSHA), queryID}}
	return freeze(f.Read, scope, route, profile, input, batches, []ConflictGroup{precedence})
}

// fixState is the change proposal, the loop count the pre-script checked against its caps, and who asked.
func fixState(f FixRead, task Scope) Batch {
	pr, trig, it := f.PR, f.Trigger, f.Iteration
	proposal := newItem("state:change-proposal", "state.task", fmt.Sprintf("forge:%s/pulls/%d", f.Repo, pr.Number),
		pr.HeadSHA, "state", f.ReadAt, fmt.Sprintf(
			"PR %s#%d · head %s · base %s · author @%s (permission: %s) · labels: %s",
			f.Repo, pr.Number, pr.HeadSHA, pr.BaseRef, pr.Author, pr.AuthorPermission, strings.Join(pr.Labels, ", ")))
	proposal.Scope, proposal.Eligibility = &task, "forge read at freeze time"
	loop := newItem("state:fix-iteration", "state.task", "fullsend:fix-pre-script", pr.HeadSHA, "state", f.ReadAt,
		fmt.Sprintf("This is human-triggered fix %d of at most %d on this PR; bot-triggered fixes stop at %d.",
			it.Human, it.HumanCap, it.BotCap))
	loop.Scope, loop.Eligibility = &task, "pre-script iteration cap check"
	requester := newItem("state:requester", "state.user", fmt.Sprintf("forge:%s/collaborators/%s", f.Repo, trig.Actor),
		trig.At, "state", f.ReadAt, fmt.Sprintf(
			"@%s requested this fix (%s) with repository permission '%s', resolved by the authorization contract (ADR 0054).",
			trig.Actor, trig.Kind, trig.ActorPermission))
	requester.Scope, requester.Eligibility = &Scope{Tenant: f.Repo, User: trig.Actor}, "dispatch authorization result"
	return batch("forge-state", "state", []Item{proposal, loop, requester})
}
