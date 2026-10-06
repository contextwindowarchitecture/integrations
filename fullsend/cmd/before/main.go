// Command before prints what fullsend hands a runtime today for a scenario's agent: the pinned agent definition,
// the constant prompt, and the text the agent then fetches for itself. scenarios/<name>/before.md is the same page.
//
//	go run ./cmd/before triage-7811
package main

import (
	"fmt"
	"os"

	"github.com/contextwindowarchitecture/integrations/fullsend/briefing"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/before <scenario>")
		os.Exit(2)
	}
	in, err := briefing.Open(".")
	if err == nil {
		var page string
		if page, err = in.Before(os.Args[1]); err == nil {
			fmt.Print(page)
			return
		}
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
