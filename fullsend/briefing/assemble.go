package briefing

import (
	"encoding/json"
	"fmt"

	assembler "github.com/contextwindowarchitecture/assembler-go"
)

// Result is one assembly: the rendered payload, or nil when the assembler refused, and the trace that accounts for
// every decision either way (R-17, R-21).
type Result struct {
	Payload []byte
	Trace   map[string]any
}

// Refused reports whether the assembler refused, and why. A refused run never reaches a runtime.
func (r Result) Refused() (bool, string) {
	refused, _ := r.Trace["refused"].(map[string]any)
	reason, _ := refused["reason"].(string)
	return r.Payload == nil, reason
}

// Assemble hands the frozen snapshot to assembler-go. Admission, conflict resolution, fitting and rendering are the
// assembler's; this package only produces and freezes (R-18, R-23).
func Assemble(s Snapshot) (Result, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return Result{}, err
	}
	out, err := assembler.Assemble(raw, assembler.Options{})
	if err != nil {
		return Result{}, fmt.Errorf("assembling: %w", err)
	}
	return Result{Payload: out.Payload, Trace: out.Trace}, nil
}
