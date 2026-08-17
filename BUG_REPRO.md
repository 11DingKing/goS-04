# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

帮我排查一级告警不升级的问题，先不要修改代码。

站长反馈：火情类一级告警被处置人接单后，如果一直没有处置完成，超过两小时也不会自动升级，站长收不到升级通知，后台定时检查这一轮的升级数量是 0；把同一条告警推进到处置中，情况一样。相反，只完成审核定级、还没有人接单的一级告警，超过两小时后能正常升级；已处置完成的告警不升级是符合预期的。

请定位具体是哪个 Go 文件、哪个符号的哪一处行为造成上面的现象，说明它如何一步步导致接单之后的一级告警永远进不了升级流程，并给出实际的代码阅读或定向复现证据。这一轮只要诊断结论，请先不要改动仓库里的代码。

## 含 Bug 版本

- 仓库：11DingKing/goS-04
- 仓库地址：https://github.com/11DingKing/goS-04.git
- parent SHA：ac8283337c93fb97c07811005e6b6692c5fe0030

## 复现步骤

```bash
git clone -- https://github.com/11DingKing/goS-04.git bug-repro
cd bug-repro
git checkout --detach ac8283337c93fb97c07811005e6b6692c5fe0030
go test ./internal/service -run "^TestUnresolvedLevelOneAlarmsEscalateAfterTimeout$" -count=1 -v
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/service -run "^TestUnresolvedLevelOneAlarmsEscalateAfterTimeout$" -count=1 -v
=== RUN   TestUnresolvedLevelOneAlarmsEscalateAfterTimeout
    overdue_escalation_test.go:39: expected the 2 unresolved level-1 alarms to escalate, got 0
--- FAIL: TestUnresolvedLevelOneAlarmsEscalateAfterTimeout (0.00s)
FAIL
FAIL	patrol-platform/internal/service	0.038s
FAIL

```

stderr：

```text
(empty)
```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test ./internal/service -run "^TestUnresolvedLevelOneAlarmsEscalateAfterTimeout$" -count=1 -v
=== RUN   TestUnresolvedLevelOneAlarmsEscalateAfterTimeout
    overdue_escalation_test.go:39: expected the 2 unresolved level-1 alarms to escalate, got 0
--- FAIL: TestUnresolvedLevelOneAlarmsEscalateAfterTimeout (0.00s)
FAIL
FAIL	patrol-platform/internal/service	0.002s
FAIL

```

stderr：

```text
(empty)
```

## 通过条件

准确定位 internal/domain/alarm.go 的 (*Alarm).IsOverdue
说明 accepted / in_progress 被当作终止状态后，EscalateOverdueAlarms 的过滤与写锁内二次确认都被短路、升级计数恒为 0、Escalated 与 EscalatedAt 不被写入的完整因果链，并解释为何未接单的一级告警仍能升级
结论有代码阅读或定向复现证据（如 go test ./internal/service -run '^TestUnresolvedLevelOneAlarmsEscalateAfterTimeout$' -count=1 -v）；目标仓库保持零改动
