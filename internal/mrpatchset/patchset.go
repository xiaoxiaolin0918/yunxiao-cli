// Package mrpatchset parses Codeup MR patch sets (GET .../changeRequests/{localId}/diffs/patches)
// and picks the latest one. The API returns an unordered array without a "latest"
// marker (#94); comments create defaults GLOBAL_COMMENT to the latest source patchset (#93).
package mrpatchset

import (
	"errors"
	"fmt"
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
	// ErrNoSourcePatchSet means only MERGE_TARGET patch sets exist.
	ErrNoSourcePatchSet = errors.New("MR has no MERGE_SOURCE patchset")
)

// PatchSet is the subset of a patch set item the CLI reasons about.
type PatchSet struct {
	BizID      string    // patchSetBizId
	Name       string    // patchSetName
	VersionNo  int64     // versionNo (legacy: patchSetNo); 0 when absent
	Type       string    // relatedMergeItemType (MERGE_SOURCE|MERGE_TARGET|"")
	CommitID   string    // commitId
	CreateTime string    // raw createTime (legacy: createdAt)
	Created    time.Time // parsed CreateTime; zero when unparseable
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
			return nil, fmt.Errorf("unexpected patchsets payload: object without result/data/items array")
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
			CreateTime: firstNonEmpty(str(m["createTime"]), str(m["createdAt"])),
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

// Latest returns the newest MERGE_SOURCE patch set: highest versionNo, then latest
// createTime, then later position. Items without a relatedMergeItemType are used only
// when no item is typed MERGE_SOURCE; MERGE_TARGET items are never returned.
func Latest(sets []PatchSet) (PatchSet, error) {
	var source, untyped []PatchSet
	sawTarget := false
	for _, ps := range sets {
		if ps.BizID == "" {
			continue
		}
		switch ps.Type {
		case MergeSource:
			source = append(source, ps)
		case "":
			untyped = append(untyped, ps)
		default:
			sawTarget = true
		}
	}
	cands := source
	if len(cands) == 0 {
		cands = untyped
	}
	if len(cands) == 0 {
		if sawTarget {
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

// newer reports whether a should be preferred over b.
func newer(a, b PatchSet) bool {
	if a.VersionNo != b.VersionNo {
		return a.VersionNo > b.VersionNo
	}
	if !a.Created.IsZero() && !b.Created.IsZero() && !a.Created.Equal(b.Created) {
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
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func num(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

var timeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05"}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}
