# workitem search 降噪（#368）

现场「群线程 ↔ 工作项」只读对账时，单关键词 `--subject` 噪声很大（几十～上百条历史关闭单）。

## 推荐组合

```bash
yunxiao workitem search --category Bug --subject-all "清空,筛选" --recent-days 30
```

| 旗标 | 行为 |
|------|------|
| `--subject` | 单个标题子串 CONTAINS（兼容旧用法） |
| `--subject-all` | CSV 多词；每个词各一条 subject CONTAINS，落在**同一** `conditionGroups` 组内 → **AND**。支持英文逗号与中文逗号 `，` |
| `--recent-days N` | 快捷映射为 `--updated-after` =（今天 − N）`00:00:00` **Asia/Shanghai**（`gmtModified` BETWEEN）。与显式 `--updated-after` **互斥** |

统一入口 `yunxiao search workitem ...` 会透传上述旗标（`--q` 仍只映射到 `--subject`）。

## 本 PR 未做

- `--as-items` 默认字段精简（serialNumber / status.name / subject / url）— 避免破坏已有脚本对完整对象的依赖；需要时另开 issue。
- `--rank`（多词命中 / 未关闭 / 近更新排序）— 改动面较大，另开。

## 相关

- skill：`skills/yunxiao-project/SKILL.md`
- 命令 Long help：`yunxiao workitem search --help`
