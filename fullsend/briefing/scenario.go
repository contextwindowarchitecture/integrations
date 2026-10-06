package briefing

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Scenario is the input of one committed scenario, scenarios/<name>/scenario.json: which agent runs, on which
// forge read, under which route policy version, with how many input tokens of briefing.
type Scenario struct {
	Description string `json:"description"`
	Agent       string `json:"agent"`
	Fixture     string `json:"fixture"`
	RoutePolicy string `json:"route_policy"`
	BudgetInput int    `json:"budget_input"`
}

// Integration is this folder: its registry, fixtures and scenarios.
type Integration struct {
	Root     string
	registry *Registry
}

// Open loads the integration rooted at dir.
func Open(dir string) (*Integration, error) {
	reg, err := LoadRegistry(filepath.Join(dir, "registry"))
	if err != nil {
		return nil, err
	}
	return &Integration{Root: dir, registry: reg}, nil
}

// Scenarios lists the committed scenario names, sorted.
func (in *Integration) Scenarios() ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(in.Root, "scenarios", "*", "scenario.json"))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(filepath.Dir(p))
	}
	sort.Strings(names)
	return names, nil
}

// Load reads a scenario's input.
func (in *Integration) Load(name string) (Scenario, error) {
	var sc Scenario
	err := readJSON(filepath.Join(in.Root, "scenarios", name, "scenario.json"), &sc)
	return sc, err
}

// Snapshot freezes the scenario's snapshot from its fixture and the registry.
func (in *Integration) Snapshot(sc Scenario) (Snapshot, error) {
	route, profile, err := in.registry.Route(sc.RoutePolicy)
	if err != nil {
		return Snapshot{}, err
	}
	fixture := filepath.Join(in.Root, "fixtures", sc.Fixture+".json")
	switch sc.Agent {
	case "triage":
		var f IssueRead
		if err := readJSON(fixture, &f); err != nil {
			return Snapshot{}, err
		}
		return Triage(f, route, profile, sc.BudgetInput), nil
	case "review-security":
		var f PullRead
		if err := readJSON(fixture, &f); err != nil {
			return Snapshot{}, err
		}
		return ReviewSecurity(f, route, profile, sc.BudgetInput), nil
	case "fix":
		var f FixRead
		if err := readJSON(fixture, &f); err != nil {
			return Snapshot{}, err
		}
		return Fix(f, route, profile, sc.BudgetInput), nil
	}
	return Snapshot{}, fmt.Errorf("no producers for agent %q", sc.Agent)
}

// Run freezes and assembles one scenario.
func (in *Integration) Run(name string) (Snapshot, Result, error) {
	sc, err := in.Load(name)
	if err != nil {
		return Snapshot{}, Result{}, err
	}
	snap, err := in.Snapshot(sc)
	if err != nil {
		return Snapshot{}, Result{}, err
	}
	res, err := Assemble(snap)
	return snap, res, err
}

// Files is every generated file of a scenario as it should be committed; a nil value is a file that must not exist.
// before.md and briefing.md are for people; snapshot.json, trace.json and payload.json are the machine record. A
// refused scenario has no payload.json. The trace is stored without trace_id and timings, which may differ run to
// run (R-23).
func (in *Integration) Files(name string) (map[string][]byte, error) {
	snap, res, err := in.Run(name)
	if err != nil {
		return nil, err
	}
	trace := make(map[string]any, len(res.Trace))
	for k, v := range res.Trace {
		if k != "trace_id" && k != "timings" {
			trace[k] = v
		}
	}
	before, err := in.Before(name)
	if err != nil {
		return nil, err
	}
	page, err := BriefingPage(name, snap, res)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{"payload.json": res.Payload, "before.md": []byte(before), "briefing.md": []byte(page)}
	if files["snapshot.json"], err = indent(snap); err != nil {
		return nil, err
	}
	if files["trace.json"], err = indent(trace); err != nil {
		return nil, err
	}
	return files, nil
}

// indent writes v as committed JSON: two-space indent, a final newline, and <, > and & left as written so bodies
// read as the model receives them.
func indent(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func readJSON(path string, into any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
