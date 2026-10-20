# packages repos list credential redaction (#386)

## Problem

`GET .../packages/.../repositories` returns `repoDescriptor` as a JSON **string**. For Hybrid / proxied repos that string embeds `hybridDescriptor.proxies[].username` and `password` in plaintext. Printing the raw list leaks credentials into terminals, CI logs, and agent transcripts.

## CLI behavior

`yunxiao packages repos list` runs an after-hook before `output.Success`:

- Recursively mask map keys `password` / `username` / `passwd` / `secret` / `accessKey` / `accessKeySecret` (case-insensitive) as `***` (empty string and JSON `null` stay unchanged).
- If `repoDescriptor` is a JSON string, parse → redact → re-serialize so nested proxy credentials are masked inside the string.

Dry-run previews are unaffected (no response body). Artifacts list/get are unchanged (no descriptor credentials observed).

## Verify

```bash
yunxiao --profile play --format json packages repos list
# data[*].repoDescriptor must not contain plaintext password/username values
```
