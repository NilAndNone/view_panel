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

- `third_party/codex-protocol/0.0.0/` 现在是 checked-in live captured bundle；刷新方式是 `make capture-protocol`
- `internal/appserver/stream.go` / `internal/appserver/client.go` 现在会优先探测 `Content-Length` framed stdio，并在未命中 framed 头时回退到 sequential JSON；checked-in `smoke-transcript.jsonl` 仍然只是最小 sequential-JSON capture evidence
- 多个 `--materials` 输入现在只按 canonical 五字段执行显式 merge contract，不替代更通用的文档合并语义
- runtime contract 启动检查现在会校验 pinned CLI/version、canonical launcher prefix、bundle 完整性、live-capture metadata 和 bundle hashes；P02 仍然负责 live handshake / turn lifecycle

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
- `internal/content/`
  - rich content catalog / compile / bundle write helpers
- `content/src/`
  - 人格与材料的 authoring source-of-truth
- `content/build/<bundle_id>/`
  - 编译后的 runtime-ready bundle；默认 bundle 是 `content/build/default/`
- `runtime/skills/`
  - `wv-prepare-stage`
  - `wv-answer-stage`
  - `wv-render-stage`
- `schemas/`
  - runtime JSON schema contracts
- `third_party/codex-protocol/0.0.0/`
  - pinned live-captured protocol bundle
- `runs/<run_id>/`
  - 每次运行的产物根目录

## Content bundle 构建

`content/src/` 是 rich content 的 authoring source-of-truth，不是 runtime stages 的直接输入。
当前 runtime contract 不变：Stage 1 / Stage 2 / Stage 3 继续只消费编译后的 JSON bundle，不要让运行链路直接读取 `content/src/` 下的源文件。

当前保留两条直接 workflow：

- direct panel run：先从 `content/src/` 构建 bundle，再把 `worldview-panel` 指向 `content/build/<bundle_id>/runtime/`
- direct distinctness eval：先从 `content/src/` 构建 bundle，再把 evaluator 指向同一个 compiled bundle 和 questions 文件；如果不走 `make content-eval`，必须显式传一个位于 `content/build/<bundle_id>/` 外部的 `-out` 路径

构建 compiled bundle：

```sh
make build-content
```

默认 bundle id 是 `default`。如果你要生成别的 bundle id，可以直接覆盖：

```sh
make build-content CONTENT_BUNDLE_ID=review_candidate
```

输出路径会跟随 `CONTENT_BUNDLE_ID`：

- `content/build/<bundle_id>/bundle-manifest.json`
- `content/build/<bundle_id>/runtime/persona-index.json`
- `content/build/<bundle_id>/runtime/personas/*.json`
- `content/build/<bundle_id>/runtime/materials/*.json`

运行 distinctness evaluator：

```sh
make content-eval
```

它会先构建当前 `CONTENT_BUNDLE_ID`，再把评估结果写到：

- `out/content-eval/<bundle_id>/latest/distinctness-report.json`
- `out/content-eval/<bundle_id>/latest/distinctness-report.md`
- `out/content-eval/<bundle_id>/latest/panel_runs/<question_id>/runs/<run_id>/`

例如：

```sh
make content-eval CONTENT_BUNDLE_ID=review_candidate
```

这个评估器会复用现有 `worldview-panel` CLI 跑真实 panel run，再用确定性启发式打分：

- near duplicate outputs
- missing anchors
- generic framing

`content/src/evals/distinctness/questions.json` 里的每个问题现在都需要显式声明
`anchor_domain`，这样评估时只会对该问题对应的 domain scope 做 anchor 命中检查，
不会把所有 domain 文本混在一起打分。

当前默认 authored cohort 已经包含：

- `analyst`
- `builder`
- `historian`
- `operator`
- `policy`
- `skeptic`

默认把 eval 输出放在 `out/content-eval/default/latest/`，是为了避免
`make build-content` 重建 `content/build/<bundle_id>/` 时把历史评估结果一起删掉。

如果你直接运行 evaluator，也可以把输出根目录指定到
`out/content-eval/<bundle_id>/no_review/` 这类路径；报告文件仍然是
`distinctness-report.json` 和 `distinctness-report.md`。关键约束是不管
bundle id 是什么，直接调用时都必须显式传 `-out`，并且这个路径必须位于
`content/build/<bundle_id>/` 外部；不要把 evaluator 输出写回 compiled bundle
树里。

构建路径的 infra smoke 使用固定 fixture，而不是 live product cohort：

```sh
make content-smoke-build
```

如果仓库直接位于 Android/Termux 共享存储路径，手动 build/run 还要注意两个现实限制：

- Go 命令本身可能因为 `go.mod` 锁限制失败；`Makefile` targets 会先把仓库 mirror 到本地临时目录再执行，并默认跳过 `.git`、`out/`、`runs/`、`content/build/`、`go_bin` 这类 generated trees；临时根目录可用 `TMP_ROOT=/abs/path` 覆盖
- 共享存储上的 repo-root `./go_bin` 常常不可执行；`make build` 默认会把二进制写到 `$HOME/worldview-panel_bin`

所以当前 workspace 的推荐手动运行路径是：直接使用 `make build` 的默认输出，或显式把二进制写到别的可执行绝对路径，而不是依赖仓库根目录的 `./go_bin`。当前 `make build BIN_PATH=/abs/path/to/worldview-panel` 也会自动创建 `BIN_PATH` 的父目录。

手动启动时还有一个容易踩坑的前提：进程的 `cwd` 必须位于这个 repo tree 内，
例如 repo root 本身；不要因为二进制在 `$HOME/worldview-panel_bin` 就从 `$HOME`
或别的无关目录启动它，否则 repo discovery 不会命中当前仓库。

例如：

```sh
make build
```

再在 repo 内部目录启动这个可执行路径，驱动现有 runtime：

```sh
cd /absolute/path/to/view_panel
"$HOME/worldview-panel_bin" \
  -question "xxx讨论" \
  -materials ./content/build/default/runtime/materials/technology.json \
  -persona-set ./content/build/default/runtime/persona-index.json \
  -outdir ./out \
  -concurrency 6 \
  -review-enabled=true \
  -worker-timeout-ms=0 \
  -max-attempts-per-persona=2 \
  -forbidden-tool-name shell,web
```

## 运行前提

你至少需要：

- Go `1.22`
- 已安装 `codex` CLI
- 已完成 `codex login`

当前 runtime contract 约束：

- pinned CLI contract 记录在 [codex-runtime-contract.md](/storage/emulated/0/projects/view_panel/docs/operations/codex-runtime-contract.md)
- checked-in protocol bundle 是 `third_party/codex-protocol/0.0.0/` 下的 live capture；需要刷新时使用 `make capture-protocol`
- startup 会先执行 runtime-contract enforcement，再进入三阶段运行
- blocking / warning 结果会写到 `runs/<run_id>/audit/runtime_contract_status.json`
- canonical launcher 是 `codex app-server --listen stdio://`
- app-server transport decoder 支持 `Content-Length` framed stdio，并在未探测到 framed 头时回退到 sequential JSON
- 如果登录态无效，worker execution 应该 fail closed

## 快速开始

先从 `content/src/` 构建默认 runtime bundle：

```sh
make build-content
```

在当前 shared-storage checkout 里，优先把二进制写到可执行路径，而不是把 repo-root `./go_bin` 当作主运行路径：

```sh
make build
```

如果默认临时目录不适合当前机器，也可以显式覆盖 `TMP_ROOT`：

```sh
make build TMP_ROOT="$HOME"
```

如果你想直接运行 `go build` / `go test` / `go run`，先把仓库 mirror 到本地可锁且可执行的目录，再在 mirror 里执行这些 Go 命令。

运行前先切到 repo root，或者 repo tree 内的任意子目录：

```sh
cd /absolute/path/to/view_panel
"$HOME/worldview-panel_bin" \
  -question "xxx讨论" \
  -materials ./content/build/default/runtime/materials/technology.json \
  -persona-set ./content/build/default/runtime/persona-index.json \
  -outdir ./out \
  -concurrency 6 \
  -review-enabled=true \
  -worker-timeout-ms=0 \
  -max-attempts-per-persona=2 \
  -forbidden-tool-name shell,web
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

如果你需要刷新 pinned protocol bundle（例如 pinned CLI / protocol contract 发生变化后），先运行：

```sh
make capture-protocol
```

当前仓库已经提供真实全流程 smoke 入口：

```sh
make smoke
```

它会：

1. 构建二进制
2. 用当前 checked-in 的 live protocol bundle 运行 startup runtime-contract check
3. 用固定 smoke dataset 跑真实 runtime flow
4. 自动检查关键 prepare / answer / render artifacts

`make smoke` 当前不会自动刷新 protocol bundle；它直接使用仓库里现有的 checked-in live bundle。需要刷新 pinned evidence 时，先单独运行 `make capture-protocol`。

`make smoke` 现在遵循当前 startup surface，不再传 legacy top-level `-model`。
手动运行时也不要再传这个 flag：`-model=<non-empty>` 会以
`invalid_cli_usage` 被拒绝，而显式空值 `-model=` 会继续落到 startup
config 校验，再以 `invalid_startup_config` 失败。

如果仓库位于 Android/Termux 共享存储路径，仓库里的 `go_bin` 可能因为 `noexec` 不能直接执行；`make smoke` 已经内置了本地 mirror 运行方式，优先用它做真实联调。

更完整的运行、排查和发布说明见：

- [runtime-smoke-and-release-checklist.md](/storage/emulated/0/projects/view_panel/docs/operations/runtime-smoke-and-release-checklist.md)

## CLI 参数

当前 CLI 启动参数：

- `-question`
  - 必填
  - 讨论问题
- `-materials`
  - 必填
  - 可重复传入，也可以传逗号分隔列表
  - 每个 entry 必须是存在的文件或目录
  - runtime 会按 CLI 输入顺序保留显式文件；目录会展开成词典序 regular files；随后把全部选中的 canonical materials files 一起 merge
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
- `-review-enabled`
  - 可选
  - 默认 `true`
  - 控制是否执行 prepare soft review
- `-worker-timeout-ms`
  - 可选
  - 默认 `0`
  - `0` 表示不启用 per-worker timeout
- `-max-attempts-per-persona`
  - 可选
  - 默认 `1`
  - 必须 `>= 1`
  - Stage 2 会对每个人格串行执行最多该次数的 attempts
  - 一旦某次 attempt 被判定为 `certified`、`rejected`，或命中 batch-level timeout / cancellation failure，就会提前停止，不再继续后续 attempts
  - forbidden-tool rejection 属于 `rejected`，不会继续重试
  - `answer_batch.json` 里的 `attempt_count` 表示该人格实际执行了多少次 attempt；只有 `batch_cancelled_before_start` 这类未实际 dispatch 的失败会是 `0`
- `-forbidden-tool-name`
  - 可选
  - 可重复传入，也可以传逗号分隔列表
  - startup 会做去重、trim 和小写归一化
  - 会进入 Stage 2 batch policy 的 forbidden tool 检查

当前 runtime contract 不支持 top-level `-model`；如果手动传入该 flag，
`-model=<non-empty>` 会以 `invalid_cli_usage` 直接返回非零退出码，而显式空值
`-model=` 会继续落到 startup config 校验，再以 `invalid_startup_config`
返回非零退出码。

CLI startup validation 失败时会向 `stderr` 输出 machine-readable JSON，并返回非零退出码。
在 startup validation 通过后，runtime 还会执行 runtime-contract check；如果是 blocking mismatch，会在写出 `runs/<run_id>/audit/runtime_contract_status.json` 后直接停止。

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

当提供多个 `-materials` entry 时，runtime 会对所有选中的 canonical materials files 执行显式 merge contract：

- 不再采用“pick one canonical file”
- 每个文件仍然沿用同一个五字段 schema；如果某个文件不提供某个字段的内容，请把该字段写成空字符串
- 显式文件按 CLI 输入顺序参与 merge；目录内 regular files 先按词典序展开，再按该顺序参与 merge
- `roleplay_prompt`、`discussion_question`、`output_contract`、`assumptions_and_constraints`：所有文件里最多只能出现一个唯一的非空白值；出现冲突非空白值会直接报错
- `supplementary_materials`：所有非空白值会按输入顺序用 `\n\n` 连接

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
  01_prepare/
  02_answer/
  03_render/
  audit/
```

关键路径：

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
- `audit/runtime_contract_status.json`

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
- 共享存储里的 repo-root `go_bin` 也可能不可执行；`make build` 现在默认写到 `$HOME/worldview-panel_bin`，也可以显式覆盖 `BIN_PATH=/abs/path/to/worldview-panel`
- 这种情况下，需要先把仓库镜像到本地可锁目录，再运行 Go 命令

一个可用做法是：

```sh
tmpdir=$(mktemp -d /data/data/com.termux/files/home/view_panel_test_XXXXXX) && \
tar -cf - --exclude=.git --exclude=out --exclude=runs --exclude=content/build --exclude=go_bin . | (cd "$tmpdir" && tar -xf -) && \
cd "$tmpdir" && \
go test ./...
```

## 已知限制

- `third_party/codex-protocol/0.0.0/` 现在是针对 pinned CLI `codex-cli 0.0.0` 的 checked-in live captured bundle，不再是 placeholder；但它只对该 pinned 版本提供静态证据，版本或协议变更时仍要用 `make capture-protocol` 重新捕获。
- app-server runtime 已支持 `Content-Length` framed stdio，并在未命中 framed 头时回退到 sequential JSON；但 checked-in `smoke-transcript.jsonl` 目前只覆盖已观测到的最小 sequential fallback capture，而 startup audit 当前也会继续记录“尚未独立验证 framing”的 warning facts。
- 多个 `--materials` 输入的 canonical 五字段 merge contract 已实现并在 startup/prepare 中生效；但它只覆盖 `roleplay_prompt`、`discussion_question`、`supplementary_materials`、`output_contract`、`assumptions_and_constraints`，不提供更通用的文档合并语义。
## 开发说明

如果你是人类开发者，建议先读：

1. [README.md](/storage/emulated/0/projects/view_panel/README.md)
2. [codex-runtime-contract.md](/storage/emulated/0/projects/view_panel/docs/operations/codex-runtime-contract.md)
3. `internal/storage/` 和 `internal/hash/` 的共享 contract
4. `internal/stage/*` 下各阶段实现

如果你是通过 Codex 继续维护这个仓库，根目录 [AGENTS.md](/storage/emulated/0/projects/view_panel/AGENTS.md) 应该是你的第一入口。
