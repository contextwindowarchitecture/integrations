package briefing

import (
	"fmt"
	"strings"
)

// Explain renders a trace as the decision table a fullsend maintainer reads: what the briefing holds, what was left
// out at which stage and why, and how each declared conflict was decided. It leaves out trace_id and timings, which
// differ run to run (R-23), so the text is stable enough to commit.
func Explain(trace map[string]any) string {
	var b strings.Builder
	p, ctx := field(trace, "profile"), field(trace, "context")
	fmt.Fprintf(&b, "profile %v v%v · snapshot %v\n", p["id"], p["version"], ctx["snapshot_digest"])
	if refused := field(trace, "refused"); refused["bool"] == true {
		fmt.Fprintf(&b, "REFUSED: %v", refused["reason"])
		if action, ok := field(trace, "recovery")["action"]; ok {
			fmt.Fprintf(&b, " -> recovery %v", action)
		}
		b.WriteString("\n")
	} else {
		budget, result := field(trace, "budget"), field(trace, "result")
		fmt.Fprintf(&b, "input tokens %v of budget %v (margin %v%%) · payload sha256 %v\n",
			result["input_tokens"], budget["input"], budget["margin_percent"], result["hash"])
	}

	b.WriteString("\nincluded\n")
	for _, row := range list(trace, "included") {
		fmt.Fprintf(&b, "  %-27s %-52v %5v tok\n", row["slot"], row["item_id"], row["tokens"])
	}
	for _, row := range list(trace, "compressed") {
		fmt.Fprintf(&b, "  compressed %v -> %v (%v -> %v tok)\n", row["item_id"], row["variant_id"], row["from"], row["to"])
	}

	b.WriteString("\nexcluded\n")
	for _, row := range list(trace, "excluded") {
		slot, ok := row["slot"]
		if !ok {
			slot = "-"
		}
		fmt.Fprintf(&b, "  %-9v %-22v %-48v %v", row["stage"], slot, row["item_id"], row["reason"])
		for _, key := range []string{"duplicate_of", "superseded_by"} {
			if kept, ok := row[key]; ok {
				fmt.Fprintf(&b, " (keeps %v)", kept)
			}
		}
		b.WriteString("\n")
	}

	if conflicts := list(trace, "conflicts"); len(conflicts) > 0 {
		b.WriteString("\nconflicts\n")
		for _, c := range conflicts {
			fmt.Fprintf(&b, "  %v [%v] %v by %v", c["group_id"], c["kind"], c["resolution"], c["decided_by"])
			if w, ok := c["winner"]; ok {
				fmt.Fprintf(&b, ", winner %v", w)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
