---
name: yunxiao-packages
version: 1.1.0
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
yunxiao packages artifacts upload --repo-id <id> --repo-type GENERIC --path a/b --version 1.0.0 --file ./f.bin --dry-run
```

`artifacts delete` is **high-risk-write**. `artifacts upload` is **write** (GENERIC/MAVEN protocol).

> **Upload**：GENERIC/MAVEN via Packages protocol API (Basic auth from 仓库指南). OpenAPI binary upload docs still unpublished.

## Upload

```bash
# GENERIC (protocol multipart)
yunxiao packages artifacts upload --repo-id <id> --repo-type GENERIC --path myapp/build --version 1.0.0 --file ./app.bin --dry-run
yunxiao packages artifacts upload --repo-id <id> --repo-type GENERIC --path myapp/build --version 1.0.0 --file ./app.bin --username <u> --password <p> --yes

# MAVEN (protocol PUT)
yunxiao packages artifacts upload --repo-id <id> --repo-type MAVEN --group-id com.example --artifact-id demo --version 1.0.0 --file ./demo-1.0.0.jar --dry-run
```

Auth: `--username`/`--password` or `YUNXIAO_PACKAGES_USERNAME`/`YUNXIAO_PACKAGES_PASSWORD` (Packages 仓库指南 credentials — not PAT).
Protocol base: `YUNXIAO_PACKAGES_PROTOCOL_BASE` (default `https://packages.aliyun.com/api/protocol`).

Not wrapped (native clients): NPM=`npm publish`, DOCKER=`docker push`, NUGET=`nuget`/`dotnet push`, PYPI=`twine`.

## Known gaps

Repo create-delete 仍跳过。Upload：GENERIC/MAVEN 已封装（protocol）；NPM/DOCKER/NUGET/PYPI 用原生客户端。
