# CPA Spark 管理接口（alpha）

技术标识保留 `cpa-spark`；路径前缀 `/v0/management/plugins/cpa-spark`。所有管理接口由 CPA 自身的 management authentication 保护。公共资源路径仅返回页面壳与 JS/CSS，不枚举账号或执行模型调用。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/state` | 配置、最近 200 个批次、服务端时间、存储诊断 |
| GET | `/accounts` | 由 `host.auth.list` 返回的 Codex/Antigravity 账号显示信息 |
| POST | `/config` | 整体保存，必须携带当前 `revision`；冲突返回 409 |
| POST | `/preview` | `{task: {...}}` 返回五次 UTC 触发、目标账号与服务端时间 |
| POST | `/run` | `{taskId: "...", key: "..."}` 创建手动批次；key 8–128 字符 |
| POST | `/run-once` | `{task: {...}, key: "..."}` 执行一次，不保存定时任务 |
| POST | `/delete-history` | `ids` / `all` / `filter` 三选一；`preview:true` 仅返回数量，保留执行中的记录 |
| POST | `/cancel` | `{runId: "..."}` 撤销尚未领取的来源；在途继续返回 |

## 配置示例

```json
{
  "revision": 0,
  "groups": [],
  "tasks": [{
    "id": "my-task",
    "name": "工作账号",
    "provider": "codex",
    "accountIds": ["从CPA取得的真实账号ID"],
    "groupIds": [],
    "excludes": [],
    "days": [1, 2, 3, 4, 5],
    "times": ["06:00", "11:00", "16:00"],
    "timezone": "Asia/Shanghai",
    "model": "填写目标账号实际可用的模型ID",
    "effort": "low",
    "enabled": false,
    "revision": 1
  }],
  "settings": {"timezone":"Asia/Shanghai","concurrency":4,"late":120,"retention":30,"paused":false},
  "overrides": {}
}
```

`late` 为秒，`retention` 为天。配置不接受管理密钥或 OAuth token；不把账号邮箱作为执行身份。界面层是 optimistic concurrency 的客户端，数据库是唯一事实来源。读取 revision=0 后的首次成功保存返回 revision=1。

内部执行状态：`queued` → `running` → `success` / `failed` / `unknown`；领取前可转 `skipped`。`running` 是已经持久化领取的保守状态，可能包括提交宿主前的账号复查，不宣称所有 running 都已到达上游。

历史清理保留 claims 与 execution_keys：旧幂等键可能返回已过展示保留期的批次 ID，但不会创建第二次模型请求。版本 0.1 尚未提供恢复已清理批次的查询接口。

推理强度由 CPA 模型定义自动生成：离散等级使用 `none` / `minimal` / `low` / `medium` / `high` / `xhigh` / `max` / `auto`；原生预算在任务中存为 `budget:N`。Codex 等级沿用 `reasoning_effort`；Gemini 等级和原生预算通过 [CPA 官方模型括号后缀](https://help.router-for.me/configuration/thinking) 传给宿主执行器。只有模型声明支持的选项会在任务下拉中显示；缺少元数据时默认不覆盖推理参数。


`delete-history` 的筛选结构为 `{query, date, status, timezone}`；日期使用 `YYYY-MM-DD`，按所选 IANA 时区判定。清除全部忽略筛选。服务端扫描数据库所有记录，范围不限于 `/state` 返回的最近 200 条。删除历史不会删除模型请求去重凭据或修改任务配置。

`accounts` 的 `plan` 仅显示从明确订阅字段解析出的类型；必要时回调 `host.auth.get_runtime` 和 `host.auth.get` 读取认证元数据。响应中不返回 OAuth token 或认证 JSON。无法确认的套餐显示“未识别”。
