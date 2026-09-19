# 验收记录

验收日期：2026-08-22（Asia/Shanghai）

## 静态质量门

以下命令均以退出码 0 完成：

```bash
cd backend
gofmt -w $(find . -name '*.go' -type f)
go test ./...
go test -race ./...
go vet ./...
go build ./...

cd ../frontend
npm run typecheck
npm run build

cd ..
docker compose config --quiet
```

- 非测试 Go 代码：3099 行。
- 非测试 Go 文件：38 个。
- `TestComplianceDecisionPreservesEveryVersionAndReviewerBoundary` 验证 v1-v4 证据快照、operator 最终决定阻断、reviewer 决策和 review 后字段锁定。
- `TestComplianceDecisionRequiresReviewBeforeAcceptance` 验证 draft 不能跳过 review。
- Vite 仅报告单 bundle 超过 500 kB 的优化提示，不影响构建或运行。

## 空卷 Compose 与 API

执行 `KEEP_RUNNING=1 ./scripts/validate.sh`，从新建 PostgreSQL/Redis 命名卷启动，四个服务均进入 healthy/running：

| 服务 | 宿主机端口 | 结果 |
|---|---:|---|
| frontend | 18514 | 页面与 `/api` 代理正常 |
| backend | 19514 | `/healthz` 正常 |
| PostgreSQL | 20514 | healthy |
| Redis | 21514 | healthy |

脚本真实验证结果：

- admin 当前会话、运行配置、四实体列表、概览和审计汇总均可读取。
- viewer 创建装置返回 403，读取审计返回 403。
- operator 创建合规决定后得到含 actor/request ID 的 v1；提交 review 后 v1、v2 同时保留。
- operator 尝试 accepted 返回 422；reviewer 使用同一 expectedVersion 完成 accepted。
- accepted 响应同时保留 v1/v2/v3，v3 actor 为 reviewer 且 request ID 非空。
- 捕集装置创建、状态迁移以及对应审计记录均通过断言。

## 内置 Browser

仅使用 Codex 内置 Browser，未使用外部 Chrome。实测页面：

- `/units`：`ComplianceBadge` 和 `EvidenceList` 有真实记录；搜索 `CU-001` 后结果、指标和共享视图同步缩小；完成 standby → running；通过确认框创建新装置，列表总数由 4 变为 5。
- `/rules`：`RuleDiff` 展示 PR-001 至 PR-003 的阈值差异，表格和证据同步显示。
- `/samples`：复用同一个 `RuleDiff`，展示 ES-001 至 ES-003 的样本差异。
- `/decisions`：显示 API 验证产生的 accepted 决定及 `v3 · reviewer · request ID`；通过 UI 将 CD-002 从 review 推进到 accepted，证据区即时显示 `v2 · admin · request ID`。
- `/audit`：能看到 operator 提交、reviewer 接受、admin UI 迁移/创建及其 request ID，状态前后值一致。

视觉与运行时检查：

- 桌面全页截图无控件重叠或文本截断。
- 390 x 844 窄屏首屏导航、指标和操作按钮可用。
- 修复工具栏溢出后，像素检查为 `innerWidth=390`、`bodyWidth=390`、`documentWidth=390`。
- Browser 控制台 `error` / `warning` 日志结果为 `[]`。

## 清理

最终验收后执行：

```bash
docker compose down -v --remove-orphans
```

项目容器、命名卷和网络均应为空；Git 使用独立 `main` 分支和仓库本地公司身份，不创建提交或推送。
