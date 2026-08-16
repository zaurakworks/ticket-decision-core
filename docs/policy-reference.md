# 规则、采集器与优先级参考

## 策略文件

一个策略文件包含版本、采集器和规则：

```json
{
  "schema_version": 1,
  "id": "order-recovery",
  "version": "2026-08-16.1",
  "collectors": [],
  "rules": []
}
```

要求：

- `schema_version` 当前必须为 `1`；
- `id` 标识策略；
- `version` 标识不可变的规则版本；
- `rules` 至少包含一条规则；
- 同一策略中的规则 ID 和采集器 ID 必须唯一。

每次决策都会回显 `ruleset_id` 和 `ruleset_version`。策略内容变化时应更新 `version`，以便复现历史判断。

完整机器约束见 [`../schemas/ruleset.schema.json`](../schemas/ruleset.schema.json)。

## 采集器

采集器描述“哪个固定外部指令能够提供哪些事实”：

```json
{
  "id": "pending-jobs-database",
  "kind": "database",
  "provides": [
    "evidence.database.pending_jobs"
  ],
  "instruction": "database.count-pending-order-jobs",
  "parameters": {
    "order_id": "ticket.order_id"
  }
}
```

### kind

只允许：

```text
log
api
database
```

实际 JSON 值没有空格，分别为 `log`、`api`、`database`。

### provides

`provides` 声明采集器一次成功调用应返回的事实路径。

一个事实只能由一个采集器提供。重复 provider 会使策略验证失败，避免引擎无法确定应该调用哪个外部数据源。

一个采集器可以同时提供多个事实。例如一次日志查询可以同时提供错误数和最近一条错误。

### instruction

`instruction` 是外部注册表中的标识，不是命令文本。修改真实查询模板不要求规则知道 shell、SQL、HTTP 或连接信息，但若修改会改变事实语义，应同时提升策略或外部指令版本。

### parameters

参数映射格式为：

```json
{
  "外部参数名": "事实路径"
}
```

例如：

```json
{
  "request_id": "ticket.request_id",
  "from": "ticket.window.from",
  "to": "ticket.window.to"
}
```

求值时，引擎从当前 facts 读取路径值并生成：

```json
{
  "parameters": {
    "request_id": "req-7f91",
    "from": "2026-08-16T08:00:00Z",
    "to": "2026-08-16T08:15:00Z"
  }
}
```

任一参数事实缺失时，该采集请求不会生成，缺失路径进入 `unresolved_facts`。

## 规则

```json
{
  "id": "retry-stuck-order",
  "priority": 200,
  "when": {
    "all": [
      {
        "fact": "evidence.logs.error_count",
        "operator": "gte",
        "value": 5
      },
      {
        "fact": "evidence.api.order_status",
        "operator": "eq",
        "value": "stuck"
      }
    ]
  },
  "then": {
    "outcome": "actionable",
    "reason_code": "stuck_order_has_pending_job",
    "directive": {
      "handler": "retry_order_pipeline"
    }
  }
}
```

### when

每个条件必须且只能使用一种形式：

- `all`：全部子条件成立；
- `any`：至少一个子条件成立；
- `not`：对子条件取反；
- `fact/operator/value`：叶子比较。

`all` 和 `any` 不允许为空。

### 三值逻辑

条件结果不只有 true/false：

| 值 | 含义 |
| --- | --- |
| TRUE | 当前事实证明条件成立 |
| FALSE | 当前事实证明条件不成立 |
| UNKNOWN | 判断所需事实路径不存在 |

组合语义：

- `all` 中任一 FALSE，则整体 FALSE；否则存在 UNKNOWN 时整体 UNKNOWN；
- `any` 中任一 TRUE，则整体 TRUE；否则存在 UNKNOWN 时整体 UNKNOWN；
- `not` 将 TRUE/FALSE 互换，UNKNOWN 保持 UNKNOWN。

因此，下列 `all` 不会因为 `user.active` 缺失而要求采集，因为已知的工单类型已经证明整条规则不符合：

```json
{
  "all": [
    {"fact": "ticket.type", "operator": "eq", "value": "account_access"},
    {"fact": "user.active", "operator": "eq", "value": true}
  ]
}
```

当 `ticket.type` 为 `hardware_repair` 时，整体结果直接是 FALSE。

### 操作符

| 操作符 | 语义 | value 要求 |
| --- | --- | --- |
| `eq` | 类型严格相等 | 任意非 null JSON 值 |
| `neq` | 类型严格不等 | 任意非 null JSON 值 |
| `in` | fact 是否属于 value | value 必须为数组 |
| `not_in` | fact 是否不属于 value | value 必须为数组 |
| `gt` | 大于 | fact 和 value 必须为数字 |
| `gte` | 大于等于 | fact 和 value 必须为数字 |
| `lt` | 小于 | fact 和 value 必须为数字 |
| `lte` | 小于等于 | fact 和 value 必须为数字 |
| `contains` | 字符串包含子串，或数组包含元素 | 类型必须对应 |
| `exists` | 路径是否存在 | value 必须为布尔值 |

类型不匹配是求值错误，不是 FALSE，也不是 `no_match`。

判断可选字段不存在：

```json
{
  "fact": "ticket.optional_owner",
  "operator": "exists",
  "value": false
}
```

### then

规则作者只能返回终态业务结果：

```text
actionable
policy_denied
already_satisfied
manual_review
```

以下结果由引擎拥有，规则中不能直接填写：

```text
need_more_info
no_match
ambiguous
```

`reason_code` 必填并应保持稳定，供外部指标、路由和审计使用。

`directive` 可选，是外部处理器消费的不透明 JSON。建议只包含处理器标识和业务参数，不包含可执行脚本或凭据。

## 优先级决议

数字越大，优先级越高。引擎按以下顺序决议：

1. 对全部规则进行三值求值；
2. 如果某条 UNKNOWN 规则能够达到或超过当前已匹配规则的优先级，先返回 `need_more_info`；
3. 只请求当前最高优先级 UNKNOWN 规则所缺的事实，避免提前查询低优先级数据；
4. 如果最高优先级只有一条 TRUE 规则，返回它的 `then`；
5. 如果同一最高优先级存在多条 TRUE 规则，返回 `ambiguous`；
6. 如果没有 TRUE，也没有 UNKNOWN，返回 `no_match`。

示例：

```text
优先级 200：高风险操作需要审批，但 ticket.risk 未采集
优先级 100：普通操作规则已经匹配
```

结果仍是 `need_more_info(ticket.risk)`。否则低优先级操作可能绕过高风险规则。

当高优先级规则被事实证明为 FALSE 后，引擎才会采用低优先级匹配或请求低优先级规则需要的事实。

## 冲突

同优先级多条规则同时匹配时，引擎不按文件顺序选择，而返回：

```json
{
  "outcome": "ambiguous",
  "reason_code": "conflicting_rules",
  "matched_rule_ids": ["rule-a", "rule-b"]
}
```

解决方法是：

- 调整规则条件，使它们互斥；或
- 明确调整优先级；或
- 保留冲突并由人工判断。

规则文件顺序不具有业务含义。

## no_match

只有在所有相关规则都得到确定的 FALSE 且不存在可能影响结果的 UNKNOWN 时，才返回：

```json
{
  "outcome": "no_match",
  "reason_code": "no_rule_match"
}
```

`no_match` 是成功的业务判断，CLI 返回退出码 0。它不能用于表示采集失败、类型错误或规则文件无效。
