package workflow

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Attempt records one PUT status probe.
type Attempt struct {
	From    string `json:"from"`
	To      string `json:"to"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Kind    string `json:"kind,omitempty"` // probe | reposition
	Outcome string `json:"outcome,omitempty"`
}

// PutStatusFunc attempts PUT {"status": to} on the probe item.
// Returns nil on HTTP success.
type PutStatusFunc func(to string) error

// GetStatusFunc returns the item's current status id.
type GetStatusFunc func() (string, error)

// ResetFunc recreates or resets the probe item when stranded.
// Returns the new current status id (typically defaultStatus).
type ResetFunc func() (string, error)

// ProbeOptions configures ExploreTransitions.
type ProbeOptions struct {
	StatusIDs     []string
	StartStatus   string
	DefaultStatus string // optional fallback when stranded
	Put           PutStatusFunc
	Get           GetStatusFunc
	Reset         ResetFunc             // optional; called when ensureAt fails for a pending from
	Sleep         func(d time.Duration) // injectable; default time.Sleep
	RateLimitWait time.Duration         // default 1s on 429-ish errors
}

// ProbeResult is the discovered graph plus diagnostics.
//
// Edges are verified only (HTTP OK on PUT status). OutcomeNeedsFields go to
// HintedEdges + RequiredHints / MissingFields — never into Edges — so consumers
// and BFS reposition do not treat false-positive required-field edges as real
// transitions (issue 61).
type ProbeResult struct {
	Edges         map[string][]string // verified (OutcomeOK)
	HintedEdges   map[string][]string // needs_fields only
	RequiredHints map[string]string   // keyed by "from→to" and optionally "to"
	MissingFields map[string][]string // keyed by "from→to" → field display names
	Attempts      []Attempt
	HardToReach   []string
	FinalStatus   string
}

// ExploreTransitions probes all ordered pairs (from,to) with from≠to.
// Denied / needs_fields keep the item at from so remaining outbound probes continue.
// Successful transitions move the item; we then best-effort return (or reset to DefaultStatus).
// Sources we cannot reach are listed in HardToReach; their remaining pairs are skipped.
func ExploreTransitions(opt ProbeOptions) (*ProbeResult, error) {
	if opt.Put == nil {
		return nil, fmt.Errorf("Put callback required")
	}
	sleep := opt.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	rateWait := opt.RateLimitWait
	if rateWait <= 0 {
		rateWait = time.Second
	}

	ids := uniqueStrings(opt.StatusIDs)
	sort.Strings(ids)
	fromOrder := preferFirst(ids, opt.StartStatus)

	edges := map[string][]string{}
	hinted := map[string][]string{}
	hints := map[string]string{}
	missing := map[string][]string{}
	var attempts []Attempt
	attempted := map[string]bool{}
	hard := map[string]bool{}

	current := opt.StartStatus
	if opt.Get != nil {
		if cur, err := opt.Get(); err == nil && cur != "" {
			current = cur
		}
	}

	putOnce := func(from, to, kind string) Outcome {
		err := opt.Put(to)
		if err != nil && isRateLimited(err.Error()) {
			sleep(rateWait)
			err = opt.Put(to)
		}
		if err == nil {
			attempts = append(attempts, Attempt{From: from, To: to, OK: true, Kind: kind, Outcome: OutcomeOK.String()})
			AddEdge(edges, from, to)
			current = to
			return OutcomeOK
		}
		msg := err.Error()
		snippet := ExtractAPIErrorBody(msg)
		out := ClassifyTransitionError(msg)
		attempts = append(attempts, Attempt{
			From: from, To: to, OK: false, Error: snippet, Kind: kind, Outcome: out.String(),
		})
		if out == OutcomeNeedsFields {
			// Hint only — do NOT AddEdge to verified graph (issue 61).
			AddEdge(hinted, from, to)
			pairKey := from + "→" + to
			if _, ok := hints[pairKey]; !ok {
				hints[pairKey] = snippet
			}
			if _, ok := hints[to]; !ok {
				hints[to] = snippet
			}
			if fields := ExtractMissingFieldNames(snippet); len(fields) > 0 {
				if _, ok := missing[pairKey]; !ok {
					missing[pairKey] = fields
				}
			}
		}
		return out
	}

	refresh := func() {
		if opt.Get == nil {
			return
		}
		if cur, err := opt.Get(); err == nil && cur != "" {
			current = cur
		}
	}

	ensureAt := func(from string) bool {
		if current == from {
			return true
		}
		if path := BFSPath(current, from, edges); len(path) > 0 {
			at := current
			for _, step := range path {
				if putOnce(at, step, "reposition") != OutcomeOK {
					break
				}
				at = step
			}
			if current == from {
				return true
			}
		}
		if current != from {
			if putOnce(current, from, "reposition") == OutcomeOK {
				return true
			}
		}
		// Fallback: jump to default then retry once.
		if opt.DefaultStatus != "" && current != opt.DefaultStatus {
			_ = putOnce(current, opt.DefaultStatus, "reposition")
			if current == from {
				return true
			}
			if current != from {
				if path := BFSPath(current, from, edges); len(path) > 0 {
					at := current
					for _, step := range path {
						if putOnce(at, step, "reposition") != OutcomeOK {
							break
						}
						at = step
					}
				} else if current != from {
					_ = putOnce(current, from, "reposition")
				}
			}
		}
		refresh()
		return current == from
	}

	resetUsed := map[string]bool{}
	tryEnsureAt := func(from string) bool {
		if ensureAt(from) {
			return true
		}
		if opt.Reset != nil && !resetUsed[from] {
			resetUsed[from] = true
			st, err := opt.Reset()
			if err == nil && st != "" {
				current = st
			}
			refresh()
			if ensureAt(from) {
				delete(hard, from)
				return true
			}
		}
		return false
	}

	pairKey := func(from, to string) string { return from + "\x00" + to }

	hasPending := func(from string) bool {
		for _, to := range ids {
			if to == from {
				continue
			}
			if !attempted[pairKey(from, to)] {
				return true
			}
		}
		return false
	}

	maxPasses := len(ids) + 2
	if maxPasses < 3 {
		maxPasses = 3
	}
	for pass := 0; pass < maxPasses; pass++ {
		progress := false
		for _, from := range fromOrder {
			if hard[from] || !hasPending(from) {
				continue
			}
			if !tryEnsureAt(from) {
				hard[from] = true
				continue
			}
			for _, to := range ids {
				if to == from {
					continue
				}
				key := pairKey(from, to)
				if attempted[key] {
					continue
				}
				if current != from {
					if !tryEnsureAt(from) {
						hard[from] = true
						break
					}
				}
				attempted[key] = true
				progress = true
				o := putOnce(from, to, "probe")
				if o == OutcomeOK {
					// Prefer returning to from so remaining outbound probes can continue.
					if current != from {
						if putOnce(current, from, "reposition") != OutcomeOK {
							if opt.DefaultStatus != "" && current != opt.DefaultStatus {
								_ = putOnce(current, opt.DefaultStatus, "reposition")
							}
							if current != from {
								_ = tryEnsureAt(from)
							}
						}
					}
				}
				// Denied / needs_fields: stay at from (approx) and continue.
			}
		}
		if !progress {
			break
		}
	}

	// Any from with unattempted pairs is hard-to-reach.
	for _, from := range ids {
		if hasPending(from) {
			hard[from] = true
		}
	}
	hardList := make([]string, 0, len(hard))
	for id := range hard {
		hardList = append(hardList, id)
	}
	sort.Strings(hardList)

	return &ProbeResult{
		Edges:         SortedCopy(edges),
		HintedEdges:   SortedCopy(hinted),
		RequiredHints: hints,
		MissingFields: missing,
		Attempts:      attempts,
		HardToReach:   hardList,
		FinalStatus:   current,
	}, nil
}

func hasEdge(edges map[string][]string, from, to string) bool {
	for _, x := range edges[from] {
		if x == to {
			return true
		}
	}
	return false
}

func preferFirst(ids []string, first string) []string {
	if first == "" {
		return append([]string{}, ids...)
	}
	out := make([]string, 0, len(ids))
	out = append(out, first)
	for _, id := range ids {
		if id == first {
			continue
		}
		out = append(out, id)
	}
	return out
}

func isRateLimited(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "429") ||
		strings.Contains(m, "rate limit") ||
		strings.Contains(m, "too many requests") ||
		strings.Contains(msg, "请求过于频繁")
}
