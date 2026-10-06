// Command after assembles a scenario's briefing and prints what the runtime would receive: the decision table, the
// agent definition, the prompt and the span attributes. A refused assembly prints the refusal; no runtime starts.
//
//	go run ./cmd/after triage-7811
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/contextwindowarchitecture/integrations/fullsend/briefing"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/after <scenario>")
		os.Exit(2)
	}
	in, err := briefing.Open(".")
	check(err)
	snap, res, err := in.Run(os.Args[1])
	check(err)
	h, err := briefing.HandoffFor(snap, res)
	check(err)

	fmt.Print(briefing.Explain(res.Trace))
	if refused, _ := res.Refused(); !refused {
		fmt.Printf("\n== agent definition (claude --agent / pi APPEND_SYSTEM.md / codex developer_instructions)\n%s\n",
			h.AgentDefinition)
		fmt.Printf("\n== prompt (stdin, replacing %q)\n%s", "Run the agent task", h.Prompt)
	}
	attrs, err := json.MarshalIndent(h.Attributes, "", "  ")
	check(err)
	fmt.Printf("\n== span attributes on the run's root span\n%s\n", attrs)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
