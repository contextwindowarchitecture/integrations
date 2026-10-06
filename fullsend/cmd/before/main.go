// Command before prints what fullsend hands a runtime today for a scenario's agent: the pinned agent definition and
// the constant prompt. Everything else the agent sees, it fetches itself during its loop, and nothing records it.
//
//	go run ./cmd/before triage-7811
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/contextwindowarchitecture/integrations/fullsend/briefing"
)

// defaultAgentPrompt is fullsend's runtime.DefaultAgentPrompt (internal/runtime/runtime.go).
const defaultAgentPrompt = "Run the agent task"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/before <scenario>")
		os.Exit(2)
	}
	in, err := briefing.Open(".")
	check(err)
	sc, err := in.Load(os.Args[1])
	check(err)
	raw, err := os.ReadFile(filepath.Join(in.Root, "fixtures", sc.Fixture+".json"))
	check(err)
	var f briefing.Read
	check(json.Unmarshal(raw, &f))

	// The fixture's agent body is written for the assembled briefing, so only the pinned file is named here.
	fmt.Printf("== agent definition\n%s at %s, which tells the agent what to fetch\n\n", f.Agent.Source, f.Agent.SHA)
	fmt.Printf("== prompt\n%s\n\n", defaultAgentPrompt)
	fmt.Println("== fetched by the agent itself, mid-loop, as one undifferentiated tool result each")
	switch sc.Agent {
	case "triage":
		var r briefing.IssueRead
		check(json.Unmarshal(raw, &r))
		fmt.Printf("  gh issue view %d --comments   # body by @%s (%s) and %d comments, any author\n",
			r.Issue.Number, r.Issue.Author, r.Issue.AuthorPermission, len(r.Comments))
	case "review-security":
		var r briefing.PullRead
		check(json.Unmarshal(raw, &r))
		fmt.Printf("  gh pr view %d     # includes the author's description\n", r.PR.Number)
		fmt.Printf("  gh pr diff %d     # %d files\n", r.PR.Number, len(r.Diff))
		fmt.Printf("  cat $PRIOR_REVIEW_FILE    # %d earlier findings, dismissed ones included\n", len(r.PriorFindings))
	}
	fmt.Println("\nNo trace: nothing records which of these the model read, in what order, or what it skipped.")
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
