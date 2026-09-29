package mrpatchset

// MarkLatest adds a boolean "latest" to every patch set object in out (a decoded
// diffs/patches response, array or wrapped), true only for Latest(...). Items keep
// their existing fields and order; out is mutated in place. Returns the latest
// patch set and whether one exists (false for empty, typed-without-MERGE_SOURCE or
// non-list payloads). An existing "latest" key on an item is overwritten.
func MarkLatest(out any) (PatchSet, bool) {
	items, ok := itemsOf(out)
	if !ok {
		return PatchSet{}, false
	}
	sets, err := Parse(out)
	if err != nil {
		return PatchSet{}, false
	}
	ps, err := Latest(sets)
	found := err == nil
	for i, it := range items {
		if m, ok := it.(map[string]any); ok {
			m["latest"] = found && i == ps.Index
		}
	}
	if !found {
		return PatchSet{}, false
	}
	return ps, true
}
