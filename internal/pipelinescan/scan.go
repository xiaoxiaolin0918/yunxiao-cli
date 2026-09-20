package pipelinescan

import (
	"errors"
	"fmt"

	"github.com/yunxiao-cli/yunxiao/internal/client"
)

// Report accumulates cross-pipeline scan outcomes for partial-success commands.
type Report struct {
	Scanned             int
	SkippedNoPermission []string
	Errors              []string
	seenSkip            map[string]struct{}
}

// IsNoPermission reports whether err is an HTTP 403 (pipeline permission denied).
func IsNoPermission(err error) bool {
	var ae *client.APIError
	if errors.As(err, &ae) && ae != nil && ae.Status == 403 {
		return true
	}
	return false
}

// NoteScannedPipeline increments the scanned pipeline counter.
func (r *Report) NoteScannedPipeline(pipelineID string) {
	if r == nil {
		return
	}
	r.Scanned++
}

// Record appends an error line; 403s also land in SkippedNoPermission (deduped by pipeline id).
func (r *Report) Record(pipelineID, op string, err error) {
	if r == nil || err == nil {
		return
	}
	msg := fmt.Sprintf("%s/%s: %v", pipelineID, op, err)
	r.Errors = append(r.Errors, msg)
	if !IsNoPermission(err) {
		return
	}
	if r.seenSkip == nil {
		r.seenSkip = map[string]struct{}{}
	}
	if _, ok := r.seenSkip[pipelineID]; ok {
		return
	}
	r.seenSkip[pipelineID] = struct{}{}
	r.SkippedNoPermission = append(r.SkippedNoPermission, pipelineID)
}

// Meta returns fields suitable for merging into output.Success meta.
func (r *Report) Meta() map[string]any {
	if r == nil {
		return map[string]any{}
	}
	m := map[string]any{
		"scanned":                       r.Scanned,
		"skipped_no_permission_count":   len(r.SkippedNoPermission),
		"error_count":                   len(r.Errors),
	}
	if len(r.SkippedNoPermission) > 0 {
		m["skipped_no_permission"] = r.SkippedNoPermission
	}
	if len(r.Errors) > 0 {
		m["errors"] = r.Errors
		// Alias used by +queue / field notes so callers can look in one place.
		m["run_list_errors"] = r.Errors
	}
	return m
}
