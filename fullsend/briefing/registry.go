package briefing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Registry holds the versioned route policies and placement profiles under registry/. A harness names one route
// policy version; the profile is the one written for it (R-20).
type Registry struct {
	routes   map[string]json.RawMessage
	profiles map[string]json.RawMessage
}

// LoadRegistry reads registry/route-policies.json and registry/profiles.json, keyed by route policy version.
func LoadRegistry(dir string) (*Registry, error) {
	r := &Registry{routes: map[string]json.RawMessage{}, profiles: map[string]json.RawMessage{}}
	if err := index(filepath.Join(dir, "route-policies.json"), "version", r.routes); err != nil {
		return nil, err
	}
	if err := index(filepath.Join(dir, "profiles.json"), "route_policy_version", r.profiles); err != nil {
		return nil, err
	}
	return r, nil
}

// Route returns the route policy at version and the profile placed for it.
func (r *Registry) Route(version string) (route, profile json.RawMessage, err error) {
	route, ok := r.routes[version]
	if !ok {
		return nil, nil, fmt.Errorf("no route policy %q in the registry", version)
	}
	profile, ok = r.profiles[version]
	if !ok {
		return nil, nil, fmt.Errorf("no profile for route policy %q in the registry", version)
	}
	return route, profile, nil
}

func index(path, key string, into map[string]json.RawMessage) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, entry := range entries {
		var head map[string]any
		if err := json.Unmarshal(entry, &head); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		version, _ := head[key].(string)
		if _, dup := into[version]; dup || version == "" {
			return fmt.Errorf("%s: missing or repeated %s %q", path, key, version)
		}
		into[version] = entry
	}
	return nil
}
