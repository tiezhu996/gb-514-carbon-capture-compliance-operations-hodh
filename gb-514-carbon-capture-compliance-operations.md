请生成 `carbon-capture-compliance-operations`「碳捕集装置合规运行」Go 全栈项目，面向工业园区管理捕集单元、排放样本、许可规则和合规决定。项目是能源环保运行控制，不做财务核算、订单、库存或报表看板产品。

## 项目主要需求

复杂度下限：核心实体不少于 3 个、核心页面不少于 4 个、横切关注点不少于 2 个、共享前端组件不少于 3 个、自定义 hooks/utils 不少于 2 个、后端中间件不少于 2 个。

### 核心实体

`CaptureUnit`（装置与运行状态）、`PermitRule`（许可阈值和版本）、`EmissionSample`（采样结果）、`ComplianceDecision`（合规/限产/停机决定）贯穿数据库、Go model/service/handler 和前端。

### 核心页面

`/units` 捕集装置；`/rules` 许可规则；`/samples` 排放样本；`/decisions` 合规决定；`/audit` 审计。`ComplianceBadge` 在装置和决定页共用，`RuleDiff` 在规则和样本页共用。

### 横切关注点

RBAC 贯穿角色表、认证授权中间件、前端守卫和显隐；合规决定必须版本化并记录证据、操作者和 request ID；全局错误、限流和请求追踪独立实现。

### 共享枚举/组件

同步 `UnitState`（standby/running/limited/stopped）与 `DecisionState`（draft/review/accepted/escalated）。共享 `StatusBadge`、`EvidenceList`、`ConfirmDialog`，hooks 为 `useAuth`、`usePagination`。

### 技术与规模要求

前端 Angular 17 + TypeScript + Vite；后端 Go 1.22 + Gin + GORM；PostgreSQL + Redis。目标 3000–4200 行、30–42 个 `.go` 文件。

### 文件结构强制清单

前端 `api/stores/types/components/common/hooks/pages/router/utils`；后端 `model/dto/repository/service/handler/router/middleware/constants/util`。README 必须列出枚举出现位置。

### 结构红线

严禁合并职责到单一文件；许可规则、样本和合规决定必须跨层拆分。

### 部署与交付

根目录必须提供 `docker-compose.yml`（顶层 `name: carbon-capture-compliance-operations`，且不写 `version:`）、`.env` 和 `.env.example`（均含 `COMPOSE_PROJECT_NAME=carbon-capture-compliance-operations`）、`README.md`、`frontend/Dockerfile`、`backend/Dockerfile` 和 `frontend/nginx.conf`。前端端口 `18514`、后端端口 `19514`；Nginx `/api` 代理、healthcheck、命名卷和 `condition: service_healthy` 齐全，提供真实 `/healthz`、Git 初始化。
