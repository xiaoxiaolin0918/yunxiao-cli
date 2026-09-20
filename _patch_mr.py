from pathlib import Path

# --- patch codeup.go create body ---
p = Path("cmd/codeup.go")
t = p.read_text(encoding="utf-8")
t2 = t.replace(
    'if len(workItemIDs) > 0 {\n\t\t\tbody["workItemIds"] = workItemIDs\n\t\t}',
    'if csv := mrlink.WorkItemIDsCSV(workItemIDs); csv != "" {\n\t\t\tbody["workItemIds"] = csv\n\t\t}',
    1,
)
# after create success path - replace warn with ensure
old_tail = '''\t\tmeta := map[string]any{"risk": risk.HighRiskWrite}
\t\tmrMap := asStringMap(out)
\t\tzhiyi.EnrichMergeRequestMeta(meta, mrMap)
\t\twarnMRWorkItemLinks(meta, resolvedWorkItems, mrMap)
\t\thandleErr(output.Success(out, meta))
\t},
}'''
new_tail = '''\t\tmeta := map[string]any{"risk": risk.HighRiskWrite}
\t\tmrMap := asStringMap(out)
\t\tzhiyi.EnrichMergeRequestMeta(meta, mrMap)
\t\tif err := ensureMRWorkItemLinks(cmd.Context(), c, repositoryID, resolvedWorkItems, mrMap, meta); err != nil {
\t\t\thandleErr(err)
\t\t\treturn
\t\t}
\t\thandleErr(output.Success(out, meta))
\t},
}'''
if old_tail not in t2:
    raise SystemExit("create success tail missing")
t2 = t2.replace(old_tail, new_tail, 1)
# Long help update
t2 = t2.replace(
    "After create, the CLI re-checks links and warns if the server\nsilently dropped workItemIds (known Codeup trap).",
    "Body field workItemIds is sent as a comma-separated string (OpenAPI).\nAfter create, the CLI verifies links via workitem extRelationRecords; if still\nmissing it attempts CreateWorkitemExtRelationRecord, then fails (ok=false) if\nlinks remain missing — it does not pretend success.",
    1,
)
p.write_text(t2, encoding="utf-8")
print("codeup.go patched")

# --- patch +create ---
p = Path("cmd/codeup_mrs_plus.go")
t = p.read_text(encoding="utf-8")
t = t.replace(
    '\t\tbody := map[string]any{\n\t\t\t"title":           title,\n\t\t\t"sourceBranch":    source,\n\t\t\t"targetBranch":    target,\n\t\t\t"sourceProjectId": repositoryID,\n\t\t\t"targetProjectId": repositoryID,\n\t\t\t"createFrom":      "WEB",\n\t\t\t"description":     desc,\n\t\t\t"reviewerUserIds": reviewerIDs,\n\t\t\t"workItemIds":     workItemIDs,\n\t\t}',
    '''\t\tbody := map[string]any{
\t\t\t"title":           title,
\t\t\t"sourceBranch":    source,
\t\t\t"targetBranch":    target,
\t\t\t"sourceProjectId": repositoryID,
\t\t\t"targetProjectId": repositoryID,
\t\t\t"createFrom":      "WEB",
\t\t\t"description":     desc,
\t\t\t"reviewerUserIds": reviewerIDs,
\t\t}
\t\tif csv := mrlink.WorkItemIDsCSV(workItemIDs); csv != "" {
\t\t\tbody["workItemIds"] = csv
\t\t}''',
    1,
)
t = t.replace(
    "\t\t\twarnMRWorkItemLinks(m, resolvedWorkItems, mrMap)\n\t\t\treturn out, m",
    "\t\t\tif err := ensureMRWorkItemLinks(cmd.Context(), c, repositoryID, resolvedWorkItems, mrMap, m); err != nil {\n\t\t\t\treturn nil, nil // signal via panic? no — runJSONMutating callback can't easily fail\n\t\t\t}\n\t\t\treturn out, m",
)
# Actually runJSONMutating callback signature - need to check if it can return error
print("checking runJSONMutating")
