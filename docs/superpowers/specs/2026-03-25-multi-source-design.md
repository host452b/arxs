# arxs — 多源论文搜索重构设计文档

**日期**：2026-03-25
**版本**：v2.0 设计稿

---

## 概述

将 arxs 从单一 arXiv 来源扩展为五源聚合 CLI 工具。用户通过 `-s` 指定学科（支持 arXiv 精确代码或顶级别名），工具自动查找对应的 top-3 论文源并发搜索，结果分组展示，重复条目保留主源。

---

## 目标源集合

| ProviderID | 名称 | 主导领域 | API |
|-----------|------|---------|-----|
| `arxiv` | arXiv | 物理/数学/CS/AI/econ/q-fin | Atom XML API |
| `zenodo` | Zenodo | 全学科数据/代码档案 | REST JSON API |
| `socarxiv` | SocArXiv | 社会科学/法学 | OSF REST API |
| `edarxiv` | EdArXiv | 教育学 | OSF REST API |
| `openalex` | OpenAlex | 经济/金融/哲学/人文（SSRN+PhilArchive 替代） | REST JSON API |

**移除**：SSRN（无公开 API，Elsevier 闭源）、PhilArchive（无正式 API）。

---

## 目录结构

保持现有结构不变，新增以下包：

```
internal/
  provider/
    provider.go             # Provider interface + SubjectFilter + ProviderID 常量
    arxiv/
      provider.go           # arXiv 实现（复用现有 api/ 逻辑）
      provider_test.go
    zenodo/
      provider.go
      provider_test.go
    socarxiv/
      provider.go           # OSF API，provider=socarxiv
      provider_test.go
    edarxiv/
      provider.go           # OSF API，provider=edarxiv
      provider_test.go
    openalex/
      provider.go
      provider_test.go
  subject/
    registry.go             # arXiv code/alias → []ProviderID + SubjectFilter per provider
    registry_test.go
  orchestrator/
    search.go               # fan-out goroutines + dedup + group by source
    search_test.go
  log/
    log.go                  # 结构化 JSON 日志（stderr，--debug 开启）
  api/                      # 保留，被 arxiv provider 复用
  model/                    # 扩展 Paper 加 Source/SourceURL 字段
  cache/                    # 不变
  store/                    # 扩展支持 MultiSourceResult
  parser/                   # 不变
```

---

## 数据模型变更

### Paper（扩展）

```go
type Paper struct {
    // 现有字段不变
    ID         string   `json:"id"`
    Title      string   `json:"title"`
    Authors    []string `json:"authors"`
    Abstract   string   `json:"abstract"`
    Categories []string `json:"categories"`
    Published  string   `json:"published"`
    Updated    string   `json:"updated"`
    PDFUrl     string   `json:"pdf_url"`
    HTMLUrl    string   `json:"html_url"`
    AbsUrl     string   `json:"abs_url"`
    Citations  int      `json:"citations"`
    // 新增
    Source     string   `json:"source"`      // "arxiv"|"zenodo"|"socarxiv"|"edarxiv"|"openalex"
    SourceURL  string   `json:"source_url"`  // 该源的摘要页 URL
}
```

### MultiSourceResult（新增）

```go
type MultiSourceResult struct {
    Query  QueryMeta   `json:"query"`
    Groups []SourceGroup `json:"groups"`  // 按主源顺序排列
    Total  int         `json:"total"`
}

type SourceGroup struct {
    Source string  `json:"source"`
    Count  int     `json:"count"`
    Papers []Paper `json:"papers"`
}
```

---

## Provider 接口

```go
// internal/provider/provider.go

type ProviderID string

const (
    ProviderArxiv     ProviderID = "arxiv"
    ProviderZenodo    ProviderID = "zenodo"
    ProviderSocArxiv  ProviderID = "socarxiv"
    ProviderEdArxiv   ProviderID = "edarxiv"
    ProviderOpenAlex  ProviderID = "openalex"
)

// SubjectFilter 携带各源自己的学科过滤参数
type SubjectFilter struct {
    ArxivCats        []string // arXiv: ["cs.AI","cs.LG"]
    OpenAlexConcepts []string // OpenAlex concept IDs: ["C41008148"]
    ZenodoKeywords   []string // Zenodo: ["machine learning"]
    OSFProvider      string   // "socarxiv" or "edarxiv"
    OSFSubjects      []string // OSF subject tags
}

type Provider interface {
    ID() ProviderID
    Search(ctx context.Context, q QueryParams, f SubjectFilter) ([]model.Paper, error)
    DownloadPDF(ctx context.Context, paper model.Paper) ([]byte, error)
}
```

---

## Subject Registry

`-s` 接受两种粒度：
- **arXiv 精确代码**：`cs.AI`、`q-fin.TR`、`astro-ph.GA`、`hep-th` 等
- **顶级别名**：`cs`、`physics`、`math`、`economics`、`sociology`、`education`、`philosophy`、`law` 等

多个 `-s` 取 union（OR 语义）：providers 取并集，SubjectFilter 各字段合并。

**部分映射表（registry.go 中完整写死，约 80 条）**：

| `-s` 输入 | 主源顺序（top-3） | arXiv cats | OpenAlex concept ID | Zenodo keywords | OSF |
|-----------|----------------|-----------|--------------------|--------------|----|
| `cs.AI` / `cs` | arxiv › zenodo › socarxiv | `cs.AI` | C154945302 | "artificial intelligence" | Computer and Information Sciences |
| `cs.LG` | arxiv › zenodo › socarxiv | `cs.LG` | C119857082 | "machine learning" | Computer and Information Sciences |
| `cs.CL` | arxiv › zenodo › socarxiv | `cs.CL` | C204321447 | "natural language processing" | Computer and Information Sciences |
| `cs.CV` | arxiv › zenodo › socarxiv | `cs.CV` | C31972630 | "computer vision" | Computer and Information Sciences |
| `cs.CY` | arxiv › socarxiv › zenodo | `cs.CY` | C17744445 | "computers and society" | Social and Behavioral Sciences |
| `physics` / `hep-th` | arxiv › zenodo | `hep-th` | C121332964 | "high energy physics" | — |
| `quant-ph` | arxiv › zenodo | `quant-ph` | C62520636 | "quantum computing" | — |
| `math` | arxiv › zenodo | `math.*` | C33923547 | "mathematics" | — |
| `stat` | arxiv › zenodo | `stat.*` | C161191863 | "statistics" | — |
| `q-fin` | arxiv › openalex › zenodo | `q-fin.*` | C187279774 | "quantitative finance" | — |
| `econ` | arxiv › openalex › zenodo | `econ.*` | C162324750 | "economics" | — |
| `q-bio` | arxiv › zenodo | `q-bio.*` | C86803240 | "quantitative biology" | — |
| `eess` | arxiv › zenodo | `eess.*` | C41008148 | "electrical engineering" | — |
| `sociology` | socarxiv › openalex › zenodo | — | C144024400 | "sociology" | Social and Behavioral Sciences |
| `education` | edarxiv › socarxiv › zenodo | — | C142362112 | "education" | Education |
| `philosophy` | openalex › socarxiv › zenodo | `physics.hist-ph` | C138885662 | "philosophy" | — |
| `law` | socarxiv › openalex › zenodo | — | C18214049 | "law" | Social and Behavioral Sciences |
| `psychology` | socarxiv › edarxiv › zenodo | — | C15744967 | "psychology" | Social and Behavioral Sciences |

---

## 各源 API 实现要点

### arXiv
- 复用现有 `internal/api/` 逻辑，包装为 Provider
- SubjectFilter.ArxivCats → `cat:cs.AI OR cat:cs.LG` 过滤
- 速率：3s（不变）

### Zenodo
- `GET https://zenodo.org/api/records?q={keywords}&type=publication&size={max}`
- 速率：1s（官方 60 req/min）

### SocArXiv / EdArXiv
- `GET https://api.osf.io/v2/preprints/?provider={id}&filter[subjects]={tag}&page[size]={max}`
- JSON:API 格式；作者列表懒加载（仅 `--verbose` 或下载摘要时触发）
- 速率：1s（保守）

### OpenAlex
- `GET https://api.openalex.org/works?filter=concepts.id:{id},is_oa:true&search={keywords}&per_page={max}&mailto=arxs`
- 免费，无需 API key；加 `mailto=` 进入 polite pool（10 req/s）
- 速率：100ms

---

## Orchestrator

```go
func Search(
    ctx context.Context,
    providers []Provider,
    q QueryParams,
    f SubjectFilter,
    log *log.Logger,
) (*model.MultiSourceResult, error)
```

**执行流程**：
1. 生成 `trace_id`（8位随机 hex），注入 `context`
2. goroutine per provider（并发 fan-out）
3. 各 provider 结果写入带 ProviderID 标签的 channel
4. 按主源顺序排列分组
5. 去重：key = DOI（优先）或 normalized title（小写去标点）
   - 按主源顺序遍历：主源条目入 seen；次源命中 seen → 丢弃
6. 返回 `MultiSourceResult`

**部分失败策略**：
- 若至少一个 provider 成功，返回部分结果（`MultiSourceResult`）+ `nil` error；各 provider 错误仅写入日志（`provider_error`）
- 若所有 provider 均返回 error，返回 `nil, errors.Join(err1, err2, ...)`，CLI 层打印错误并退出非零状态

**未知 `-s` 输入处理**：
- `subject.Lookup()` 对无法识别的 subject 返回 `ErrUnknownSubject`，CLI 在发起任何 HTTP 请求前打印错误和有效别名列表，退出非零状态：
  ```
  error: unknown subject "astrophysics"
  valid aliases: cs, physics, math, stat, q-fin, econ, q-bio, eess, sociology, education, philosophy, law, psychology, ...
  arXiv codes: cs.AI, cs.LG, cs.CL, cs.CV, hep-th, quant-ph, ...
  ```

---

## AI 日志可观测性（`internal/log/`）

- 输出到 **stderr**，JSON 格式，默认关闭，`--debug` 或 `ARXS_DEBUG=1` 开启
- 每条日志携带 `trace_id` 用于并发请求关联

**关键事件节点**：

```json
{"ts":"2026-03-25T10:00:00Z","trace_id":"a3f2c1b9","level":"info","step":"subject_lookup","subjects":["cs.AI","q-fin"],"providers":["arxiv","zenodo","openalex"],"elapsed_ms":0}
{"ts":"2026-03-25T10:00:00Z","trace_id":"a3f2c1b9","level":"info","step":"provider_start","provider":"arxiv","query":"ti:transformer AND cat:cs.AI","max":50}
{"ts":"2026-03-25T10:00:03Z","trace_id":"a3f2c1b9","level":"info","step":"provider_done","provider":"arxiv","count":45,"elapsed_ms":3120}
{"ts":"2026-03-25T10:00:04Z","trace_id":"a3f2c1b9","level":"error","step":"provider_error","provider":"openalex","error":"HTTP 429: rate limited","elapsed_ms":980}
{"ts":"2026-03-25T10:00:04Z","trace_id":"a3f2c1b9","level":"info","step":"dedup","before":110,"after":87,"removed":23}
{"ts":"2026-03-25T10:00:04Z","trace_id":"a3f2c1b9","level":"info","step":"done","total":87,"elapsed_ms":4200}
```

**完整事件列表**：`subject_lookup`、`provider_start`、`provider_done`、`provider_error`、`http_request`（debug）、`http_response`（debug）、`dedup`、`cache_hit`、`cache_miss`、`download_start`、`download_done`、`download_error`

---

## CLI 变更

### `-s` 改为 StringArray

```go
searchCmd.Flags().StringArrayVarP(&flagSubjects, "subject", "s", nil, "Subject (repeatable, OR): -s cs.AI -s q-fin")
```

**逗号兼容**：每个 StringArray 元素在 `subject.Lookup()` 前先按逗号拆分，支持混合写法：
- `-s cs.AI -s q-fin` → `["cs.AI", "q-fin"]`
- `-s cs.AI,cs.LG` → `["cs.AI", "cs.LG"]`
- `-s cs.AI,cs.LG -s q-fin` → `["cs.AI", "cs.LG", "q-fin"]`

### `--debug` 日志开关

`--debug` 注册为 `rootCmd` 的 persistent flag，全局生效：
```go
// cmd/root.go
var flagDebug bool
rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable structured JSON debug logging to stderr")
```

Logger 在 `cmd/root.go` 的 `PersistentPreRun` 中构建，存入 `context` 传递给 orchestrator：
```go
rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
    logger := log.New(flagDebug)
    cmd.SetContext(log.WithLogger(cmd.Context(), logger))
    return nil
}
```

### `download` 命令变更

**store 扩展**：新增 `WriteMultiSourceResult`/`ReadMultiSourceResult`（返回 `*model.MultiSourceResult`），旧 `WriteResults`/`ReadResults` 保留（arXiv-only 单源结果向后兼容）。`download` 命令优先尝试 `ReadMultiSourceResult`，失败后 fallback 到 `ReadResults` 并将 papers 包装为单组。

**全局序号**：`download 1 3 5` 中的序号按输出表格的全局 `#` 列映射（跨分组连续编号），与终端展示一致。

**非 arXiv PDF 下载**：`runDownload` 根据 `paper.Source` 分发到对应 Provider 的 `DownloadPDF`；若 `paper.PDFUrl` 为空（如 OpenAlex 仅有 landing page），打印警告并输出 `paper.SourceURL` 供用户手动访问：
```
warn: #74 openalex/W2741809807 has no direct PDF link
      visit: https://doi.org/10.1016/j.econ.2025.01.003
```

### 终端输出（分组）

```
Found 87 papers across 3 sources (after dedup), saved to arxiv-results.json

[arXiv — 45 papers]
 #   Published    Category   Cited   Title
 1   2025-03-01   cs.AI      1203    Attention Is All You Need Revisited
 ...

[Zenodo — 28 papers]
 #   Published    Source     Title
 46  2024-11-10   zenodo      Dataset: LLM Benchmark Suite
 ...

[OpenAlex — 14 papers]
 #   Published    Source     Cited   Title
 74  2025-01-20   openalex    312    Economic Impacts of Transformer Models
```

### 摘要文件末尾新增来源段

```
Title: Attention Is All You Need Revisited
Authors: A. Vaswani, N. Shazeer
ID: 2401.12345
URL: https://arxiv.org/abs/2401.12345

Abstract:
We propose...

---
Source: arXiv (https://arxiv.org/abs/2401.12345)
Retrieved via: arxs v2.0.0 | 2026-03-25
```

---

## 测试策略

**实现顺序**（严格按依赖）：
1. `model` 扩展 + `log` 包
2. `subject/registry`（纯逻辑，无 HTTP）
3. 逐个 Provider（调通 + 测试后再继续下一个）：arxiv → zenodo → socarxiv → edarxiv → openalex
4. `orchestrator`（依赖全部 Provider）
5. CLI 接入

**每个 Provider 测试覆盖**（httptest mock server）：
1. 正常响应 → 正确解析 Paper 字段（title/authors/abstract/source/source_url/pdf_url）
2. HTTP 4xx/5xx → 返回 error，不 panic
3. 空结果 → 返回空 slice，不 error
4. 畸形 JSON/XML → 返回 parse error
5. **超时/Context 取消** → mock server hang，`context.WithTimeout(100ms)`，验证返回 `context.DeadlineExceeded`，无 goroutine 泄漏

**orchestrator 测试**：
- mock providers，验证 dedup（相同 DOI/title 保留主源）
- 验证分组顺序（主源在前）
- 验证并发安全（-race flag）

---

## 速率限制汇总

| 源 | 间隔 | 依据 |
|----|------|------|
| arXiv | 3s | 现有不变 |
| Zenodo | 1s | 官方 60 req/min |
| SocArXiv/EdArXiv | 1s | OSF 无明确限制，保守 |
| OpenAlex | 100ms | polite pool 10 req/s |

---

## 合规声明

- arXiv：遵守现有 API Terms of Use，3s 限速不变
- Zenodo：使用官方 REST API，遵守速率限制
- OSF：使用官方公开 API
- OpenAlex：完全开源，CC0 许可，使用 polite pool

---

## 实现范围外（不在本次）

- SSRN / PhilArchive 支持（无公开 API）
- bioRxiv / medRxiv（生物医学方向扩展）
- OpenReview（会议评审系统）
- `arxs open`、`arxs bib` 等扩展命令
