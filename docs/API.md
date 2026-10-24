# CPA Spark 管理接口（alpha）

技术标识保留 `cpa-spark`；路径前缀 `/v0/management/plugins/cpa-spark`。所有管理接口由 CPA 自身的 management authentication 保护。公共资源路径仅返回页面壳与 JS/CSS，不枚举账号或执行模型调用。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/state` | 配置、各任务最近一次及进行中批次的轻量摘要、今日计数、服务端时间、存储诊断 |
| GET | `/accounts` | 由 `host.auth.list` 返回的 Codex/Antigravity 账号显示信息 |
| POST | `/config` | 整体保存，必须携带当前 `revision`；冲突返回 409 |
| POST | `/preview` | `{task: {...}}` 返回五次 UTC 触发、目标账号与服务端时间 |
| POST | `/run` | `{taskId: "...", key: "..."}` 创建手动批次；key 8–128 字符 |
| POST | `/run-once` | `{task: {...}, key: "..."}` 执行一次，不保存定时任务 |
| POST | `/history` | `{mode,filter,page,pageSize}` 服务端分页，`pageSize` 为 20/50/100，默认 50 |
| POST | `/history-detail` | `{id}` 按批次 ID 读取完整详情，支持不在首页摘要中的历史 |
| POST | `/account-timeline` | `{id}` 读取指定账号近 24 小时的执行记录 |
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


`delete-history` 的筛选结构为 `{query, date, status, timezone}`；日期使用 `YYYY-MM-DD`，按所选 IANA 时区判定。删除全部忽略筛选。服务端按整个保留期计算筛选范围，覆盖所有页，`preview:true` 返回待删除的已结束记录数。删除历史不会删除模型请求去重凭据或修改任务配置。

`accounts` 的 `plan` 仅显示从明确订阅字段解析出的类型；必要时回调 `host.auth.get_runtime` 和 `host.auth.get` 读取认证元数据。响应中不返回 OAuth token 或认证 JSON。无法确认的套餐显示“未识别”。

### 执行记录显示模式（0.1.8）

前端默认按账号实际执行展示，同一次合并请求仅显示一条；任务模式按任务批次展示。`history-detail.items[].executionStatus` 为实际请求状态，`status` 保留该任务来源的状态。

删除接口原有任务批次范围保持兼容，`filter.taskId` 可按稳定任务 ID 筛选。按账号删除使用 `{"mode":"account","executionIds":["执行 ID"],"preview":true}` 预览，省略 `preview` 执行删除；`{"mode":"account","all":true}` 删除全部已结束的账号执行。账号模式不能混用批次 IDs；支持 `{"mode":"account","filter":{...}}` 跨页删除筛选结果。共享执行被删除后所有关联任务均不再显示此账号请求，其他账号、进行中记录及防重凭据保留。

### 分页协议

`POST /history` 示例：

```json
{"mode":"account","page":1,"pageSize":50,"filter":{"query":"","date":"","status":"all","timezone":"Asia/Shanghai","taskId":"","accountMatches":[]}}
```

返回 `rows,total,deletable,allDeletable,page,pageSize,pages`。`deletable` 为所有匹配页中的已结束数量；`allDeletable` 忽略筛选。空结果保留第 1 页；删除后越界页自动退至最后一页。按 `started DESC,id DESC` 排序。账号模式以真实 execution ID 去重，任务模式以 run ID 展示并提供 `accountCount` 和聚合 `status`。账号的 `runs` 只包含来源摘要；完整结果按需通过 `history-detail` 获取。

`filter.date` 沿用现有单日筛选，按所选时区的本地午夜区间计算，覆盖夏令时的非 24 小时日期。`accountMatches` 是客户端按当前账号名或邮箱匹配到的 ID 列表，仅作为账号模式关键词搜索的附加匹配范围；不影响其他过滤条件。

`/state.runs` 的 `summary:true` 表示不携带逐账号 items，首页的今日成功数/尝试数读取 `todayStats`；详情和近 24 小时时间线分别请求，不与分页列表共享截断范围。旧数据库自动在事务中补建来源索引，原有记录、配置和防重凭据不迁移删除。
