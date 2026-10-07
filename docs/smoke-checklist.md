# Play / sandbox smoke checklist

Manual checklist after a release or large workitem/Codeup change. Prefer a **play** tenant profile and always start with `--dry-run` where writes exist.

## Prep

```bash
yunxiao --version
yunxiao auth status
yunxiao doctor
yunxiao profile doctor --space-id <play-space>   # optional: enum drift
```

## Workitem

```bash
# precheck (#107)
yunxiao workitem +bug-create --title "smoke" --dry-run
# display-value options (#126)
yunxiao workitem create --type-id <bug-type> --field priority=高 --dry-run
# transition direct-try fallback (#123)
yunxiao workitem +bug-transition --id <id> --to <status> --dry-run
# risk / req create (#128)
yunxiao workitem +risk-create --title "smoke-risk" --dry-run
yunxiao workitem +req-create --title "smoke-req" --dry-run
# labels (#141)
yunxiao project labels list
yunxiao workitem search --labels <name> --per-page 5
```

## Codeup

```bash
# bare org/repo (#125)
yunxiao codeup mrs list --repo org/repo --per-page 5
# merge_rejected diagnose (#127) — only on a disposable MR
yunxiao codeup mrs merge --repo <id> --local-id <n> --dry-run
```

## Auth / doctor / update

```bash
yunxiao auth status          # OAuth expiry warning within ~24h (#122)
yunxiao profile doctor --fix-suggest
yunxiao update --check
```

## Agent sidecars

```bash
yunxiao skills path
yunxiao skills list
# after yunxiao update: skills/ next to the binary should match the new version
```

Mark each command: pass / fail / skipped. Attach JSON envelopes for failures.