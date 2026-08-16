# Ticket Decision Core

一个无副作用、可审计的工单决策内核。外部系统负责读取工单、查询日志、调用接口、查询数据库以及执行工单操作；本项目只负责：

1. 根据当前事实判断还缺少哪些信息；
2. 将缺失事实转换成固定采集指令及其查询参数；
3. 根据采集结果匹配版本化规则；
4. 返回结构化决策，不直接执行任何外部操作。

## 边界

```text
工单/采集 CLI
    │  初始 facts
    ▼
ticket-decision-core
    │  collection_requests
    ▼
日志/API/数据库外部适配器
    │  新 facts
    ▼
ticket-decision-core
    │  actionable/no_match/manual_review/...
    ▼
工单处理 CLI
```

本项目不会：

- 执行 shell、SQL 或 HTTP 请求；
- 保存数据库凭据或 API Token；
- 自由生成查询指令；
- 读写工单；
- 把技术错误解释成 `no_match`。

`instruction` 是外部系统注册的稳定标识，例如 `logs.search-order-errors`，不是可直接执行的命令文本。

## 快速开始

要求 Go 1.26 或更高版本。

```powershell
git clone https://github.com/zaurakworks/ticket-decision-core.git
cd ticket-decision-core
go test ./...
```

验证策略文件：

```powershell
go run ./cmd/ticket-decision validate --rules demo/policy.json
```

使用初始工单事实执行第一轮判断：

```powershell
go run ./cmd/ticket-decision evaluate `
  --rules demo/policy.json `
  --input demo/round-1-ticket.json
```

结果为 `need_more_info`，并包含三个参数已经绑定的采集请求：日志、API 和数据库。

使用外部采集后的事实执行第二轮判断：

```powershell
go run ./cmd/ticket-decision evaluate `
  --rules demo/policy.json `
  --input demo/round-2-collected.json
```

结果为 `actionable`，并返回外部处理器可以消费的 `directive`。

连续查看两轮流程：

```powershell
go run ./cmd/decision-demo
```

## 输入

输入是一次不可变的事实快照：

```json
{
  "run_id": "ticket-9001/revision-1",
  "facts": {
    "ticket": {
      "request_id": "req-7f91",
      "order_id": "order-2048"
    }
  }
}
```

规则通过点路径访问事实，例如：

```text
ticket.request_id
evidence.logs.error_count
evidence.api.order_status
evidence.database.pending_jobs
```

机器契约见 [`schemas/input.schema.json`](schemas/input.schema.json)。

## 输出

第一轮可能要求外部采集：

```json
{
  "outcome": "need_more_info",
  "missing_facts": [
    "evidence.api.order_status",
    "evidence.database.pending_jobs",
    "evidence.logs.error_count"
  ],
  "collection_requests": [
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
  ]
}
```

采集完成后可能得到可执行决策：

```json
{
  "outcome": "actionable",
  "reason_code": "stuck_order_has_pending_job",
  "matched_rule_ids": ["retry-stuck-order"],
  "directive": {
    "handler": "retry_order_pipeline"
  }
}
```

支持的结果：

| Outcome | 含义 | 责任方 |
| --- | --- | --- |
| `actionable` | 规则已给出操作指令 | 外部执行器 |
| `need_more_info` | 缺少事实 | 外部采集器 |
| `no_match` | 信息充分但无规则匹配 | 外部路由或人工 |
| `ambiguous` | 同优先级规则冲突 | 人工或规则维护者 |
| `policy_denied` | 策略明确禁止自动执行 | 外部审批流程 |
| `already_satisfied` | 目标状态已经满足 | 外部回写工单 |
| `manual_review` | 规则明确要求人工处理 | 人工队列 |

机器契约见 [`schemas/decision.schema.json`](schemas/decision.schema.json)。

## 文档

- [外部采集与执行集成](docs/integration.md)
- [规则、采集器与优先级参考](docs/policy-reference.md)
- [完整两轮 Demo 策略](demo/policy.json)

## 命令退出码

| 退出码 | 含义 |
| --- | --- |
| `0` | 命令成功；`no_match` 也是成功的业务判断 |
| `2` | 命令参数错误 |
| `3` | 规则、输入或求值无效 |
| `4` | 输出失败 |

错误以 JSON 写入 stderr：

```json
{
  "error": {
    "code": "invalid_ruleset",
    "message": "..."
  }
}
```

## 仓库结构

```text
cmd/ticket-decision/  机器调用的 validate/evaluate CLI
cmd/decision-demo/    两轮流程演示
decision/             三值条件、优先级、采集计划与输出契约
demo/                 日志/API/数据库组合示例
schemas/              JSON Schema 机器契约
testdata/             测试输入和规则
```
