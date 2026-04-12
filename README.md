# view_panel

`view_panel` 是一个多人格 panel runtime：给定一个讨论问题、材料输入和人格集合，系统会按固定三阶段流程运行：

1. `01_prepare`：把问题、材料和人格确定性组装成 canonical Stage 1 artifacts
2. `02_answer`：为每个人格生成密封输入并运行单 worker / batch answer
3. `03_render`：把 Stage 2 结果聚合成 raw / certified render inputs，再输出 panel artifacts

当前仓库已经有一版可运行原型，但还不是 production-ready。

## 当前状态

当前实现状态：

- 已有可编译的 Go 模块和 CLI 入口
- 已实现 startup config / validation、shared storage / hash、prepare / answer / render 主链
- 已实现 app-server lifecycle 骨架
- 已实现面向 Stage 1 / Stage 2 / Stage 3 的 schema、skills 和 artifact contracts

当前仍然不是生产就绪，主要原因：

- `third_party/codex-protocol/0.0.0/` 目前是 checked-in placeholder bundle，不是 live-generated protocol export
- `internal/appserver/client.go` 当前按连续 JSON 流读取 stdout；如果真实协议使用 framed stdio，需要进一步适配
- 多个 `--materials` 输入当前使用保守的确定性策略，不是完整的多文档 merge contract
- prepare soft review 仍通过接口 seam 注入 reviewer，尚未绑定一个固定的 review runtime

## 仓库结构

当前最重要的目录：

- `cmd/worldview-panel/`
  - CLI 入口
- `internal/config/`
  - startup config normalization
- `internal/schema/`
  - startup validation
- `internal/storage/`
  - run-root layout、canonical writes、JSONL append
- `internal/hash/`
  - canonical JSON / file SHA-256 helpers
- `internal/stage/prepare/`
  - P04-P06：prepare assembly、hard gate、soft review
- `internal/stage/answer/`
  - P07-P09：workspace seal、single worker、batch orchestration
- `internal/stage/render/`
  - P10-P11：render aggregation、final render outputs
- `internal/appserver/`
  - P02：Codex app-server lifecycle
- `runtime/skills/`
  - `wv-prepare-stage`
  - `wv-answer-stage`
  - `wv-render-stage`
- `schemas/`
  - runtime JSON schema contracts
- `third_party/codex-protocol/0.0.0/`
  - pinned protocol bundle placeholder
- `runs/<run_id>/`
  - 每次运行的产物根目录

## 运行前提

你至少需要：

- Go `1.22`
- 已安装 `codex` CLI
- 已完成 `codex login`

当前 runtime contract 约束：

- pinned CLI contract 记录在 [codex-runtime-contract.md](/storage/emulated/0/projects/view_panel/docs/operations/codex-runtime-contract.md)
- canonical launcher 是 `codex app-server --listen stdio://`
- 如果登录态无效，worker execution 应该 fail closed

## 快速开始

先编译：

```sh
go build -o go_bin ./cmd/worldview-panel
```

运行：

```sh
./go_bin \
  -question "xxx讨论" \
  -materials ./materials/topic_x.json \
  -persona-set ./runtime/persona-index.json \
  -outdir ./out \
  -concurrency 6 \
  -model gpt-5.4
```

运行后，产物会写到：

```text
<outdir>/runs/<run_id>/
```

如果你省略 `-outdir`，默认是当前目录下的：

```text
./runs/<run_id>/
```

## Smoke

当前仓库已经提供真实全流程 smoke 入口：

```sh
make smoke
```

它会：

1. 构建二进制
2. 用固定 smoke dataset 跑真实 runtime flow
3. 自动检查关键 prepare / answer / render artifacts

如果仓库位于 Android/Termux 共享存储路径，仓库里的 `go_bin` 可能因为 `noexec` 不能直接执行；`make smoke` 已经内置了本地 mirror 运行方式，优先用它做真实联调。

更完整的运行、排查和发布说明见：

- [runtime-smoke-and-release-checklist.md](/storage/emulated/0/projects/view_panel/docs/operations/runtime-smoke-and-release-checklist.md)

## CLI 参数

当前 CLI 只接受 6 个启动参数：

- `-question`
  - 必填
  - 讨论问题
- `-materials`
  - 必填
  - 可重复传入，也可以传逗号分隔列表
  - 每个 entry 必须是存在的文件或目录
- `-persona-set`
  - 必填
  - 人格集合索引文件路径
- `-outdir`
  - 可选
  - 默认 `.`
- `-concurrency`
  - 可选
  - 默认 `1`
  - 必须 `>= 1`
- `-model`
  - 可选
  - 默认 `gpt-5.3-codex-spark`

CLI startup validation 失败时会向 `stderr` 输出 machine-readable JSON，并返回非零退出码。

## 输入约定

### 材料输入

当前 `materials` loader 支持两种 JSON 形状：

1. 顶层直接包含这 5 个字段
2. 顶层包含 `incident` 对象，而这 5 个字段在 `incident` 内

这 5 个字段是唯一正式字段名：

- `roleplay_prompt`
- `discussion_question`
- `supplementary_materials`
- `output_contract`
- `assumptions_and_constraints`

每个字段都必须是 JSON string。

### persona-set 输入

当前 `persona-set` loader 只允许“索引决定 membership”，不会靠扫描 `runtime/personas/` 自动发现人格。

支持两种索引形状：

1. 顶层数组
2. 顶层对象，形如：

```json
{
  "personas": [...]
}
```

每个 entry 可以是：

1. 一个字符串路径
2. 一个对象，支持：
   - `ref`
   - `path`
   - `persona_id`

约束：

- 每个 entry 只能设置 `ref` 或 `path` 其中之一
- persona backing file 必须是 JSON object
- persona definition 必须有 `persona_id`
- duplicate `persona_id` 会直接报错

### persona definition

当前 persona definition 的最低要求：

```json
{
  "persona_id": "xxx"
}
```

其它字段会保留给 Stage 1 prepare rendering 使用。

## 运行产物目录

每次运行的 canonical 布局：

```text
runs/<run_id>/
  request/
  01_prepare/
  02_answer/
  03_render/
  audit/
```

关键路径：

- `request/log.jsonl`
- `01_prepare/personas/<persona_id>/dispatch_input_v1.json`
- `01_prepare/personas/<persona_id>/agents.md`
- `01_prepare/personas/<persona_id>/prompt.txt`
- `01_prepare/personas/<persona_id>/manifest.json`
- `01_prepare/personas/<persona_id>/hashes.json`
- `01_prepare/prepare_review_v1.json`
- `01_prepare/prepare_gate_status_v1.json`
- `02_answer/personas/<persona_id>/workspace/AGENTS.md`
- `02_answer/personas/<persona_id>/input/prompt.txt`
- `02_answer/personas/<persona_id>/outgoing_input.json`
- `02_answer/personas/<persona_id>/result.raw.txt`
- `02_answer/personas/<persona_id>/result.json`
- `02_answer/personas/<persona_id>/attestation.json`
- `02_answer/personas/<persona_id>/status.json`
- `02_answer/answer_batch.json`
- `03_render/raw_render_input.json`
- `03_render/certified_render_input.json`
- `03_render/raw_panel.json`
- `03_render/raw_panel.md`
- `03_render/raw_cards.json`
- `03_render/certified_panel.json`
- `03_render/certified_panel.md`
- `03_render/certified_cards.json`
- `03_render/status.json`
- `audit/events.jsonl`
- `audit/errors.log`

## 三阶段说明

### Stage 1: Prepare

prepare 阶段负责：

- 加载材料
- 解析 persona-set
- 为每个人格写入唯一 canonical `dispatch_input_v1.json`
- 预渲染真正会发出去的：
  - `agents.md`
  - `prompt.txt`
- 生成 `manifest.json` 和 `hashes.json`
- 运行 advisory review
- 运行 hard gate

Stage 2 不允许再从 `dispatch_input_v1.json` 临时重组 prompt。

### Stage 2: Answer

answer 阶段负责：

- 从 `prepare_gate_status_v1.json` 决定是否允许进入 Stage 2
- 读取 Stage 1 的：
  - `agents.md`
  - `prompt.txt`
  - `hashes.json`
- 不读取 `dispatch_input_v1.json`
- 生成 worker-side sealed snapshot
- 生成 `outgoing_input.json`
- 运行单人格 worker
- 运行 batch orchestration
- 写出 `answer_batch.json`

当前 authoritative final output 只认：

- `item/completed.agentMessage`

### Stage 3: Render

render 阶段负责：

- 从 `answer_batch.json` 聚合 Stage 3 输入
- 写出：
  - `raw_render_input.json`
  - `certified_render_input.json`
- raw / certified 两条支路分别渲染
- 写出 panel / cards / markdown
- 最终写 `03_render/status.json`

当前 certified gate 使用：

- `min_success_rate = 0.8`

raw path 不受这个门槛阻断。

## 错误输出

CLI 当前会把主要失败分成几类 status：

- `invalid_cli_usage`
- `invalid_startup_config`
- `startup_failed`
- `runtime_failed`

输出格式是 JSON，便于脚本消费。

## 测试与验证

当前仓库可以用：

```sh
go test ./...
```

但要注意一个真实环境约束：

- 如果仓库直接位于 Android/Termux 的共享存储路径，Go 可能在 `go.mod` 上做文件锁时失败
- 这种情况下，需要先把仓库镜像到本地可锁目录，再运行 Go 命令

一个可用做法是：

```sh
tmpdir=$(mktemp -d /data/data/com.termux/files/home/view_panel_test_XXXXXX) && \
tar -cf - --exclude=.git . | (cd "$tmpdir" && tar -xf -) && \
cd "$tmpdir" && \
go test ./...
```

## 已知限制

当前最重要的限制：

- protocol bundle 还是 placeholder，不是 live-generated
- transport framing 可能还要按真实 app-server 行为调整
- material 多输入还没有完整 merge 语义
- prepare soft review 还没有固定绑定到唯一 runtime reviewer
- 当前还没有真正的细粒度自动化 smoke / e2e test 套件

## 开发说明

如果你是人类开发者，建议先读：

1. [README.md](/storage/emulated/0/projects/view_panel/README.md)
2. [codex-runtime-contract.md](/storage/emulated/0/projects/view_panel/docs/operations/codex-runtime-contract.md)
3. `internal/storage/` 和 `internal/hash/` 的共享 contract
4. `internal/stage/*` 下各阶段实现

如果你是通过 Codex 继续维护这个仓库，根目录 [AGENTS.md](/storage/emulated/0/projects/view_panel/AGENTS.md) 应该是你的第一入口。
