package ingest

import (
	"fmt"
	"slices"
	"strings"
)

// A Registry holds the adapters, one per file type. It is filled at start-up and read-only
// afterwards; every listing is in sorted file-type order (§A6, NF-03).
type Registry struct {
	byType map[string]Adapter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{byType: map[string]Adapter{}} }

// Register adds a. A second adapter for the same file type is refused: which one would
// read a file must never depend on start-up order.
func (r *Registry) Register(a Adapter) error {
	if a == nil {
		return fmt.Errorf("ingest: cannot register a nil adapter")
	}
	ft := a.FileType()
	if ft == "" || strings.TrimSpace(ft) != ft {
		return fmt.Errorf("ingest: adapter file type %q is empty or has surrounding spaces", ft)
	}
	if _, dup := r.byType[ft]; dup {
		return fmt.Errorf("ingest: an adapter for file type %q is already registered", ft)
	}
	r.byType[ft] = a
	return nil
}

// Lookup returns the adapter for fileType.
func (r *Registry) Lookup(fileType string) (Adapter, bool) {
	a, ok := r.byType[fileType]
	return a, ok
}

// FileTypes returns every registered file type, sorted.
func (r *Registry) FileTypes() []string {
	types := make([]string, 0, len(r.byType))
	for ft := range r.byType {
		types = append(types, ft)
	}
	slices.Sort(types)
	return types
}

// Adapters returns every registered adapter, sorted by file type.
func (r *Registry) Adapters() []Adapter {
	types := r.FileTypes()
	out := make([]Adapter, len(types))
	for i, ft := range types {
		out[i] = r.byType[ft]
	}
	return out
}

// ForFile returns the one adapter whose Sniff accepts the file. No match is
// UNSUPPORTED_FILE_TYPE; more than one is also refused, naming them, because the tool
// never guesses which reader to use.
func (r *Registry) ForFile(head []byte, name string) (Adapter, error) {
	var hits []Adapter
	for _, a := range r.Adapters() {
		if a.Sniff(head, name) {
			hits = append(hits, a)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return nil, &Error{Code: UnsupportedFileType, File: name,
			Msg: fmt.Sprintf("the file is not a type the tool can read (it reads: %s)", strings.Join(r.FileTypes(), ", "))}
	}
	types := make([]string, len(hits))
	for i, a := range hits {
		types[i] = a.FileType()
	}
	return nil, &Error{Code: UnsupportedFileType, File: name,
		Msg: fmt.Sprintf("the file looks like more than one file type (%s); the tool does not guess", strings.Join(types, ", "))}
}
