# Semantic Retrieval Tools (Serena Parity) for Crush

## 背景 / 需求

- 目标：把 Serena 的核心“语义检索”能力（基于 LSP 的符号树、引用查询、定位精确代码片段）移植到 Crush，作为高级工具提供给代理调用。
- 现状：Crush 仅有 `lsprefs`/`lsp_diagnostics` 等基础 LSP 工具；Serena 已有完整的符号检索/编辑工具栈（`find_symbol`、`find_referencing_symbols`、`symbol_overview` 等），并通过 `LanguageServerSymbolRetriever` 和 `CodeEditor` 组合实现。
- 约束：Crush 已有 powernap LSP 客户端，Fantasy 工具框架，LSP 已可启动；需要新增 DocumentSymbol/rename 等调用与符号建模层。

## 方案评审（对先前初稿的补充与优化）

- 保留：建设符号抽象层（name_path matcher、kind 过滤、按目录/文件限制搜索、可选 body 载入、引用查询），在工具层暴露只读检索（优先）和可选编辑能力。
- 优化点：
  - **增量 rollout**：优先落地只读三件套（overview/find_symbol/find_referencing_symbols），编辑类（rename/insert/replace）放第二阶段，避免早期破坏性写入风险。
  - **LSP 能力探测**：不同语言服务器 DocumentSymbol/References 质量不一，先在工具输出中标注“provider 可能不支持/结果为空”，并在客户端层检测 capabilities 决定是否降级。
  - **性能与范围控制**：在检索层加入 `max_files`/`max_depth`/`max_results` 与 ignored patterns（复用现有 LSP fileTypes 与项目 `ignore` 逻辑），并对 body 读取做大小截断，避免大仓库全量遍历超时。
  - **结果去重/排序**：按路径+range 排序、去重，优先返回用户指定 path 下的结果，再返回全局匹配，提升可读性。
  - **缓存策略**：可选基于 mtime 的轻量缓存（文件 → symbol tree）避免高频 DocumentSymbol 请求；需在保存/写入后失效缓存或监听 LSP 文件变更。
  - **line/offset 一致性**：Crush/powernap API 使用 0-based，文案/输出使用 1-based；在工具层统一转换，避免调用方混淆。
  - **风险提示**：编辑类操作需提示“依赖 LSP rename/applyEdit 支持；部分语言（如 JS/TS 混合）易出现重命名范围不足/过度”；缺省不暴露编辑工具或要求显示开关。

## 技术方案

1) **LSP 客户端扩展（powernap 封装）**
   - 新增调用：`DocumentSymbols(ctx, path)`（textDocument/documentSymbol，支持 DocumentSymbolResult union）；可选 `Definition`、`PrepareRename`、`Rename` 为编辑阶段预留。
   - 复用/加强：`FindReferences` 继续使用，调用前确保 `OpenFileOnDemand`; offset/line 0-based。
2) **符号建模与检索层（建议新包 `internal/lspsymbols` 或 `internal/lsp/symbols`）**
   - 结构：`Symbol{ NamePath, Kind, RelativePath, SelectionRange, BodyRange, Children, Body? }`。
   - `NamePathMatcher`：绝对/相对/substring/overload 索引匹配规则与 Serena 对齐。
   - `SymbolRetriever`：
     - 按文件/目录获取 DocumentSymbol 树 → 过滤 ignore patterns → matcher 过滤 → 深度裁剪。
     - `include_body`: 用 BodyRange 读取文件切片，带字符上限。
     - `SymbolOverview`: 提取顶层符号（name_path, kind, body_location）。
     - `FindReferencingSymbols`: 先匹配唯一符号 → selectionRange line/char → LSP references → 附加引用附近行内容；过滤 include/exclude kinds。
   - 结果排序/去重：按 path → line → char。
   - 可选缓存：文件 mtime + hash → 已解析符号树；文件写入后失效。
3) **工具层（Fantasy AgentTool）**
   - `lspoverview`：参数 `path`, `max_answer_chars`；返回顶层符号 JSON。
   - `lspfind`：参数 `name_path_pattern`, `path`(文件/目录), `depth`, `include_body`, `include_kinds`/`exclude_kinds`, `substring`, `max_answer_chars`。
   - `lsprefs`：参数 `name_path`, `path`, `include_kinds`/`exclude_kinds`, `max_answer_chars`；结果包含引用上下文。
   - 输出：有序 JSON，字段包含 `relative_path`, `kind`, `body_location`, `body?`, `children?`，长度超限给出提示。
   - 权限/安全：仍走现有 workingDir 与权限校验；对 body 读取加入大小限制。
4) **编辑能力（第二阶段，可选）**
   - `lsprename`：调用 LSP rename，前置 `PrepareRename` 检查；需要 confirm 机制。
   - `lspreplace`/`lspinsertbefore`/`lspinsertafter`: 基于 symbol range 读写文件；写入后通知 LSP/失效缓存。
   - 默认关闭，可通过配置/flag 启用。

## 开发计划

- Phase 0：能力验证
  - 调研 powernap 对 documentSymbol/rename 支持（API/常量齐全度）；小仓运行冒烟（Go/Python/TS）。
  - 明确默认 `max_answer_chars`、`max_results`、忽略规则。
- Phase 1：只读工具落地
  - 实现 DocumentSymbols 包装 + `SymbolRetriever`/matcher/排序/截断。
  - 新增工具 `lspoverview` / `lspfind` / `lsprefs`，集成注册到工具列表；文档与系统 prompt 更新。
  - 基础测试：unit（matcher、排序、截断）、集成（对示例仓库生成稳定输出）。
- Phase 2：性能与稳健性
  - 加入 mtime 缓存、忽略规则、结果上限；错误提示和 capability 探测。
  - 大仓库基准测试，调优默认上限。
- Phase 3（可选）：编辑类能力
  - 实现 rename/replace/insert；添加保护: dry-run/预览、确认开关。
  - 回归测试（多语言）、恢复/撤销策略评估。

## 风险与缓解

- DocumentSymbol 质量不齐 / 返回空：输出显式提示，提供 `path` 限定与结果上限，必要时 fallback 使用 `rg`。
- 大型仓库性能：目录过滤、结果/文件上限、缓存；按需 lazy load。
- 多语言/多 LSP 场景：`HandlesFile` 选择 client；capability 探测决定是否启用工具或降级。
- 编辑操作风险：默认关闭；使用前置校验、dry-run、undo 提示。

## 待决问题

- 默认输出上限（字符/结果/文件数）取值？
- 是否需要用户级开关决定是否启用编辑工具？
- 缓存策略是内存型还是磁盘持久？失效粒度如何与 LSP change 事件对齐？

## 当前实施快照

- 默认输出上限已对齐 Serena：`max_answer_chars = 150000`，调用参数未显式传入时使用该默认值。
- 首版缓存：基于 mtime/size 的内存缓存（每文件 DocumentSymbol 结果），暂未落盘；文件内容变化后会失效。
- 新增只读工具：`lspoverview`、`lspfind`、`lsprefs`，依赖 LSP，可在 Agent 层单独启用/选择使用，输出 JSON（name_path、kind、relative_path、body_location、可选 body/children）。`lspfind` 支持 max_results + truncated；`lsprefs` 返回“引用它的符号”（附上下文片段），支持 include_imports/include_self/max_results，并默认排除声明。
- 新增编辑工具：`lspreplace`、`lspinsertbefore`、`lspinsertafter`、`lsprename`，默认加入工具列表（非只读）；基于 LSP range 进行定位/修改，插入/替换时保留最少的空行习惯。
- LSP 能力失败时（documentSymbol/references/rename）会返回“方法不支持”提示，便于上层改用 grep/rg 或降级方案。

## 已知未对齐 / TODO

- 引用类型判定仍为启发式（基于行首/语言），未消费服务器返回的引用分类；如需要更精确的 import 过滤需服务器提供 richer metadata。
- `workspace/symbol` 路径收集默认启用，但未做能力探测/分页；若服务器不支持则回退 glob。
- 重载签名匹配已使用 signatureHelp/detail，但未做跨语言的 Definition/SignatureHelp 组合 disambiguation；需特定语言扩展时再补。
