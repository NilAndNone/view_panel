可以。下面这个方案我直接按 Go 主编排 + Codex app-server + 三阶段独立 fresh session 来定，而且是能直接开工的版本。

这个方案只保留你现在真正不能退的几条硬约束：
dispatch_input_v1 单源、Stage 2 只能消费 Stage 1 的正式产物、实际发送内容单独落盘、主审计目录在 run_root/audit/。 这正好对准事故报告里指出的根因：准备输入和实际派发走了两条路径，真正发出去的内容不是准备好的那一份，而且日志也不够闭合。  

先给结论：

v1 定案

1. ./go_bin -question "..." 是唯一入口。


2. 每个阶段都跑新的 Codex 会话；在 app-server 语义里，会话核心原语是 thread，所以 Stage 1 一个新 thread，Stage 2 每个人格一个新 thread，Stage 3 一个新 thread。app-server 默认走 JSONL over stdio；客户端必须先 initialize，再发 initialized，然后 thread/start、turn/start。


3. v1 不做一个长驻共享 broker 进程。 我建议 一个工作单元一个 app-server 子进程：Stage 1 一个子进程，Stage 2 每个人格一个子进程，Stage 3 一个子进程。理由有两个：一是 thread 是会话单位，二是 app-server 的 per-turn 设置会成为同一 thread 后续 turn 的默认值，所以“一线程一回合”最干净，最不容易串上下文。


4. 人格用 AGENTS.md，阶段流程用 Skill，问题和材料用 user text。 Codex 会在开始工作前读取 AGENTS.md；skills 则是带 SKILL.md 的目录，显式调用时用 $<skill-name> 加 skill input item，官方还特别建议带 skill item，这样服务器会直接注入完整 skill 指令，避免模型自己解析名字带来的额外延迟。


5. 用 stdio，不用 WebSocket。 官方文档里 codex app-server --listen stdio:// 保持默认 JSONL-over-stdio；ws:// 是 experimental，主要用于开发/测试。CLI 文档还明确写了 app-server 主要用于开发和调试，可能会变化，所以要做薄适配层 + 版本锁定。




---

一、整体形态

运行次数

一次完整运行包含：

Stage 1：1 个 app-server 子进程，1 个 thread，1 个 turn

Stage 2：24 个 app-server 子进程，24 个 thread，24 个 turn

Stage 3：1 个 app-server 子进程，1 个 thread，1 个 turn


合计 26 个独立 thread。这跟你之前想要的“每个阶段新 session、Stage 2 各人格独立”是一致的，而且和现有设计文档里“worker / synthesizer 都由 app-server runtime 驱动”的方向也一致。 

目录

我建议直接把运行目录做成：

runs/<run_id>/
  00_request/
      request.json

        01_prepare/
            manifest.json
                prepare_review.json
                    prepare_status.json
                        personas/
                              <persona_id>/
                                      dispatch_input_v1.json
                                              agents.md
                                                      prompt.txt
                                                              hashes.json

                                                                02_answer/
                                                                    personas/
                                                                          <persona_id>/
                                                                                  workspace/
                                                                                            AGENTS.md
                                                                                                    outgoing_input.json
                                                                                                            protocol.out.jsonl
                                                                                                                    protocol.in.jsonl
                                                                                                                            events.jsonl
                                                                                                                                    result.raw.txt
                                                                                                                                            result.json
                                                                                                                                                    attestation.json
                                                                                                                                                            status.json

                                                                                                                                                              03_render/
                                                                                                                                                                  render_input.json
                                                                                                                                                                      panel.json
                                                                                                                                                                          panel.md
                                                                                                                                                                              cards.json
                                                                                                                                                                                  protocol.out.jsonl
                                                                                                                                                                                      protocol.in.jsonl
                                                                                                                                                                                          events.jsonl
                                                                                                                                                                                              status.json

                                                                                                                                                                                                audit/
                                                                                                                                                                                                    events.jsonl
                                                                                                                                                                                                        run_summary.json

                                                                                                                                                                                                          run_index.json

                                                                                                                                                                                                          主证据永远看 runs/<run_id>/...，不要把 ~/.codex/log/... 当主诊断来源。 事故报告已经证明旧日志可能不是本轮日志；你自己的设计文档也已经把 global log 定成了 locator only，而不是诊断主源。 


                                                                                                                                                                                                          ---

                                                                                                                                                                                                          二、三阶段分别干什么

                                                                                                                                                                                                          Stage 1：Prepare

                                                                                                                                                                                                          目标

                                                                                                                                                                                                          把 24 个人格和公共材料确定性组装成唯一合法的 dispatch_input_v1，并提前渲染出 真正会被发出去的 agents.md 和 prompt.txt。
                                                                                                                                                                                                          这一步结束后，Stage 2 不再允许任何重组装。

                                                                                                                                                                                                          Go 负责

                                                                                                                                                                                                          1. 读取：

                                                                                                                                                                                                          question

                                                                                                                                                                                                          materials/

                                                                                                                                                                                                          runtime/personas/

                                                                                                                                                                                                          runtime/persona-index.json



                                                                                                                                                                                                          2. 为每个人格生成 dispatch_input_v1.json，至少包含事故报告要求的 5 个字段：

                                                                                                                                                                                                          roleplay_prompt

                                                                                                                                                                                                          discussion_question

                                                                                                                                                                                                          supplementary_materials

                                                                                                                                                                                                          output_contract

                                                                                                                                                                                                          assumptions_and_constraints  



                                                                                                                                                                                                          3. 把 dispatch_input_v1 预渲染成两份最终发送载体：

                                                                                                                                                                                                          agents.md ← roleplay_prompt

                                                                                                                                                                                                          prompt.txt ← 其余 4 部分拼成最终 user text



                                                                                                                                                                                                          4. 计算并落盘：

                                                                                                                                                                                                          dispatch_input_sha256

                                                                                                                                                                                                          agents_sha256

                                                                                                                                                                                                          prompt_sha256

                                                                                                                                                                                                          bundle_sha256 = sha256(agents.md bytes || prompt.txt bytes || stage skill version)



                                                                                                                                                                                                          5. 写 manifest.json



                                                                                                                                                                                                          Codex 在 Stage 1 干什么

                                                                                                                                                                                                          只做 review，不做 authoring。

                                                                                                                                                                                                          也就是 Stage 1 仍然跑一个新的 app-server session，但这个 session 的职责只是：

                                                                                                                                                                                                          检查 24 份输入是否都齐全

                                                                                                                                                                                                          检查人格差异是否过弱

                                                                                                                                                                                                          检查输出契约是否冲突

                                                                                                                                                                                                          生成 prepare_review.json


                                                                                                                                                                                                          它不能改写 canonical packet。

                                                                                                                                                                                                          为什么这样拆

                                                                                                                                                                                                          因为这次事故的根因不是“材料没准备出来”，而是“准备出来之后，派发时又走了另一条临场拼接路径”。所以 Stage 1 的权威必须在 Go，而不是让模型临时再拼一次。


                                                                                                                                                                                                          ---

                                                                                                                                                                                                          Stage 2：Answer

                                                                                                                                                                                                          目标

                                                                                                                                                                                                          对每个人格启动一个全新 Codex worker，会话只接收：

                                                                                                                                                                                                          该人格的 AGENTS.md

                                                                                                                                                                                                          该人格的 prompt.txt

                                                                                                                                                                                                          该阶段的 wv-answer-stage skill


                                                                                                                                                                                                          然后输出结构化 worker_result_v1。

                                                                                                                                                                                                          Go 负责

                                                                                                                                                                                                          对每个人格：

                                                                                                                                                                                                          1. 从 01_prepare/personas/<id>/ 读取预渲染好的：

                                                                                                                                                                                                          agents.md

                                                                                                                                                                                                          prompt.txt

                                                                                                                                                                                                          hashes.json



                                                                                                                                                                                                          2. 创建隔离 workspace：

                                                                                                                                                                                                          runs/<run_id>/02_answer/personas/<id>/workspace/AGENTS.md



                                                                                                                                                                                                          3. 写 outgoing_input.json，记录：

                                                                                                                                                                                                          persona_id

                                                                                                                                                                                                          agents_path

                                                                                                                                                                                                          prompt_path

                                                                                                                                                                                                          skill_name

                                                                                                                                                                                                          skill_path

                                                                                                                                                                                                          agents_hash

                                                                                                                                                                                                          prompt_hash

                                                                                                                                                                                                          skill_hash

                                                                                                                                                                                                          combined_hash



                                                                                                                                                                                                          4. 启动一个 新的 codex app-server --listen stdio://


                                                                                                                                                                                                          5. 做握手：

                                                                                                                                                                                                          initialize

                                                                                                                                                                                                          initialized



                                                                                                                                                                                                          6. 立刻调用 configRequirements/read，读出当前环境允许的 approvalPolicy 和 sandboxMode；官方示例里会返回 allowedApprovalPolicies、allowedSandboxModes、网络要求等。


                                                                                                                                                                                                          7. thread/start


                                                                                                                                                                                                          8. turn/start

                                                                                                                                                                                                          cwd 指向 persona workspace

                                                                                                                                                                                                          outputSchema 指向 worldview_worker_result_v1

                                                                                                                                                                                                          input 只放两项：

                                                                                                                                                                                                          text: "$wv-answer-stage\n" + <prompt.txt 全文>

                                                                                                                                                                                                          skill: {"type":"skill","name":"wv-answer-stage","path":".../SKILL.md"}
                                                                                                                                                                                                          显式 skill 注入是官方推荐方式。




                                                                                                                                                                                                          9. 持续读取通知，直到 turn/completed


                                                                                                                                                                                                          10. 从 item/completed 里的最终 agentMessage 取结果文本；官方文档明确说 item/completed 是权威最终状态，item/agentMessage/delta 只是流式增量。


                                                                                                                                                                                                          11. 把结果 JSON 解析并再用本地 schema 校验


                                                                                                                                                                                                          12. 写：



                                                                                                                                                                                                          result.raw.txt

                                                                                                                                                                                                          result.json

                                                                                                                                                                                                          attestation.json

                                                                                                                                                                                                          status.json


                                                                                                                                                                                                          Stage 2 的硬限制

                                                                                                                                                                                                          这一步是整个系统最关键的一层，必须强约束：

                                                                                                                                                                                                          1）禁止重渲染
                                                                                                                                                                                                          Stage 2 不能再根据 dispatch_input_v1 临时拼 prompt。
                                                                                                                                                                                                          它只能发送 Stage 1 已经写好的 agents.md 和 prompt.txt。
                                                                                                                                                                                                          这条就是事故整改的核心。

                                                                                                                                                                                                          2）禁止上下文延续
                                                                                                                                                                                                          不允许：

                                                                                                                                                                                                          thread/resume

                                                                                                                                                                                                          thread/fork

                                                                                                                                                                                                          turn/steer


                                                                                                                                                                                                          因为你要的是各人格独立样本，而不是连续对话。官方文档里这些接口都存在，但 turn/steer 本质是给当前 in-flight turn 追加输入，不适合这里。

                                                                                                                                                                                                          3）禁止工具偷懒
                                                                                                                                                                                                          app-server 的 item 类型里有：

                                                                                                                                                                                                          commandExecution

                                                                                                                                                                                                          fileChange

                                                                                                                                                                                                          mcpToolCall

                                                                                                                                                                                                          dynamicToolCall

                                                                                                                                                                                                          collabToolCall

                                                                                                                                                                                                          webSearch

                                                                                                                                                                                                          imageView
                                                                                                                                                                                                          你的 Stage 2 是内容生成，不是跑工具。
                                                                                                                                                                                                          所以我建议 白名单 只允许：

                                                                                                                                                                                                          userMessage

                                                                                                                                                                                                          agentMessage

                                                                                                                                                                                                          reasoning

                                                                                                                                                                                                          plan
                                                                                                                                                                                                          只要出现任何工具类 item，直接判该 persona FAILED。官方文档也明确说 item/completed 是最终权威状态，命令/文件变更若被审批或拒绝都会反映在 item 生命周期里。


                                                                                                                                                                                                          Stage 2 的输出结构

                                                                                                                                                                                                          这里直接沿用你设计文档里已经很好的 worker schema：

                                                                                                                                                                                                          judgment.factual

                                                                                                                                                                                                          judgment.value

                                                                                                                                                                                                          judgment.strategy

                                                                                                                                                                                                          diagnosis[]

                                                                                                                                                                                                          recommended_actions[]

                                                                                                                                                                                                          voice_style

                                                                                                                                                                                                          blind_spot

                                                                                                                                                                                                          overuse_risk

                                                                                                                                                                                                          signature_line

                                                                                                                                                                                                          confidence 


                                                                                                                                                                                                          这样你的人格差异会更清楚，前端也更好展示。


                                                                                                                                                                                                          ---

                                                                                                                                                                                                          Stage 3：Render

                                                                                                                                                                                                          目标

                                                                                                                                                                                                          读取 Stage 2 的结构化答案，做汇总和前端展示产物。

                                                                                                                                                                                                          Go 负责

                                                                                                                                                                                                          1. 扫描 02_answer/personas/*/status.json


                                                                                                                                                                                                          2. 只收集 schema_valid=true 且 forbidden_tool_use=false 的结果


                                                                                                                                                                                                          3. 组装 render_input.json



                                                                                                                                                                                                          Codex 负责

                                                                                                                                                                                                          启动一个全新 app-server session，显式调用 wv-render-stage skill，输出：

                                                                                                                                                                                                          panel.json

                                                                                                                                                                                                          panel.md

                                                                                                                                                                                                          cards.json


                                                                                                                                                                                                          Render 的限制

                                                                                                                                                                                                          不能回写 Stage 2 的人格答案

                                                                                                                                                                                                          不能替失败人格补答案

                                                                                                                                                                                                          只能汇总、聚类、对比、生成展示层


                                                                                                                                                                                                          成功率阈值

                                                                                                                                                                                                          你的设计文档当前锁的是 80% certified success rate 才允许 synthesis。 
                                                                                                                                                                                                          我建议 Go 配置里把它做成：

                                                                                                                                                                                                          min_success_rate: 0.8

                                                                                                                                                                                                          默认按 0.8 跑，后面再调。


                                                                                                                                                                                                          ---

                                                                                                                                                                                                          三、Skill 和 AGENTS 怎么分工

                                                                                                                                                                                                          这部分我建议固定成：

                                                                                                                                                                                                          1. AGENTS.md

                                                                                                                                                                                                          只放人格系统信息：

                                                                                                                                                                                                          你是谁

                                                                                                                                                                                                          你的价值偏好

                                                                                                                                                                                                          你的视角和盲区

                                                                                                                                                                                                          你的说话风格

                                                                                                                                                                                                          你在回答时优先关注什么


                                                                                                                                                                                                          官方文档说明 Codex 会在开始工作前读取 AGENTS.md，并且会从全局到项目目录层层合并。

                                                                                                                                                                                                          所以运行时一定要注意两件事：

                                                                                                                                                                                                          Stage 2 的 workspace 不要放在主 repo 里，避免 repo 根目录 AGENTS.md 污染人格会话。

                                                                                                                                                                                                          运行机器最好用专门的 runner 账号，确保 ~/.codex/AGENTS.md 是空的或受控的。
                                                                                                                                                                                                          这个要求不是多余，而是为了防止隐藏系统提示把人格搞乱。这个结论来自官方的 AGENTS 发现顺序。


                                                                                                                                                                                                          2. Skill

                                                                                                                                                                                                          只放阶段流程：

                                                                                                                                                                                                          wv-prepare-stage

                                                                                                                                                                                                          wv-answer-stage

                                                                                                                                                                                                          wv-render-stage


                                                                                                                                                                                                          官方把 skill 定义成“带 SKILL.md 的目录，可附带脚本/资源”，并且 full SKILL.md 只会在 skill 被用到时加载。v1 我建议全都做成 instruction-only skill，不带脚本。

                                                                                                                                                                                                          3. user text

                                                                                                                                                                                                          只放这次任务数据：

                                                                                                                                                                                                          问题

                                                                                                                                                                                                          补充材料

                                                                                                                                                                                                          统一约束

                                                                                                                                                                                                          输出契约


                                                                                                                                                                                                          这正是你最初想要的“system 定人格，user 发问题和材料”。


                                                                                                                                                                                                          ---

                                                                                                                                                                                                          四、app-server 客户端怎么写

                                                                                                                                                                                                          我建议你在 Go 里只写一个很薄的协议层：

                                                                                                                                                                                                          type Client struct {
                                                                                                                                                                                                              cmd    *exec.Cmd
                                                                                                                                                                                                                  stdin  io.WriteCloser
                                                                                                                                                                                                                      lines  *bufio.Scanner
                                                                                                                                                                                                                          nextID atomic.Int64
                                                                                                                                                                                                                          }

                                                                                                                                                                                                                          func NewClient(ctx context.Context, workdir string) (*Client, error)
                                                                                                                                                                                                                          func (c *Client) Initialize(ctx context.Context) error
                                                                                                                                                                                                                          func (c *Client) ReadRequirements(ctx context.Context) (*Requirements, error)
                                                                                                                                                                                                                          func (c *Client) StartThread(ctx context.Context, req ThreadStartRequest) (*ThreadStartResult, error)
                                                                                                                                                                                                                          func (c *Client) RunSingleTurn(ctx context.Context, req SingleTurnRequest) (*TurnArtifacts, error)
                                                                                                                                                                                                                          func (c *Client) Close() error

                                                                                                                                                                                                                          传输层约束

                                                                                                                                                                                                                          官方文档说明 app-server 消息是 JSON-RPC 风格：

                                                                                                                                                                                                                          request：method + params + id

                                                                                                                                                                                                                          response：id + result/error

                                                                                                                                                                                                                          notification：没有 id，只有 method + params。


                                                                                                                                                                                                                          v1 的实现建议

                                                                                                                                                                                                                          不要做复杂多路复用。
                                                                                                                                                                                                                          因为你是“一个进程一个 thread 一个 turn”，所以协议循环可以非常简单：

                                                                                                                                                                                                                          1. 发 initialize


                                                                                                                                                                                                                          2. 发 initialized


                                                                                                                                                                                                                          3. 发 configRequirements/read


                                                                                                                                                                                                                          4. 发 thread/start


                                                                                                                                                                                                                          5. 发 turn/start


                                                                                                                                                                                                                          6. 循环读消息直到 turn/completed


                                                                                                                                                                                                                          7. 解析最终 item/completed.agentMessage


                                                                                                                                                                                                                          8. 关闭进程



                                                                                                                                                                                                                          这比一个共享 app-server 上 multiplex 24 个 thread 简单很多，也更稳。

                                                                                                                                                                                                                          失败处理

                                                                                                                                                                                                                          超时：先发 turn/interrupt，再等短时间，再 kill 进程。官方文档有 turn/interrupt，成功后 turn 会以 interrupted 结束。

                                                                                                                                                                                                                          收到 error 事件：把 message、codexErrorInfo、additionalDetails 全部落盘。官方文档说明 turn fail 时会有 error 事件，且可能带 httpStatusCode。



                                                                                                                                                                                                                          ---

                                                                                                                                                                                                                          五、日志怎么做才算“完备”

                                                                                                                                                                                                                          这里不要省。

                                                                                                                                                                                                                          你的 incident 已经证明：看旧全局日志没有意义。所以日志必须分三层。

                                                                                                                                                                                                                          第一层：原始协议流

                                                                                                                                                                                                                          每个 stage / persona 都保留：

                                                                                                                                                                                                                          protocol.out.jsonl：Go 发给 app-server 的每一条 JSON

                                                                                                                                                                                                                          protocol.in.jsonl：app-server 回来的每一条 JSON


                                                                                                                                                                                                                          不要压缩，不要覆盖。

                                                                                                                                                                                                                          第二层：归一化事件

                                                                                                                                                                                                                          Go 再生成：

                                                                                                                                                                                                                          events.jsonl


                                                                                                                                                                                                                          每条至少有：

                                                                                                                                                                                                                          run_id

                                                                                                                                                                                                                          stage

                                                                                                                                                                                                                          persona_id

                                                                                                                                                                                                                          ts

                                                                                                                                                                                                                          event

                                                                                                                                                                                                                          thread_id

                                                                                                                                                                                                                          turn_id

                                                                                                                                                                                                                          status

                                                                                                                                                                                                                          agents_hash

                                                                                                                                                                                                                          prompt_hash

                                                                                                                                                                                                                          skill_hash

                                                                                                                                                                                                                          combined_hash

                                                                                                                                                                                                                          schema_version

                                                                                                                                                                                                                          duration_ms

                                                                                                                                                                                                                          error_code

                                                                                                                                                                                                                          artifact_path


                                                                                                                                                                                                                          第三层：高层摘要

                                                                                                                                                                                                                          prepare_status.json

                                                                                                                                                                                                                          status.json

                                                                                                                                                                                                                          run_summary.json


                                                                                                                                                                                                                          全局索引

                                                                                                                                                                                                                          保留一个轻量：

                                                                                                                                                                                                                          ~/.codex/log/worldview-panel-go/run_index.jsonl

                                                                                                                                                                                                                          只记录：

                                                                                                                                                                                                                          run_id

                                                                                                                                                                                                                          run_root

                                                                                                                                                                                                                          start_at

                                                                                                                                                                                                                          end_at

                                                                                                                                                                                                                          status


                                                                                                                                                                                                                          这和你现有 broker 设计里“global index 只是定位器，完整诊断回 round_root/audit/”是同一思路。

                                                                                                                                                                                                                          另外一个关键点

                                                                                                                                                                                                                          虽然 app-server 自己也会持久化 thread log，官方文档甚至写了 thread/archive 会把 persisted thread log 作为 JSONL 文件归档，但你这里不要把它当主证据，最多当辅助证据。
                                                                                                                                                                                                                          主证据必须是你自己的 run_root。


                                                                                                                                                                                                                          ---

                                                                                                                                                                                                                          六、Go 工程怎么拆

                                                                                                                                                                                                                          我建议这样：

                                                                                                                                                                                                                          cmd/worldview-panel/main.go

                                                                                                                                                                                                                          internal/appserver/client.go
                                                                                                                                                                                                                          internal/appserver/messages.go
                                                                                                                                                                                                                          internal/appserver/runner.go

                                                                                                                                                                                                                          internal/orchestrator/run.go

                                                                                                                                                                                                                          internal/stage/prepare/prepare.go
                                                                                                                                                                                                                          internal/stage/answer/answer.go
                                                                                                                                                                                                                          internal/stage/render/render.go

                                                                                                                                                                                                                          internal/audit/logger.go
                                                                                                                                                                                                                          internal/audit/summary.go

                                                                                                                                                                                                                          internal/schema/validate.go
                                                                                                                                                                                                                          internal/schema/types.go

                                                                                                                                                                                                                          internal/persona/library.go
                                                                                                                                                                                                                          internal/materials/loader.go
                                                                                                                                                                                                                          internal/storage/layout.go
                                                                                                                                                                                                                          internal/hash/hash.go

                                                                                                                                                                                                                          runtime/skills/wv-prepare-stage/SKILL.md
                                                                                                                                                                                                                          runtime/skills/wv-answer-stage/SKILL.md
                                                                                                                                                                                                                          runtime/skills/wv-render-stage/SKILL.md

                                                                                                                                                                                                                          runtime/personas/
                                                                                                                                                                                                                          runtime/persona-index.json

                                                                                                                                                                                                                          schemas/dispatch_input_v1.json
                                                                                                                                                                                                                          schemas/prepare_review_v1.json
                                                                                                                                                                                                                          schemas/worldview_worker_result_v1.json
                                                                                                                                                                                                                          schemas/panel_result_v1.json
                                                                                                                                                                                                                          schemas/audit_event_v1.json

                                                                                                                                                                                                                          主流程

                                                                                                                                                                                                                          func main() {
                                                                                                                                                                                                                              req := parseFlags()
                                                                                                                                                                                                                                  run := orchestrator.NewRun(req)

                                                                                                                                                                                                                                      prep := prepare.Run(run)
                                                                                                                                                                                                                                          answers := answer.Run(run, prep)
                                                                                                                                                                                                                                              panel := render.Run(run, prep, answers)

                                                                                                                                                                                                                                                  audit.WriteRunSummary(run, prep, answers, panel)
                                                                                                                                                                                                                                                  }


                                                                                                                                                                                                                                                  ---

                                                                                                                                                                                                                                                  七、直接执行步骤

                                                                                                                                                                                                                                                  1）安装和鉴权

                                                                                                                                                                                                                                                  官方 CLI 文档给的安装方式是：

                                                                                                                                                                                                                                                  npm i -g @openai/codex

                                                                                                                                                                                                                                                  第一次运行 codex 会提示登录；CLI 参考页也有 codex login。支持 ChatGPT 账号或 API key。CLI 当前支持 macOS 和 Linux，Windows 仍属 experimental。

                                                                                                                                                                                                                                                  2）锁版本并生成协议 schema

                                                                                                                                                                                                                                                  因为 app-server 官方自己写明“主要用于开发和调试，可能变化”，而且 generate-json-schema 产出的 bundle 是和你运行的 Codex 版本严格对应的，所以建议把 CLI 版本锁住，然后执行：

                                                                                                                                                                                                                                                  codex app-server generate-json-schema --out ./third_party/codex-protocol



                                                                                                                                                                                                                                                  3）跑 app-server 冒烟测试

                                                                                                                                                                                                                                                  codex debug app-server send-message-v2 "healthcheck"

                                                                                                                                                                                                                                                  这个命令就是官方给的 V2 thread/turn 调试流。

                                                                                                                                                                                                                                                  4）编译 Go

                                                                                                                                                                                                                                                  go build -o go_bin ./cmd/worldview-panel

                                                                                                                                                                                                                                                  5）执行

                                                                                                                                                                                                                                                  ./go_bin \
                                                                                                                                                                                                                                                    -question "xxx讨论" \
                                                                                                                                                                                                                                                      -materials ./materials/topic_x \
                                                                                                                                                                                                                                                        -persona-set ./runtime/persona-index.json \
                                                                                                                                                                                                                                                          -out ./runs \
                                                                                                                                                                                                                                                            -concurrency 6 \
                                                                                                                                                                                                                                                              -model gpt-5.4

                                                                                                                                                                                                                                                              gpt-5.4 这里按官方 app-server 示例写成默认值；最终用哪个模型你可以做成配置项。


                                                                                                                                                                                                                                                              ---

                                                                                                                                                                                                                                                              八、最重要的 8 条验收标准

                                                                                                                                                                                                                                                              1. Stage 2 没有 01_prepare/.../agents.md 和 prompt.txt 就不能运行。


                                                                                                                                                                                                                                                              2. Stage 2 不能从 dispatch_input_v1.json 临时重渲染。


                                                                                                                                                                                                                                                              3. 每个人格都必须有 outgoing_input.json 和 hash。


                                                                                                                                                                                                                                                              4. protocol.out.jsonl / protocol.in.jsonl 缺任何一个都算运行不完整。


                                                                                                                                                                                                                                                              5. 结果只从 item/completed 的最终 agentMessage 取。


                                                                                                                                                                                                                                                              6. 只要出现工具型 item，就判该人格失败。


                                                                                                                                                                                                                                                              7. Render 只能读 Stage 2 的结构化结果，不能脑补缺失人格。


                                                                                                                                                                                                                                                              8. run_root/audit/ 不完整时，这轮 run 直接标 FAILED。这条和事故报告“旧日志不能当主证据”的教训是同一个方向。




                                                                                                                                                                                                                                                              ---

                                                                                                                                                                                                                                                              九、一句话定版

                                                                                                                                                                                                                                                              直接做：Go orchestrator + 3 个 stage skills + 每个工作单元一个 app-server 子进程 + Stage 1 预渲染 AGENTS.md/prompt.txt + Stage 2 只发预渲染产物 + run_root 完整审计。

                                                                                                                                                                                                                                                              这套最接近你要的“稳定先跑起来、重点放内容差异、别在流程壳上浪费时间”，同时又把这次事故里最要命的双路径问题一次性钉死。 

                                                                                                                                                                                                                                                              下一步最合适的是直接把 internal/appserver/client.go 和 internal/stage/answer/answer.go 的骨架写出来。
