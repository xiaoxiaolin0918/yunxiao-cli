// Package mrpatchset parses Codeup MR patch sets (GET .../changeRequests/{localId}/diffs/patches)
// and picks the latest one. The API returns an unordered array without a "latest"
// marker (#94); comments create defaults GLOBAL_COMMENT to the latest source patchset (#93).
package mrpatchset

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// MergeSource / MergeTarget are relatedMergeItemType values.
const (
	MergeSource = "MERGE_SOURCE"
	MergeTarget = "MERGE_TARGET"
)

var (
	// ErrNoPatchSets means the MR has no usable patch set (empty list / no biz ids).
	ErrNoPatchSets = errors.New("MR has no patchsets")
	// ErrNoSourcePatchSet means the payload has typed entries (MERGE_TARGET or any other
	// relatedMergeItemType) but no MERGE_SOURCE patch set.
	ErrNoSourcePatchSet = errors.New("MR has no MERGE_SOURCE patchset")
)

// PatchSet is the subset of a patch set item the CLI reasons about.
type PatchSet struct {
	BizID      string    // patchSetBizId
	Name       string    // patchSetName
	VersionNo  int64     // versionNo (legacy: patchSetNo); 0 when absent / not an integer
	Type       string    // relatedMergeItemType (MERGE_SOURCE|MERGE_TARGET|"")
	CommitID   string    // commitId
	CreateTime string    // createTime (legacy: createdAt); numeric epoch values kept as digits
	Created    time.Time // parsed CreateTime (UTC for zoneless / epoch values); zero when unparseable
	Index      int       // position in the API response (lets callers mark the raw item)
}

// Parse extracts patch sets from a decoded JSON response: a top-level array, or an
// object wrapping it under result / data / items / patchSets.
func Parse(out any) ([]PatchSet, error) {
	list, ok := out.([]any)
	if !ok {
		m, isMap := out.(map[string]any)
		if !isMap {
			return nil, fmt.Errorf("unexpected patchsets payload %T (want array)", out)
		}
		for _, k := range []string{"result", "data", "items", "patchSets"} {
			if l, ok := m[k].([]any); ok {
				list = l
				break
			}
		}
		if list == nil {
			return nil, fmt.Errorf("unexpected patchsets payload: object without result/data/items/patchSets array")
		}
	}
	sets := make([]PatchSet, 0, len(list))
	for i, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		ps := PatchSet{
			BizID:      str(m["patchSetBizId"]),
			Name:       str(m["patchSetName"]),
			Type:       strings.ToUpper(str(m["relatedMergeItemType"])),
			CommitID:   str(m["commitId"]),
			CreateTime: firstNonEmpty(timeStr(m["createTime"]), timeStr(m["createdAt"])),
			Index:      i,
		}
		ps.VersionNo = num(m["versionNo"])
		if ps.VersionNo == 0 {
			ps.VersionNo = num(m["patchSetNo"])
		}
		ps.Created = parseTime(ps.CreateTime)
		sets = append(sets, ps)
	}
	return sets, nil
}

// Latest returns the newest MERGE_SOURCE patch set by the total order
// (versionNo, has-parseable-createTime, createTime, response position): highest version
// wins; among equal versions an item with a parseable createTime beats one without, then
// the later createTime, then the later position. MERGE_TARGET items are never returned.
// Items without a relatedMergeItemType are candidates only when there is no MERGE_SOURCE
// candidate and no entry has a MERGE_TARGET or unknown relatedMergeItemType (such a value
// disables the fallback, even on an entry without patchSetBizId). With such a value and no
// MERGE_SOURCE candidate, Latest returns ErrNoSourcePatchSet. Entries without
// patchSetBizId are never returned.
func Latest(sets []PatchSet) (PatchSet, error) {
	var source, untyped []PatchSet
	sawTyped := false // any relatedMergeItemType other than MERGE_SOURCE (MERGE_TARGET or unknown)
	for _, ps := range sets {
		if ps.Type != "" && ps.Type != MergeSource {
			sawTyped = true // checked before the BizID filter: the payload is typed either way
		}
		if ps.BizID == "" {
			continue
		}
		switch ps.Type {
		case MergeSource:
			source = append(source, ps)
		case "":
			untyped = append(untyped, ps)
		}
	}
	cands := source
	if len(cands) == 0 && !sawTyped {
		cands = untyped
	}
	if len(cands) == 0 {
		if sawTyped {
			return PatchSet{}, ErrNoSourcePatchSet
		}
		return PatchSet{}, ErrNoPatchSets
	}
	best := cands[0]
	for _, ps := range cands[1:] {
		if newer(ps, best) {
			best = ps
		}
	}
	return best, nil
}

// newer reports whether a sorts after b in the lexicographic key
// (VersionNo, !Created.IsZero(), Created, Index). Being a strict total order, the
// pick does not depend on response order except for exact ties (then later wins).
func newer(a, b PatchSet) bool {
	if a.VersionNo != b.VersionNo {
		return a.VersionNo > b.VersionNo
	}
	aHas, bHas := !a.Created.IsZero(), !b.Created.IsZero()
	if aHas != bHas {
		return aHas
	}
	if aHas && !a.Created.Equal(b.Created) {
		return a.Created.After(b.Created)
	}
	return a.Index > b.Index
}

func str(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case nil:
		return ""
	case json.Number:
		return t.String()
	case float64:
		// Avoid fmt's %v scientific notation for large integral values (1.7e+12).
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

// timeStr is str for createTime: numeric epoch values stay plain digits.
func timeStr(v any) string {
	if f, ok := v.(float64); ok && f == math.Trunc(f) {
		return strconv.FormatInt(int64(f), 10)
	}
	return str(v)
}

// num parses an integral version. Integral floats ("3.0", 3.0) are accepted;
// fractional or non-numeric values yield 0 (treated as "no version").
func num(v any) int64 {
	switch t := v.(type) {
	case float64:
		return integral(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		return numString(t.String())
	case string:
		return numString(t)
	}
	return 0
}

func numString(s string) int64 {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return integral(f)
	}
	return 0
}

func integral(f float64) int64 {
	if f != math.Trunc(f) || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0
	}
	return int64(f)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Zoned layouts first; zoneless layouts are interpreted as UTC (not the host's zone).
var timeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999"}

// allDigits reports whether s is non-empty and only ASCII digits (no sign, no spaces).
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseTime accepts RFC3339, zoneless "YYYY-MM-DD[T ]hh:mm:ss[.frac]" (UTC) and
// all-digit epoch values of at least 10 digits (>= 1e11 → milliseconds, otherwise
// seconds). Shorter numerics (e.g. date-like "20260929") are not treated as epoch.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if allDigits(s) {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n < 1e9 { // overflow, or fewer than 10 significant digits: not an epoch
			return time.Time{}
		}
		if n >= 1e11 {
			return time.UnixMilli(n).UTC()
		}
		return time.Unix(n, 0).UTC()
	}
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.UTC); err == nil {
			return t
		}
	}
	return time.Time{}
}
