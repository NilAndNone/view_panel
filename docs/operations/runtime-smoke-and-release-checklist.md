# Runtime Smoke And Release Checklist

这份文档是当前 `view_panel` runtime prototype 的综合操作手册，覆盖：

1. 运行前检查
2. 真实 smoke 执行
3. 结果判读
4. 常见失败排查
5. 发布前检查

## 运行前检查

在执行真实 smoke 之前，先确认：

1. Go 版本可用

```sh
go version
```

2. Codex CLI 可用

```sh
codex --version
```

3. Codex 登录态有效

```sh
codex login
```

4. 当前 runtime contract 已知

- [codex-runtime-contract.md](/storage/emulated/0/projects/view_panel/docs/operations/codex-runtime-contract.md)
- startup 会先写 `runs/<run_id>/audit/runtime_contract_status.json`，然后才决定是否继续进入三阶段

5. 你知道当前 protocol bundle 还是 placeholder

- `third_party/codex-protocol/0.0.0/`

它目前不是 live-generated export，所以如果你在做 runtime contract 级别的联调，不要把它当成真实协议快照。

## 共享存储锁问题

如果仓库直接位于 Android/Termux 共享存储路径，Go 命令可能失败：

```text
go: RLock .../go.mod: function not implemented
```

当前 `Makefile` 已经把 `go build` 和 `go run ./tools/smokecheck` 放到了本地 mirror 目录里执行，所以：

- `make build`
- `make smoke`
- `make smoke-check`

都不需要你手工先复制仓库。

另外一个常见现实问题是共享存储上的 `noexec`：

- `make build` 可以把 `go_bin` 写回仓库
- 但这个 `go_bin` 不一定能在共享存储路径上直接执行
- `make smoke` 已经绕开了这个问题，它会在本地 mirror 里重新 build 并执行二进制，再把 run artifacts 写回仓库

所以如果你只是想验证真实全流程，优先用：

```sh
make smoke
```

如果你自己直接跑 `go build` / `go test`，仍然要注意这个限制。

## 真实 Smoke 执行

### 1. 构建

```sh
make build
```

### 2. 跑完整 smoke

```sh
make smoke
```

这会做四件事：

1. 构建二进制
2. 运行 startup runtime-contract check
3. 用固定 smoke dataset 跑真实全流程
4. 自动找到最新 `run_id` 并执行 `smoke-check`

### 3. Smoke dataset

当前固定 smoke dataset 在：

- `testdata/smoke/materials/smoke.json`
- `testdata/smoke/runtime/persona-index.json`
- `testdata/smoke/runtime/personas/analyst.json`
- `testdata/smoke/runtime/personas/builder.json`

规模固定为：

- 1 个材料文件
- 2 个人格

### 4. 输出目录

smoke 运行结果默认写到：

```text
out/smoke/runs/<run_id>/
```

如果你需要手工重跑检查：

```sh
make smoke-check RUN_ID=<run_id>
```

### 5. 相关 CLI 控制

真实 smoke 或人工运行时，当前还支持这些 startup 控制：

- `-review-enabled`
  - 默认 `true`
- `-worker-timeout-ms`
  - 默认 `0`，表示不启用 per-worker timeout
- `-max-attempts-per-persona`
  - 默认 `1`，当前只支持 `1`
- `-forbidden-tool-name`
  - 可重复传入或逗号分隔，进入 Stage 2 forbidden-tool policy

## 结果判读

### Startup runtime contract

先看：

- `audit/runtime_contract_status.json`

这里会记录 startup runtime-contract enforcement 结果。当前要点：

1. `status == "blocked"`
   - 说明 startup 在进入三阶段前就已经 fail closed
2. `status == "warning"`
   - 说明允许继续运行，但有 warning facts
3. `status == "ok"`
   - 说明本地 startup contract 检查通过

重点字段：

- `blocking_errors`
- `warnings`
- `observed_cli_version`
- `pinned_cli_version`
- `launcher_form`
- `protocol_bundle_path`

### Stage 1: Prepare

重点先看：

- `01_prepare/prepare_gate_status_v1.json`

smoke checker 当前要求：

1. `status == "pass"`
2. `can_proceed_to_stage2 == true`
3. `persona_ids` 长度必须是 `2`

如果这里不过，后面的 Stage 2 / Stage 3 结果都不可信。

### Stage 2: Answer

重点看：

- `02_answer/answer_batch.json`

当前 smoke checker 关心：

1. `counts.total_personas == 2`
2. `certified + failed + rejected == 2`

这一步不是要求所有人格都成功，而是要求 batch artifact 结构和计数闭合。

### Stage 3: Render

重点看：

- `03_render/raw_render_input.json`
- `03_render/certified_render_input.json`
- `03_render/status.json`

这里要区分：

1. `raw`
2. `certified`

`raw` 代表尽量展示可用结果。

`certified` 代表通过 `min_success_rate` 门槛后的可信结果路径。

prototype 阶段里，`certified` 没过门槛而被跳过，不一定是 bug。

## 当前 Smoke Checker 在检查什么

当前 `tools/smokecheck` 会检查：

1. prepare gate 通过
2. Stage 2 persona 总数闭合
3. render 关键 artifact 存在
4. raw / certified 分支状态和 `available` 关系一致

特别是：

- `raw_render_input.available == true` 时，`status.raw.state` 不能是 `skipped`
- `certified_render_input.available == true` 时，`status.certified.state` 必须是 `rendered`
- `certified_render_input.available == false` 时，`status.certified.state` 不能伪装成 `rendered`

## 常见失败排查

### 1. `go.mod` 锁失败

症状：

```text
go: RLock .../go.mod: function not implemented
```

处理：

- 优先使用 `make build` / `make smoke` / `make smoke-check`
- 不要先怀疑业务代码

### 2. Codex app-server 启动失败

常见原因：

1. `codex` 未安装
2. `codex login` 未完成
3. 当前登录态失效
4. 本地 Codex 环境与 runtime contract 不一致

先检查：

```sh
codex --version
codex login
```

然后再看：

- [codex-runtime-contract.md](/storage/emulated/0/projects/view_panel/docs/operations/codex-runtime-contract.md)
- `runs/<run_id>/audit/runtime_contract_status.json`

如果这里已经是 `status == "blocked"`，后面的 Stage 1/2/3 问题就不是首要矛盾。

### 3. Prepare gate block

重点看：

- `01_prepare/prepare_gate_status_v1.json`

一般从这几类问题入手：

1. `dispatch_input_v1.json` 结构不对
2. `manifest.json` / `hashes.json` 不一致
3. 审计路径不可写
4. persona 目录结构不对

### 4. Answer batch 大量失败

重点看：

- `02_answer/answer_batch.json`
- 各人格：
  - `status.json`
  - `attestation.json`
  - `result.json`

区分：

1. `failed`
2. `rejected`
3. `certified`

尤其注意：

- `rejected` 是 batch policy rejection
- `failed` 是运行期失败或结构失败

### 5. Certified gate 没过

重点看：

- `03_render/certified_render_input.json`

先看：

1. `T`
2. `C`
3. `observed_success_rate`
4. `gate_reason`

如果 certified 没过，不代表 raw 一定不可用。

### 6. Render branch skipped 或 failed

重点看：

- `03_render/status.json`

当前语义：

- `raw.state == skipped` 通常说明 raw input unavailable
- `certified.state == skipped` 通常说明上游 gate 没过
- `failed` 说明 branch 自己执行失败

## 发布前检查

真正准备提交或发版前，至少确认：

1. `make smoke` 跑过
2. smoke-check 通过
3. `README.md` 仍然和当前 CLI / artifact contract 对齐
4. `AGENTS.md` 仍然和当前仓库结构对齐
5. 如果改了 runtime contract：
   - `docs/operations/codex-runtime-contract.md` 已更新
   - `third_party/codex-protocol/<version>/` 已同步
6. 如果改了输入合同：
   - `dispatch_input_v1` 5 个正式字段名没有漂移
7. 如果改了 Stage 2 / Stage 3 artifact contract：
   - smoke checker 也同步更新

## 推荐操作顺序

如果你只是想确认当前原型还能跑，建议顺序是：

1. `make build`
2. `make smoke`
3. 看 `out/smoke/runs/<run_id>/03_render/status.json`
4. 再按需要往前追 `02_answer` 和 `01_prepare`
