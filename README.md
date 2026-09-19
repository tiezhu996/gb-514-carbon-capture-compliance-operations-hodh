
# 碳捕集装置合规运行

工业园区捕集装置、许可规则、排放样本和合规决定平台。项目采用前后端分离和明确的领域分层，重点保证状态迁移、RBAC、审计日志、请求追踪与限流在各层保持一致。

## Docker Compose 快速启动

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

- Web 工作台：http://127.0.0.1:18514
- 后端健康检查：http://127.0.0.1:19514/healthz
- 后端 API：http://127.0.0.1:19514/api
- 演示账号：`viewer`、`operator`、`reviewer`、`admin`，密码均为 `Admin123!`（仅限本地演示，生产环境必须更换）

停止并清理本项目容器与数据卷：

```bash
docker compose down -v --remove-orphans
```

## 主要功能

| 业务模块 | 后端实体 | API 前缀 | 状态流 |
|---|---|---|---|
| 捕集装置 | `CaptureUnit` | `/api/units` | standby, running, limited, stopped |
| 许可规则 | `PermitRule` | `/api/rules` | draft, active, superseded, retired |
| 排放样本 | `EmissionSample` | `/api/samples` | collected, testing, verified, invalid |
| 合规决定 | `ComplianceDecision` | `/api/decisions` | draft, review, accepted, escalated |

- JWT 登录和 viewer/operator/reviewer/admin 四级 RBAC；写操作至少需要 operator，删除仅 admin，审计至少 reviewer。
- 合规决定每次创建、草稿修改和状态迁移都会事务追加不可变版本，保存状态、证据、操作者和 request ID；进入 review 后业务字段锁定，accepted/escalated 仅 reviewer 或 admin 可执行。
- 许可规则作废（retired）必须走专用按装置核验流程：先核验同一 `relatedCode` 的捕集装置存在且设施一致、存在最新已核验（verified）排放样本且设施一致，且无草稿（draft）或复核中（review）的关联合规决定；存在在途决定时拒绝并返回每个决定编号与阻断原因。样本缺失、装置不一致或读取失败同样拒绝。无阻断时仅 admin 可作废；旧规则以 retired 保留历史，已接受决定通过创建时快照的 `permitRuleCode/permitRuleVersion` 继续引用原版本。并发作废与新决定提交在同一装置锁事务内串行，只有一方成功，失败整体回滚不留半更新。
- 所有状态变化使用乐观锁并写入不可覆盖的审计日志。
- 请求 ID、结构化日志、全局错误映射和 Redis 分布式限流。
- 提供脱敏运行配置、当前会话、审计汇总和单实体审计历史接口。
- Angular 路由守卫和按钮显隐与后端角色边界一致；viewer 可浏览但不能写入，审计页要求 reviewer。
- 业务工作台支持查询、新建、状态推进、风险标识及操作审计查看；`ComplianceBadge` 复用于装置/决定页，`RuleDiff` 复用于规则/样本页，`EvidenceList` 展示版本证据上下文。

## 合规决定版本规则

| 操作 | 允许角色 | 版本行为 |
|---|---|---|
| 创建 draft | operator/reviewer/admin | 写入 v1，保留证据、actor、request ID |
| 修改 draft | operator/reviewer/admin | 乐观锁更新并追加下一版本 |
| draft → review | operator/reviewer/admin | 追加复核版本，禁止跳过 review |
| review → accepted/escalated | reviewer/admin | 追加最终决定版本；operator 会被拒绝 |
| review 后修改字段 | 无 | 返回业务规则错误，历史与证据不可覆盖 |

## 技术栈

| 层次 | 技术 |
|---|---|
| 前端 | Angular 17 + TypeScript + Vite + Angular Material |
| 后端 | Go 1.22 + Gin + GORM |
| 数据 | PostgreSQL + Redis |
| 部署 | Docker Compose + Nginx |

## 本地开发

后端可使用 SQLite 开发模式，不需要先启动数据库：

```bash
cd backend
go mod download
DATABASE_DRIVER=sqlite DATABASE_DSN=local.db REDIS_ADDR='' \
JWT_SECRET=local-development-secret PORT=8080 go run ./cmd/server
```

前端开发服务器：

```bash
cd frontend
npm install
npm run dev
```

质量检查：

```bash
cd backend && go test ./... && go build ./...
cd ../frontend && npm run typecheck && npm run build
cd .. && docker compose config --quiet
```

也可以从项目根目录执行 `./scripts/validate.sh`，脚本会构建、启动、检查健康接口和鉴权 API，并在结束时关闭容器。

## 目录结构

```text
.
├── backend/
│   ├── cmd/server/                 # 服务入口与优雅退出
│   └── internal/
│       ├── config/                 # 环境配置
│       ├── constants/              # 状态枚举与迁移图
│       ├── database/               # 连接、迁移与演示数据
│       ├── dto/                    # 输入契约
│       ├── handler/                # HTTP 接口
│       ├── middleware/             # JWT、追踪、限流
│       ├── model/                  # GORM 实体
│       ├── repository/             # 持久化边界
│       ├── router/                 # 路由装配
│       ├── service/                # 业务规则与审计
│       └── util/                   # 统一 HTTP 响应
├── frontend/src/
│   ├── api/                        # 按实体拆分的 API
│   ├── components/common/          # 共享业务组件
│   ├── hooks/                      # 认证与分页 hooks
│   ├── pages/                      # 五个路由页面
│   ├── router/                     # 路由配置
│   ├── stores/                     # 按实体拆分的状态仓库
│   ├── types/                      # 共享类型与枚举
│   └── utils/                      # 格式化与状态工具
├── docker-compose.yml
└── runtime_smoke.json
```

## 共享枚举位置

| 枚举 | 值 | 前后端出现位置 |
|---|---|---|
| `UnitState` | `standby, running, limited, stopped` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |
| `DecisionState` | `draft, review, accepted, escalated` | `backend/internal/constants/status.go`、`frontend/src/types/status.ts` |

每个实体自己的完整迁移图同样位于 `backend/internal/constants/status.go`；页面使用的状态列表位于 `frontend/src/types/status.ts`。修改状态时必须同步两处并更新对应服务测试。

## 环境变量

| 变量 | 说明 |
|---|---|
| `COMPOSE_PROJECT_NAME` | 固定英文 Compose 项目名，支持中文父目录 |
| `DB_NAME/DB_USER/DB_PASSWORD` | 数据库名称与业务账号 |
| `DB_ROOT_PASSWORD` | MySQL 管理员密码（PostgreSQL 项目保留统一模板字段） |
| `JWT_SECRET` | JWT 签名密钥，生产环境必须替换 |
| `FRONTEND_PORT/BACKEND_PORT/DB_PORT` | 宿主机端口映射 |
| `REDIS_PORT` | Redis 宿主机端口 |
| `MINIO_*` | 证据对象存储配置（启用 MinIO 的项目） |

## API 使用示例

```bash
token=$(curl -sS -X POST http://127.0.0.1:19514/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin123!"}' | jq -r '.data.token')

curl -sS http://127.0.0.1:19514/api/overview \
  -H "Authorization: Bearer $token"
```

## 许可规则作废核验

| 接口 | 方法/角色 | 行为 |
|---|---|---|
| `/api/rules/:id/retirement-check` | GET，登录即可 | 只读按装置核验，返回 `allowed` 及全部 `blockers`（含决定编号与原因） |
| `/api/rules/:id/retire` | POST，仅 admin | 事务内重新核验并作废；阻断返回 422 `retirement_blocked` + `details`，版本冲突返回 409 |

阻断码：`rule_state`、`device_missing`、`device_mismatch`、`sample_missing`、`in_flight_decision`（带 `decisionCode`）；装置/样本/决定的数据库读取失败返回 5xx 而不是放行。`retired` 为终态，不能再经通用 `/transition` 迁移；草稿规则不绑定装置也无法作废。

## License

MIT
