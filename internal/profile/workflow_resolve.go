package profile

import (
	"fmt"
	"strings"
)

// WorkflowResolve is the status graph used by workitem +transition.
type WorkflowResolve struct {
	TypeID   string
	Category string
	Name     string
	Edges       map[string][]string
	HintedEdges map[string][]string
	Statuses    map[string]string // alias/displayName → status id
	Source      string            // "workflows" | "bug_legacy"
}

// AllStatusIDs returns every known status id (statuses values + edge endpoints).
func (w WorkflowResolve) AllStatusIDs() map[string]bool {
	out := map[string]bool{}
	for _, id := range w.Statuses {
		if id != "" {
			out[id] = true
		}
	}
	for _, graph := range []map[string][]string{w.Edges, w.HintedEdges} {
		for from, tos := range graph {
			if from != "" {
				out[from] = true
			}
			for _, to := range tos {
				if to != "" {
					out[to] = true
				}
			}
		}
	}
	return out
}

// ResolveWorkflow loads edges/statuses for typeID from workflows[typeID].
// When missing, falls back to legacy bug_edges/bug_statuses if typeID matches bug_type_id.
func (p *Profile) ResolveWorkflow(typeID string) (WorkflowResolve, error) {
	typeID = strings.TrimSpace(typeID)
	if p == nil {
		return WorkflowResolve{}, fmt.Errorf("profile required")
	}
	if typeID == "" {
		return WorkflowResolve{}, fmt.Errorf("type_id required to resolve workflow")
	}

	if wf, ok := p.Workflows[typeID]; ok && len(wf.Edges) > 0 {
		statuses := wf.Statuses
		if statuses == nil {
			statuses = map[string]string{}
		}
		return WorkflowResolve{
			TypeID:      typeID,
			Category:    wf.Category,
			Name:        wf.Name,
			Edges:       wf.Edges,
			HintedEdges: wf.HintedEdges,
			Statuses:    statuses,
			Source:      "workflows",
		}, nil
	}

	if p.BugTypeID != "" && typeID == p.BugTypeID {
		edges := p.StatusGraph()
		if len(edges) == 0 {
			return WorkflowResolve{}, fmt.Errorf(
				"profile %q has no workflow edges for bug type %s; run: yunxiao workitem +explore-workflow --type-id %s --category Bug --write-profile --yes; for one-off status set use: yunxiao workitem update --id <id> --status <id> [--cancel-reason …]",
				p.Name, typeID, typeID)
		}
		statuses := p.BugStatuses
		if statuses == nil {
			statuses = map[string]string{}
		}
		return WorkflowResolve{
			TypeID:   typeID,
			Category: "Bug",
			Name:     "缺陷",
			Edges:    edges,
			Statuses: statuses,
			Source:   "bug_legacy",
		}, nil
	}

	return WorkflowResolve{}, fmt.Errorf(
		"profile %q missing workflows[%s] edges; run: yunxiao workitem +explore-workflow --type-id %s --write-profile --yes (sandbox first); for one-off status set use: yunxiao workitem update --id <id> --status <id> [--cancel-reason …]",
		p.Name, typeID, typeID)
}
