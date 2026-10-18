# Windows Shell 与 `--jq` / 中文内联（#370）

PowerShell / 控制台代码页下，**内联中文 argv** 与 **复杂 `--jq` 表达式**（含花括号、引号嵌套）容易被 shell 改写或截断，表现为 CLI exit ≠ 0、stdout 为空，而去掉 `--jq` 后同一条命令正常。

## 一律优先（醒目）

| 场景 | 推荐 | 避免 |
|------|------|------|
| 建单 / 评论含中文 | `--subject-file` / `--title-file` / `--description-file` / `--content-file` / `--custom-fields-file`（UTF-8，可带 BOM；CLI 去 BOM） | PowerShell 内联 `--subject "中文…"` |
| 只要部分 JSON 字段 | 省略 `--jq`，用 Node/Python 解析 stdout；或管道到外部 `jq` | 在 WinPS 里写 `{subject: .data.subject, …}` 这类复杂内联 `--jq` |
| 消费完整信封 | Node/Python subprocess 以 **bytes/Buffer** 读 stdout 再 `JSON.parse` | `>` / `Out-File` 重定向（易改编码） |

`yunxiao doctor` 在 Windows 上会在 `meta.windows_shell_hint` 重复本条指引。

## `--jq` 失败时

CLI 报 `error.subtype=jq_failed`，并在 `error.hint` 给出可操作建议（编码 / 引号 / 改用管道或外部过滤）。常见根因不在云效 API，而在 **argv 到达 CLI 之前已被 PowerShell 弄坏**。

可操作顺序：

1. 先去掉 `--jq` 确认信封本身 `ok: true`。
2. 复杂过滤改用：
   ```powershell
   yunxiao workitem get ZYPT-1 --profile zhiyi | python -c "import sys,json; d=json.load(sys.stdin); print(d['data'].get('subject'))"
   ```
   或安装外部 jq 后：`yunxiao … | jq '.data.subject'`（注意管道编码；仍优先 Python/Node）。
3. 中文写操作改走 `*-file`（见 skill `yunxiao-project` / `#85` `#89`）。

## 未做：`--out-file`

将完整 JSON 落到文件可彻底避开控制台编码，但涉及全局输出契约与大量命令接线；**本 issue 不实现**。需要落盘时用外部重定向到 UTF-8 文件，或在 Agent 侧用 subprocess 写文件。

## 相关

- README「Agent / scripts on Windows」双语节
- [workitem-comments-oapi-gaps.md](../02-domains/workitem-comments-oapi-gaps.md)（`--content-file`）
- skill `yunxiao-shared` / `yunxiao-project`
- git-bash MSYS 路径改写：[yunxiao-shared](../../../skills/yunxiao-shared/SKILL.md) / README #117
