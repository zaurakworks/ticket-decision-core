# 外部采集与执行集成

## 组件职责

决策内核是纯函数：

```text
Decision = Evaluate(VersionedPolicy, FactSnapshot)
```

它不保存运行状态，也不执行采集或操作。调用方负责：

- 从工单读取初始事实；
- 注册固定采集指令；
- 执行日志、API、数据库查询；
- 将查询结果规范化为事实；
- 保存每一轮输入和输出；
- 执行或审批最终 `directive`；
- 控制重试、超时、并发和最大轮数。

## 固定指令注册表

策略只引用稳定的 `instruction` 标识：

```json
{
  "id": "order-error-logs",
  "kind": "log",
  "instruction": "logs.search-order-errors",
  "parameters": {
    "request_id": "ticket.request_id",
    "from": "ticket.window.from",
    "to": "ticket.window.to"
  },
  "provides": [
    "evidence.logs.error_count",
    "evidence.logs.last_error"
  ]
}
```

外部系统维护注册表，将标识映射到真实实现：

```text
logs.search-order-errors
  → 日志 CLI 的固定查询模板

api.get-order-status
  → 固定 HTTP 方法、路径和响应解析器

database.count-pending-order-jobs
  → 固定数据源、只读 SQL 和结果解析器
```

注册表必须约束：

- 允许的参数名称和类型；
- 参数是否必填；
- 超时和最大返回量；
- 允许访问的日志索引、接口和数据库；
- 输出到哪些事实路径；
- 凭据来源和权限。

不要把 shell、SQL、URL、Token 或连接串放入策略文件。策略只决定使用哪个已注册指令以及参数值。

## 一轮调用

### 1. 构造事实快照

```json
{
  "run_id": "ticket-9001/revision-1",
  "facts": {
    "ticket": {
      "request_id": "req-7f91",
      "order_id": "order-2048",
      "window": {
        "from": "2026-08-16T08:00:00Z",
        "to": "2026-08-16T08:15:00Z"
      }
    }
  }
}
```

`run_id` 应包含工单标识和事实版本。决策内核只回显它；调用方使用它关联审计记录并拒绝过期采集结果。

### 2. 执行判断

```powershell
go run ./cmd/ticket-decision evaluate `
  --rules demo/policy.json `
  --input demo/round-1-ticket.json
```

### 3. 分发采集请求

一个请求形如：

```json
{
  "collector_id": "order-status-api",
  "kind": "api",
  "instruction": "api.get-order-status",
  "parameters": {
    "order_id": "order-2048"
  },
  "produces": [
    "evidence.api.order_status"
  ]
}
```

调用方应校验：

1. `instruction` 已注册且类型与 `kind` 一致；
2. 参数符合注册表，不接受额外参数；
3. 请求仍对应当前工单版本；
4. 适配器只写入 `produces` 声明的事实路径；
5. 每次调用和原始响应均有可追溯记录，但敏感值不进入普通日志。

同一轮的多个 `collection_requests` 没有顺序依赖，可以并行执行。决策内核按 `collector_id` 排序，只为保证输出稳定。

### 4. 合并采集结果

外部适配器负责把不同数据源转换成统一事实：

```json
{
  "evidence": {
    "logs": {
      "error_count": 12,
      "last_error": "worker lease expired"
    },
    "api": {
      "order_status": "stuck"
    },
    "database": {
      "pending_jobs": 1
    }
  }
}
```

下一轮输入应保留初始事实，加入新事实，并使用新的 `run_id`：

```text
ticket-9001/revision-1 → ticket-9001/revision-2
```

缺少字段和字段值为 `null` 不等价：

- 路径不存在：条件结果可能为 UNKNOWN，并触发采集；
- 路径存在但值为 `null`：它是已提供值，通常会导致类型不匹配；
- 规则值不允许为 `null`；需要判断字段是否存在时使用 `exists`。

### 5. 再次判断

如果返回 `actionable`，外部执行器按 `directive.handler` 选择固定处理器：

```json
{
  "outcome": "actionable",
  "directive": {
    "handler": "retry_order_pipeline"
  }
}
```

`directive` 也是路由数据，不应直接解释为 shell 命令。

## 主循环

调用方可以实现为：

```text
facts = read_ticket()

repeat:
    decision = evaluate(policy, facts)
    persist(facts, decision)

    if decision.outcome == need_more_info:
        if decision.unresolved_facts is not empty:
            route_to_manual_or_initial_fact_collector()
            stop

        results = execute_registered_collectors(decision.collection_requests)
        facts = merge_as_new_snapshot(facts, results)
        continue

    if decision.outcome == actionable:
        approve_if_required(decision.directive)
        execute_registered_handler(decision.directive)
        stop

    route_terminal_outcome(decision)
    stop
```

必须由调用方设置最大判断轮数。超过上限应转人工或标记配置问题，不能把它转换成 `no_match`。

## 无法生成采集请求

当规则需要某个事实，但：

- 没有采集器声明可以提供它；或
- 采集器所需查询参数不存在；

内核返回 `unresolved_facts`：

```json
{
  "outcome": "need_more_info",
  "missing_facts": ["evidence.logs.error_count"],
  "unresolved_facts": ["ticket.request_id"]
}
```

此时外部不能执行不完整请求。它应补充初始工单字段、转人工，或修正规则/采集器配置。

## 失败边界

以下情况不是业务判断：

- CLI 无法启动；
- 规则文件无效；
- 数据类型不符合比较操作；
- 日志/API/数据库查询超时；
- 外部返回无法解析；
- 工单版本已变化。

这些错误由调用方按技术错误处理。只有成功求值后返回的 `no_match` 才表示“信息充分但无规则符合”。
