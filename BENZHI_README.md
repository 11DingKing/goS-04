# BENZHI_README

## 项目说明

- 项目：11DingKing/goS-04
- 项目用途：面向祁连山华隆保护站的巡护任务协同调度后端服务，覆盖巡护员、值班调度员、站长三类角色，围绕巡护网格、巡护任务、告警事件、手持终端四个核心实体展开业务编排。
- Go 工具链：`golang:1.26`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/server

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-4-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-4-arm64 linux/arm64
docker run -it benzhi-task-4-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-4-arm64:latest
```

## 题目验证命令

1. 预期退出码 1：`go test ./internal/service -run "^TestUnresolvedLevelOneAlarmsEscalateAfterTimeout$" -count=1 -v`

## Bug 复现

Bug 现象、触发步骤和完整错误信息见 `BUG_REPRO.md`。
