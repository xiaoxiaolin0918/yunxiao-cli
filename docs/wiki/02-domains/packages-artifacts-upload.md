# packages artifacts upload (#379)

## API conclusion

- OpenAPI on `openapi-rdc` for **GENERIC upload/download** was announced 2025-02-27, but help-center product index still only lists List/Get/Delete; upload doc slugs 404.
- MCP `operations/packages` still only list/get artifacts.
- Working binary path is the **Packages protocol** host from 仓库指南 (same shape as funpub):

  - GENERIC: `POST {protocol}/{org}/generic/{repo}/files/{path}?version=...` multipart field `file`, Basic auth
  - MAVEN: `PUT {protocol}/{org}/maven/{repo}/{groupPath}/{artifact}/{version}/{file}` binary body, Basic auth

## CLI

`yunxiao packages artifacts upload` wraps GENERIC + MAVEN with `--dry-run` / `--yes` (risk=write).

Credentials: Packages protocol username/password (flags or `YUNXIAO_PACKAGES_*`), distinct from `YUNXIAO_ACCESS_TOKEN`.

## Not wrapped

| Type | Status |
|------|--------|
| GENERIC | supported (protocol) |
| MAVEN | supported (protocol) |
| NPM | native: `npm publish` |
| DOCKER | native: `docker push` |
| NUGET | native: nuget/dotnet push |
| PYPI | native: twine |

OpenAPI PAT multipart upload: **not implemented** (docs unpublished) — not the same as "native client" above.
