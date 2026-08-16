# 巡护任务协同调度平台

面向祁连山华隆保护站的巡护任务协同调度后端服务，覆盖巡护员、值班调度员、站长三类角色，围绕巡护网格、巡护任务、告警事件、手持终端四个核心实体展开业务编排。

## 业务规则

- 关键点位打卡间隔不得超过六小时（超时仍记录但标记违规）
- 一级告警须在两小时内处置，逾期自动升级并通知站长
- 核心区巡护须双人同行并提前报备
- 巡护轨迹每十五分钟自动回传一次
- 逾期未处置的告警自动升级并通知站长
- 两名巡护员同时认领同一网格任务时以先提交者为准
- 同一告警被多人同时接单时仅保留最早接单人
- 终端离线时轨迹与打卡记录本地暂存，网络恢复后按时间顺序自动补传
- 处置人超时未响应时任务自动回滚到待派池并重新分配，全程保留操作留痕

## 技术栈

- Go 1.26，仅使用标准库，零外部依赖
- HTTP 接入：`net/http` ServeMux（方法+路径模式）
- 持久化：线程安全内存存储（`sync.RWMutex`，每实体独立锁）
- 后台任务：基于 `context` 取消的定时检查器
- 配置：JSON 文件 + 默认值，支持环境变量 `PATROL_CONFIG` 指定路径

## 项目结构

```
cmd/server/          程序入口
internal/config/     配置加载与校验
internal/domain/     领域实体、状态机、业务规则
internal/store/      线程安全内存持久化
internal/service/    应用编排（并发边界、幂等、失败恢复）
internal/transport/  HTTP 路由与处理器
internal/worker/     后台定时任务（升级、超时回滚）
```

## 快速开始

```bash
# 本地运行
go run ./cmd/server

# 服务监听 :58839
curl http://localhost:58839/api/health
```

## 主要接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/grids | 创建巡护网格 |
| GET | /api/grids/{id} | 查询网格 |
| POST | /api/tasks | 下发巡护任务 |
| GET | /api/tasks/{id} | 查询任务 |
| POST | /api/tasks/{id}/claim | 认领任务 |
| POST | /api/tasks/{id}/pre-report | 双人巡护提前报备 |
| POST | /api/tasks/{id}/start | 开始巡护 |
| POST | /api/tasks/{id}/clockin | 关键点位打卡 |
| POST | /api/tasks/{id}/track | 上报轨迹点 |
| POST | /api/tasks/{id}/complete | 完成巡护 |
| POST | /api/tasks/{id}/rollback | 回滚任务到待派池 |
| POST | /api/alarms | 上报告警事件 |
| POST | /api/alarms/{id}/review | 审核定级 |
| POST | /api/alarms/{id}/assign | 指派处置人 |
| POST | /api/alarms/{id}/accept | 接单 |
| POST | /api/alarms/{id}/resolve | 处置完成 |
| POST | /api/terminals | 注册终端 |
| POST | /api/terminals/{id}/offline | 终端离线 |
| POST | /api/terminals/{id}/sync | 离线数据补传 |
| POST | /api/terminals/{id}/clockin | 离线缓存打卡 |
| POST | /api/terminals/{id}/track | 离线缓存轨迹 |

## 配置

`config.json` 位于项目根目录，可通过环境变量 `PATROL_CONFIG` 指定替代路径：

```json
{
  "http_addr": ":58839",
  "track_interval": "15m",
  "clock_in_max_gap": "6h",
  "level1_timeout": "2h",
  "claim_timeout": "30m",
  "accept_timeout": "30m"
}
```

## 测试

```bash
go test -timeout=120s -count=1 ./...
```

覆盖正常路径、错误路径、状态迁移、并发争用（任务认领/告警接单）、幂等（打卡/轨迹/离线补传）、失败恢复（超时回滚/告警升级/离线同步）。

## Docker

```bash
# 构建镜像（默认 amd64）
docker build -t patrol-platform:1.0 .

# 多架构构建（amd64 + arm64）
docker buildx build --platform linux/amd64,linux/arm64 -t patrol-platform:1.0 --load .

# 运行容器
docker run -p 58839:58839 patrol-platform:1.0

# 验证
curl http://localhost:58839/api/health
```
