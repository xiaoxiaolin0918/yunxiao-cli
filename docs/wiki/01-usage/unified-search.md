# 统一搜索：`yunxiao search`

顶层薄路由（#352）：按**显式类型**转到已有域命令，不发明新搜索引擎，也不做多域混搜 / 默认搜全部。

## 用法

```bash
yunxiao search <type> [域命令已有旗标…]
yunxiao search --type <type> [域命令已有旗标…]
```

类型必须是**第一个位置参数**，或通过 `--type` / `--type=` 指定（便于 `yunxiao search --dry-run --type workitem …`）。不要把类型放在其它旗标后面的裸参数里。

## 类型表

| type | 落到 | `--q` / `--query` 映射 |
|------|------|------------------------|
| `workitem` / `issue` | `workitem search` | `--subject` |
| `mr` / `mrs` | `codeup mrs list` | `--search` |
| `repo` / `repos` | `codeup repos list` | `--search` |
| `case` / `cases` | `testhub cases search` | **无关键词旗标**；传 `--q` 会报错（请用 `--repo-id` 等） |
| `app-tag` | `appstack tags search` | `--search` |
| `pipeline` | `pipeline list` | `--name` |

v1 **不做**：`packages`、AppStack change-request 全文搜、多域一次返回。

未知 type → 非零退出，并打印上表。

## 旗标规则

- 全局旗标透传：`--jq` / `--dry-run` / `--format` / `--organization-id` / `--profile` / `--yes`
- 域旗标原样交给目标命令（与直接调用该命令一致）
- `--q` / `--query` 映射到上表关键词旗标；若目标已有同旗标且值不同 → 报错（不静默丢）
- 输出信封与目标子命令一致


> workitem 降噪旗标（#368）：--subject-all / --recent-days 走透传；详见 [workitem-search-noise.md](../02-domains/workitem-search-noise.md)。
## 权威 schema

本命令只是路由。参数与风险以目标命令为准，例如：

```bash
yunxiao schema workitem.search
yunxiao codeup mrs list --help
yunxiao search --help          # 类型表 + 示例
```

## 示例

```bash
yunxiao search workitem --subject login --category Bug
yunxiao search workitem --subject-all "清空,筛选" --recent-days 30  # #368 降噪
yunxiao search issue --q login --assigned-to self
yunxiao search --type mr --state opened --search fix
yunxiao search repo --q yunxiao-cli
yunxiao search cases --repo-id <id>
yunxiao search app-tag --q demo
yunxiao search pipeline --q deploy --dry-run
```

## 迁移

原 `yx search` / 「无统一 search」条目已改为指向本命令：见 [gh-yx-migration.md](../00-process/gh-yx-migration.md)。
