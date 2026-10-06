// Command scenarios checks or rewrites the committed scenarios. In each folder under scenarios/, scenario.json is
// the input, and snapshot.json, trace.json, payload.json and explain.txt are what the code builds from it now. They
// are committed so that a change to a fixture, an agent definition, a route policy or the assembler pin shows up as
// a diff of the briefing someone can review.
//
//	go run ./cmd/scenarios -check    # exit 1 when a committed file differs from what the code builds now
//	go run ./cmd/scenarios -write    # rebuild every scenario, then read the diff before committing it
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/contextwindowarchitecture/integrations/fullsend/briefing"
)

func main() {
	check := flag.Bool("check", false, "fail when a committed scenario differs from what the code builds now")
	write := flag.Bool("write", false, "rebuild every committed scenario")
	flag.Parse()
	if *check == *write {
		fmt.Fprintln(os.Stderr, "pass exactly one of -check or -write")
		os.Exit(2)
	}
	in, err := briefing.Open(".")
	if err != nil {
		fail(err)
	}
	names, err := in.Scenarios()
	if err != nil {
		fail(err)
	}
	var stale []string
	for _, name := range names {
		files, err := in.Files(name)
		if err != nil {
			fail(fmt.Errorf("%s: %w", name, err))
		}
		for _, file := range sorted(files) {
			path := filepath.Join("scenarios", name, file)
			want := files[file]
			have, err := os.ReadFile(path)
			exists := err == nil
			if (want == nil && !exists) || (want != nil && exists && bytes.Equal(have, want)) {
				continue
			}
			if *check {
				stale = append(stale, filepath.Join(name, file))
				continue
			}
			if want == nil {
				fail(os.Remove(path))
				fmt.Println("removed", path)
			} else {
				fail(os.WriteFile(path, want, 0o644))
				fmt.Println("wrote", path)
			}
		}
	}
	if len(stale) > 0 {
		fmt.Fprintln(os.Stderr, "stale (run go run ./cmd/scenarios -write, then review the diff):")
		for _, s := range stale {
			fmt.Fprintln(os.Stderr, "  "+s)
		}
		os.Exit(1)
	}
	if *check {
		fmt.Printf("%d scenarios are current\n", len(names))
	}
}

func sorted(files map[string][]byte) []string {
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fail(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
