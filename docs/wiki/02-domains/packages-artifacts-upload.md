# packages artifacts upload (#379)

## API conclusion

- OpenAPI on `openapi-rdc` for **GENERIC upload/download** was announced 2025-02-27, but help-center product index still only lists List/Get/Delete; upload doc slugs 404.
- MCP `operations/packages` still only list/get artifacts.
- Working binary path is the **Packages protocol** host from 仓库指南 (same shape as funpub):

  - GENERIC: `POST {protocol}/{org}/generic/{repo}/files/{path}?version=...` multipart field `file`, Basic auth
  - MAVEN: `PUT {protocol}/{org}/maven/{repo}/{groupPath}/{artifact}/{version}/{file}` binary body, Basic auth

## CLI

`yunxiao packages artifacts upload` wraps GENERIC + MAVEN with `--dry-run` / `--yes` (risk=write).

### Dual credentials (both required)

1. **OpenAPI identity** — normal CLI auth (`YUNXIAO_ACCESS_TOKEN` / `credentials.json` / profile token). Used by `mustClient` and `ResolveOrgID` so the protocol URL can include `{org}`. Setting only `YUNXIAO_PACKAGES_*` is not enough.
2. **Packages protocol Basic** — `--username`/`--password` or `YUNXIAO_PACKAGES_USERNAME`/`YUNXIAO_PACKAGES_PASSWORD` from 仓库指南. Distinct from the PAT. **Validated even on `--dry-run`** (before HTTP); without them dry-run fails.

`--file` must be a relative path that exists under cwd; dry-run still reads the local file (`readRelativeFile`) for size/basename in the preview.

### Examples

```bash
# dry-run still needs protocol Basic (flag or env) + existing --file
yunxiao packages artifacts upload --repo-id <id> --repo-type GENERIC \
  --path myapp/build --version 1.0.0 --file ./app.bin \
  --username <u> --password <p> --dry-run

yunxiao packages artifacts upload --repo-id <id> --repo-type MAVEN \
  --group-id com.example --artifact-id demo --version 1.0.0 --file ./demo.jar \
  --username <u> --password <p> --dry-run
```

## Supported

| Type | Status |
|------|--------|
| GENERIC | supported (protocol POST multipart) |
| MAVEN | supported (protocol PUT) |

## Not wrapped

| Type | Status |
|------|--------|
| NPM | native: `npm publish` |
| DOCKER | native: `docker push` |
| NUGET | native: nuget/dotnet push |
| PYPI | native: twine |

OpenAPI PAT multipart upload: **not implemented** (docs unpublished) — not the same as "native client" above.
