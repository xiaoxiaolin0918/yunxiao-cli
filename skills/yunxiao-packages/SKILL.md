---
name: yunxiao-packages
version: 1.1.1
description: "Yunxiao Packages: list/get/delete artifacts; upload GENERIC/MAVEN via protocol API."
metadata:
  requires:
    bins: ["yunxiao"]
  cliHelp: "yunxiao packages --help"
---

# packages

开始前先读 [`../yunxiao-shared/SKILL.md`](../yunxiao-shared/SKILL.md)。

> List 响应 `meta` 可能含 `has_more` / `total` / `page`；`has_more==true` 时勿把首页当全集（见 yunxiao-shared）。

## Typed commands

```bash
yunxiao packages repos list
yunxiao packages repos list --repo-types MAVEN --page 1
yunxiao packages artifacts list --repo-id <id> --repo-type GENERIC
yunxiao packages artifacts get --repo-id <id> --repo-type GENERIC --id <aid>
```

```bash
yunxiao packages artifacts delete --repo-id <id> --repo-type GENERIC --id <aid> --dry-run
yunxiao packages artifacts delete --repo-id <id> --repo-type MAVEN --id <aid> --version-id <vid> --yes
# dry-run still needs protocol Basic creds (flag or env) + a real --file on disk
yunxiao packages artifacts upload --repo-id <id> --repo-type GENERIC --path a/b --version 1.0.0 --file ./f.bin --username <u> --password <p> --dry-run
```

`artifacts delete` is **high-risk-write**. `artifacts upload` is **write** (GENERIC/MAVEN protocol).

> **Upload**：GENERIC/MAVEN via Packages protocol API. OpenAPI binary upload docs still unpublished.

## Upload

**Dual credentials (both required):**

1. **OpenAPI identity** — normal `yunxiao` auth (`YUNXIAO_ACCESS_TOKEN` / credentials / profile). `mustClient` + `ResolveOrgID` need this to build `{org}` in the protocol URL. Protocol-only env is not enough.
2. **Packages protocol Basic** — `--username`/`--password` or `YUNXIAO_PACKAGES_USERNAME`/`YUNXIAO_PACKAGES_PASSWORD` from 仓库指南. **Not** the PAT. Checked even on `--dry-run` (before any HTTP).

Also: `--file` must exist as a relative path under cwd; dry-run still calls `readRelativeFile`.

```bash
# GENERIC (protocol multipart) — dry-run needs Basic creds too
yunxiao packages artifacts upload --repo-id <id> --repo-type GENERIC --path myapp/build --version 1.0.0 --file ./app.bin --username <u> --password <p> --dry-run
# or: export YUNXIAO_PACKAGES_USERNAME=... YUNXIAO_PACKAGES_PASSWORD=...
yunxiao packages artifacts upload --repo-id <id> --repo-type GENERIC --path myapp/build --version 1.0.0 --file ./app.bin --yes

# MAVEN (protocol PUT)
yunxiao packages artifacts upload --repo-id <id> --repo-type MAVEN --group-id com.example --artifact-id demo --version 1.0.0 --file ./demo-1.0.0.jar --username <u> --password <p> --dry-run
```

Protocol base: `YUNXIAO_PACKAGES_PROTOCOL_BASE` (default `https://packages.aliyun.com/api/protocol`).

### Supported / not wrapped

| Type | Status |
|------|--------|
| GENERIC | supported (protocol POST multipart) |
| MAVEN | supported (protocol PUT) |
| NPM | not wrapped — `npm publish` |
| DOCKER | not wrapped — `docker push` |
| NUGET | not wrapped — nuget/dotnet push |
| PYPI | not wrapped — twine |

## Known gaps

Repo create-delete still skipped. Upload: GENERIC/MAVEN via protocol; NPM/DOCKER/NUGET/PYPI use native clients.
