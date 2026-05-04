// Package browser loads an index.json produced by the indexer and exposes
// query helpers used by the HTTP server and CLI.
package browser

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/wesen/codebase-browser/internal/indexer"
)

// Loaded wraps the deserialised index with auxiliary lookup maps.
type Loaded struct {
	Raw         []byte
	Index       *indexer.Index
	byPackageID map[string]*indexer.Package
	byFileID    map[string]*indexer.File
	bySymbolID  map[string]*indexer.Symbol
}

// LoadFromFile loads index.json from disk.
func LoadFromFile(path string) (*Loaded, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	return LoadFromBytes(data)
}

// LoadFromFS loads index.json at name from an fs.FS (used with go:embed).
func LoadFromFS(fsys fs.FS, name string) (*Loaded, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	return LoadFromBytes(data)
}

// LoadFromBytes decodes an index from raw JSON bytes.
func LoadFromBytes(data []byte) (*Loaded, error) {
	var idx indexer.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	loaded := LoadIndex(&idx)
	loaded.Raw = data
	return loaded, nil
}

// LoadIndex wraps an in-memory index with the lookup maps used by browser
// helpers. Use this when the index was reconstructed from SQLite rather than
// decoded from index.json.
func LoadIndex(idx *indexer.Index) *Loaded {
	l := &Loaded{
		Index:       idx,
		byPackageID: make(map[string]*indexer.Package, len(idx.Packages)),
		byFileID:    make(map[string]*indexer.File, len(idx.Files)),
		bySymbolID:  make(map[string]*indexer.Symbol, len(idx.Symbols)),
	}
	for i := range idx.Packages {
		l.byPackageID[idx.Packages[i].ID] = &idx.Packages[i]
	}
	for i := range idx.Files {
		l.byFileID[idx.Files[i].ID] = &idx.Files[i]
	}
	for i := range idx.Symbols {
		l.bySymbolID[idx.Symbols[i].ID] = &idx.Symbols[i]
	}
	return l
}

func (l *Loaded) Package(id string) (*indexer.Package, bool) {
	p, ok := l.byPackageID[id]
	return p, ok
}
func (l *Loaded) File(id string) (*indexer.File, bool)     { f, ok := l.byFileID[id]; return f, ok }
func (l *Loaded) Symbol(id string) (*indexer.Symbol, bool) { s, ok := l.bySymbolID[id]; return s, ok }

// FindSymbols returns symbols matching a name substring and optional kind filter.
// If nameQuery is empty it matches everything.
func (l *Loaded) FindSymbols(nameQuery, kind string) []*indexer.Symbol {
	nameQuery = strings.ToLower(nameQuery)
	// Return an empty (non-nil) slice so the JSON endpoint renders [] not null.
	out := []*indexer.Symbol{}
	for i := range l.Index.Symbols {
		s := &l.Index.Symbols[i]
		if kind != "" && s.Kind != kind {
			continue
		}
		if nameQuery != "" && !strings.Contains(strings.ToLower(s.Name), nameQuery) {
			continue
		}
		out = append(out, s)
	}
	return out
}
