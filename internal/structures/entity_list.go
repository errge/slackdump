package structures

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	// excludePrefix is the prefix that is used to mark channel exclusions, i.e.
	// for export or when downloading conversations.
	excludePrefix = "^"
	filePrefix    = "@"

	// maxFileEntries is the maximum non-empty entries that will be read from
	// the file. Who ever needs more than 64Ki channels.
	maxFileEntries = 65536
)

// EntityList is an Inclusion/Exclusion list
type EntityList struct {
	Include []string
	Exclude []string
}

func HasExcludePrefix(s string) bool {
	return strings.HasPrefix(s, excludePrefix)
}

func hasFilePrefix(s string) bool {
	return strings.HasPrefix(s, filePrefix)
}

// NewEntityList creates an EntityList from a slice of IDs or URLs (entites).
func NewEntityList(entities []string) (*EntityList, error) {
	var el EntityList

	index, err := buildEntryIndex(entities)
	if err != nil {
		return nil, err
	}
	el.fromIndex(index)

	return &el, nil
}

// LoadEntityList creates an EntityList from a slice of IDs or URLs (entites).
func LoadEntityList(filename string) (*EntityList, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readEntityList(f, maxFileEntries)
}

// readEntityList is a rather naïve implementation that reads the entire file up
// to maxEntries entities (empty lines are skipped), and populates the slice of
// strings, which is then passed to NewEntityList.  On large lists it will
// probably use a silly amount of memory.
func readEntityList(r io.Reader, maxEntries int) (*EntityList, error) {
	br := bufio.NewReader(r)
	var elements []string
	total := 0
	var exit bool
	for n := 1; ; n++ {
		if total >= maxEntries {
			return nil, errors.New("maximum file size exceeded")
		}
		line, err := br.ReadString('\n')
		if errors.Is(err, io.EOF) {
			exit = true
		} else if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			if exit {
				break
			}
			continue
		}
		// test if it's a valid line
		elements = append(elements, line)
		if exit {
			break
		}

		total++
	}
	return NewEntityList(elements)
}

func (el *EntityList) fromIndex(index map[string]bool) {
	for ent, include := range index {
		if include {
			el.Include = append(el.Include, ent)
		} else {
			el.Exclude = append(el.Exclude, ent)
		}
	}
	sort.Strings(el.Include)
	sort.Strings(el.Exclude)
}

// Index returns a map where key is entity, and value show if the entity
// should be processed (true) or not (false).
func (el *EntityList) Index() EntityIndex {
	if el == nil {
		return map[string]bool{}
	}
	idx := make(map[string]bool, len(el.Include)+len(el.Exclude))
	for _, v := range el.Include {
		idx[v] = true
	}
	for _, v := range el.Exclude {
		idx[v] = false
	}
	return idx
}

type EntityIndex map[string]bool

// IsExcluded returns true if the entity is excluded (is in the list, and has
// value false).
func (ei EntityIndex) IsExcluded(ent string) bool {
	v, ok := ei[ent]
	return ok && !v
}

// IsIncluded returns true if the entity is included (is in the list, and has
// value true).
func (ei EntityIndex) IsIncluded(ent string) bool {
	v, ok := ei[ent]
	return ok && v
}

// HasIncludes returns true if there's any included entities.
func (el *EntityList) HasIncludes() bool {
	return len(el.Include) > 0
}

// HasExcludes returns true if there's any excluded entities.
func (el *EntityList) HasExcludes() bool {
	return len(el.Exclude) > 0
}

// IsEmpty returns true if there's no entries in the list.
func (el *EntityList) IsEmpty() bool {
	return len(el.Include)+len(el.Exclude) == 0
}

func buildEntryIndex(links []string) (map[string]bool, error) {
	index := make(map[string]bool, len(links))
	var excluded []string
	var files []string
	// add all included items
	for _, ent := range links {
		if ent == "" {
			continue
		}
		switch {
		case HasExcludePrefix(ent):
			trimmed := strings.TrimPrefix(ent, excludePrefix)
			if trimmed == "" {
				continue
			}
			sl, err := ParseLink(trimmed)
			if err != nil {
				return nil, err
			}
			excluded = append(excluded, sl.String())
		case hasFilePrefix(ent):
			trimmed := strings.TrimPrefix(ent, filePrefix)
			if trimmed == "" {
				continue
			}
			files = append(files, trimmed)
		default:
			// no prefix
			sl, err := ParseLink(ent)
			if err != nil {
				return nil, err
			}
			index[sl.String()] = true
		}
	}
	// process files
	for _, file := range files {
		el, err := LoadEntityList(file)
		if err != nil {
			return nil, err
		}
		for ent, include := range el.Index() {
			if include {
				index[ent] = true
			} else {
				excluded = append(excluded, ent)
			}
		}
	}
	for _, ent := range excluded {
		index[ent] = false
	}
	return index, nil
}

// Generator returns a channel where all included entries are streamed.
// The channel is closed when all entries have been sent, or when the context
// is cancelled.
func (el *EntityList) Generator(ctx context.Context) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for _, ent := range el.Include {
			select {
			case <-ctx.Done():
				return
			case ch <- ent:
			}
		}
	}()
	return ch
}
