# AI 效果评估文档

本文档记录 AI 组件的离线评测方法、指标定义、测试集构建和 CI 门禁配置。

---

## 1. RAG 检索评测

### 1.1 指标定义

| 指标 | 定义 | 计算方式 |
|------|------|----------|
| **Recall@K** | 前 K 个结果中包含期望文档的比例 | `Recall@K = (命中期望文档数) / (期望文档总数)` |
| **MRR (Mean Reciprocal Rank)** | 平均倒数排名 | `MRR = (1/N) * Σ(1/rank_i)`，其中 `rank_i` 是第 i 个查询首个相关文档的排名 |

### 1.2 测试集构建

测试集位于 `testdata/rag_eval_cases.json`，包含 20 个真实用户查询场景：

| 类别 | 样本数 | 示例 |
|------|--------|------|
| 房间管理 | 4 | 开播/关播/创建/修改标题 |
| 礼物系统 | 3 | 目录/价格/送礼流程 |
| 连麦系统 | 3 | 发起/接受/规则 |
| PK 系统 | 3 | 发起/规则/倒计时 |
| 审核体系 | 4 | 规则/申诉/违规词/举报 |
| 房管权限 | 2 | 权限/申请 |
| 数据查看 | 2 | 在线人数/数据看板 |

每个测试案例包含：
- `query`: 用户自然语言查询
- `expected_doc_ids`: 期望召回的知识库文档 ID 列表
- `description`: 场景描述

### 1.3 运行评测

```bash
# 本地运行（使用 Mock Embedder）
EVAL_ENABLED=1 go test -run TestEvalAll ./internal/ai/evaluation/ -v

# CI 中运行（GitHub Actions 自动设置 EVAL_ENABLED=1）
go test -run TestEvalAll ./internal/ai/evaluation/
```

### 1.4 门禁阈值

| 指标 | 当前阈值 | 说明 |
|------|----------|------|
| MRR | >= 0.5 | 平均倒数排名 |
| Recall@3 | >= 0.4 | Top-3 召回率 |

> **注意**：阈值基于当前 Mock Embedder 设定。接入真实 Embedding 模型后应重新标定。

---

## 2. Agent 工具调用评测

### 2.1 指标定义

| 指标 | 定义 | 计算方式 |
|------|------|----------|
| **Tool Name Accuracy** | 工具名识别准确率 | `正确工具名调用次数 / 总测试用例数` |
| **Args Parse Success** | 参数解析成功率 | `JSON 参数成功解析次数 / 触发工具调用次数` |

### 2.2 测试集构建

测试集位于 `testdata/agent_tool_eval_cases.json`，覆盖 5 个核心工具：

| 工具 | 测试用例数 | 参数复杂度 |
|------|------------|------------|
| `mute` | 2 | room_id, user_id, duration |
| `unmute` | 1 | room_id, user_id |
| `query_room_status` | 2 | room_id |
| `query_leaderboard` | 2 | room_id, period, top_n |
| `query_pk_status` | 1 | room_id |

### 2.3 门禁阈值

| 指标 | 当前阈值 |
|------|----------|
| Tool Name Accuracy | >= 75% |
| Args Parse Success | >= 75% |

---

## 3. SSE 端到端延迟评测

### 3.1 指标定义

| 指标 | 定义 |
|------|------|
| **First Token Latency** | 从用户发送消息到收到第一个流式文本 token 的耗时 |
| **Full Response Latency** | 从用户发送消息到收到 `done` 事件的总耗时 |

统计分位点：Min, Mean, Median, P50, P95, P99

### 3.2 测试场景

测试集位于 `testdata/sse_latency_eval_cases.json`，8 个场景覆盖：
- 简单问候（基线）
- RAG 检索查询
- 单工具调用查询
- 多工具调用查询
- 知识库问答

### 3.3 门禁阈值

| 指标 | 当前阈值 | 说明 |
|------|----------|------|
| First Token P95 | <= 5s | 首包延迟 |
| Full Response P95 | <= 30s | 完整响应（含多轮工具调用） |

---

## 4. CI 门禁配置

### 4.1 GitHub Actions Workflow

在 `.github/workflows/ci.yml` 中新增评测 job：

```yaml
jobs:
  ai-eval:
    name: AI Evaluation
    runs-on: ubuntu-latest
    needs: build
    if: github.event_name == 'push' || github.event_name == 'pull_request'
    env:
      EVAL_ENABLED: "1"
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true
      - name: Run AI Evaluation
        run: |
          go test -run TestEvalAll ./internal/ai/evaluation/ -timeout 10m
      - name: Upload Evaluation Artifacts
        uses: actions/upload-artifact@v4
        with:
          name: ai-eval-metrics
          path: test_output/*.json
          retention-days: 7
```

### 4.2 本地开发流程

```bash
# 1. 生成/更新测试集（如有新场景）
go test -run TestRAGEval ./internal/ai/evaluation/ -v

# 2. 运行全量评测
EVAL_ENABLED=1 go test -run TestEvalAll ./internal/ai/evaluation/ -v

# 3. 查看生成的指标文件
cat test_output/rag_metrics.json
cat test_output/agent_tool_metrics.json
cat test_output/sse_latency_metrics.json
```

---

## 5. 指标看板

评测产出的 JSON 指标文件可接入 Grafana/Prometheus 做趋势看板：

- `rag_metrics.json` → RAG 检索趋势
- `agent_tool_metrics.json` → Agent 工具调用准确率趋势
- `sse_latency_metrics.json` → SSE 延迟分位点趋势

建议设置告警：
- MRR 环比下降 > 10% → 告警
- Tool Name Accuracy 环比下降 > 5% → 告警
- SSE P95 环比上升 > 20% → 告警

---

## 6. 常见问题

### Q: 为什么用 Mock Embedder 跑评测？
A: 保证 CI 确定性、无外部依赖、秒级完成。真实 Embedding 模型评测建议在专门的夜ly pipeline 中跑。

### Q: 测试集怎么维护？
A: 新增用户反馈的 Bad Case → 加入测试集 → 跑评测 → 优化 Prompt/检索/重排 → 验证指标回升。

### Q: 怎么对接真实向量库评测？
A: 见 [向量数据库接入文档](./vector_store.md)，切换 `VECTOR_STORE=pgvector` 环境变量即可。

---

## 7. 版本记录

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-08-28 | 初始版本：RAG/Agent/SSE 三大评测 + CI 门禁 |