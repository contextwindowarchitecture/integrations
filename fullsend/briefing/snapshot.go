// Package briefing assembles a fullsend agent's briefing, the context its runtime starts from, with a Context Window
// Architecture (CWA) assembler. It is a sketch of the internal/cwa package ADR 0133 adds to `fullsend run`: host-side
// producers read the forge with read-level credentials after the harness pre-script, the run freezes a snapshot, and
// assembler-go turns it into a payload and a trace before the sandbox exists. Nothing here calls a model.
package briefing

import "encoding/json"

const (
	// Tokenizer is the published estimate CWA defines for models without a local tokenizer; the 20% margin each
	// snapshot sets covers its error (R-16).
	Tokenizer = "estimate-utf8/v1"
	// Renderer writes a message request: system entries, one user message, and tools (R-7).
	Renderer = "cwa-messages/v1"
	// Window is the model's context limit. A snapshot's budget.input is the briefing's share, and reserved_output
	// keeps the rest for what the runtime adds: its own system prompt, tool schemas, every later turn of its agent
	// loop and the output.
	Window = 200_000
	// MarginPercent reserves room for the estimating tokenizer's error (R-16).
	MarginPercent = 20
)

// Snapshot is everything that can affect an assembly, frozen before it begins (R-23). Field order follows the
// specification's snapshot schema so committed snapshots read top to bottom.
type Snapshot struct {
	AssemblyTime string          `json:"assembly_time"`
	Scope        Scope           `json:"scope"`
	Budget       Budget          `json:"budget"`
	Profile      json.RawMessage `json:"profile"`
	RoutePolicy  json.RawMessage `json:"route_policy"`
	Tokenizer    string          `json:"tokenizer"`
	Renderer     string          `json:"renderer"`
	Batches      []Batch         `json:"batches"`
	Conflicts    []ConflictGroup `json:"conflicts"`
}

// Scope keys an item or request. An item key the request lacks never matches, and a missing key is never a
// wildcard (R-2).
type Scope struct {
	Tenant  string `json:"tenant,omitempty"`
	User    string `json:"user,omitempty"`
	Session string `json:"session,omitempty"`
	Task    string `json:"task,omitempty"`
	Step    string `json:"step,omitempty"`
}

// Budget is the briefing's share of the window (R-16).
type Budget struct {
	Input          int `json:"input"`
	ReservedOutput int `json:"reserved_output"`
	MarginPercent  int `json:"margin_percent"`
}

// Batch is one producer's output. The producer's identity is the runner component that read the source, bound by
// the run, never read from an item (R-15).
type Batch struct {
	Producer Producer    `json:"producer"`
	Items    []Item      `json:"items"`
	Excluded []Exclusion `json:"excluded"`
}

// Producer is the authenticated identity of a batch and the kind the route lists it with.
type Producer struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// Exclusion is an entry a producer suppressed and reports instead of emitting (R-9, R-14).
type Exclusion struct {
	ItemID string `json:"item_id"`
	Reason string `json:"reason"`
	Stage  string `json:"stage"`
}

// ConflictGroup is a contradiction the run declares; the assembler never finds one by reading prose (R-11).
type ConflictGroup struct {
	ID    string   `json:"id"`
	Kind  string   `json:"kind"`
	Fact  string   `json:"fact,omitempty"`
	Items []string `json:"items"`
}

// Item is one candidate for one slot. The eight required fields come first (R-2); the rest are policy fields the
// assembler fills from slot defaults when they are omitted, and traces when it does (R-3).
type Item struct {
	ID            string    `json:"id"`
	Slot          string    `json:"slot"`
	Source        string    `json:"source"`
	SourceVersion string    `json:"source_version"`
	Authority     string    `json:"authority"`
	Trust         string    `json:"trust"`
	Freshness     string    `json:"freshness"`
	Expires       string    `json:"expires,omitempty"`
	Scope         *Scope    `json:"scope,omitempty"`
	Relevance     *float64  `json:"relevance,omitempty"`
	Tier          string    `json:"tier,omitempty"`
	Lineage       string    `json:"lineage,omitempty"`
	InjectionRisk string    `json:"injection_risk,omitempty"`
	Eligibility   string    `json:"eligibility,omitempty"`
	Variants      []Variant `json:"variants,omitempty"`
	Body          string    `json:"body"`
}

// Variant is a shorter body the producer computed before assembly; the assembler may select it but never writes
// one (R-18).
type Variant struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Method  string `json:"method"`
	Lineage string `json:"lineage"`
}

// newItem fills the required fields. Governing and state items are verified by the source that produced them;
// everything else is the producer's unverified claim, which only the governance gate reads (R-10, R-15).
func newItem(id, slot, source, sourceVersion, authority, freshness, body string) Item {
	trust := "unverified"
	if authority == "governing" || authority == "state" {
		trust = "verified"
	}
	return Item{ID: id, Slot: slot, Source: source, SourceVersion: sourceVersion, Authority: authority,
		Trust: trust, Freshness: freshness, Body: body}
}

func batch(producerID, kind string, items []Item, excluded ...Exclusion) Batch {
	if items == nil {
		items = []Item{}
	}
	if excluded == nil {
		excluded = []Exclusion{}
	}
	return Batch{Producer: Producer{ID: producerID, Kind: kind}, Items: items, Excluded: excluded}
}

func score(v float64) *float64 { return &v }
