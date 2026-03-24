# arxs — arXiv 论文搜索 CLI 工具设计文档

## 概述

`arxs` 是一个 Go 语言编写的命令行工具，通过 arXiv 官方 API（`export.arxiv.org`）快速搜索学术论文，支持按标题、摘要、作者、分类和日期筛选，并可下载 PDF 或摘要到本地。

## 技术选型

- **语言**：Go（编译为单一二进制，无运行时依赖）
- **CLI 框架**：Cobra（子命令、帮助文档、Shell 补全自动生成）
- **HTTP / XML**：Go 标准库 `net/http` + `encoding/xml`
- **API**：arXiv Atom API（`http://export.arxiv.org/api/query`）

## 合规与礼仪

### 必须遵守的规则

1. **User-Agent 标识**：每个 HTTP 请求附带 `arxs/<version> (https://github.com/xxx/arxs)` 格式的 User-Agent，让 arXiv 知道访问来源。
2. **限速**：请求间隔 ≥ 3 秒，硬编码在 `ratelimit.go` 中，不可通过参数绕过。
3. **缓存**：arXiv 数据每日 UTC 午夜更新一次。同一查询同一天内命中缓存则不重复请求。缓存存储在当前目录 `.arxs-cache/`，支持 `--no-cache` 跳过。
4. **致谢声明**：arXiv API Terms of Use 要求在产品中包含致谢。工具在 `arxs about` 和 `--version` 输出中显示：
   > Thank you to arXiv for use of its open access interoperability.
5. **请求量控制**：单次查询默认最多 50 条结果（`--max` 可调，上限 2000），避免对 API 造成不必要压力。

## 项目结构

```
arxs/
├── main.go                  # 入口
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go              # cobra root 命令，全局 flag（--version）
│   ├── search.go            # search 子命令
│   ├── list.go              # list 子命令
│   ├── download.go          # download 子命令
│   └── about.go             # about 子命令：协议声明、致谢
├── internal/
│   ├── api/
│   │   ├── client.go        # HTTP 客户端（UA 标识 + 限速 + 缓存检查）
│   │   ├── query.go         # 构建 arXiv API 查询 URL
│   │   └── ratelimit.go     # 限速器（3s 间隔，不可配置跳过）
│   ├── model/
│   │   ├── paper.go         # Paper 结构体
│   │   └── atom.go          # Atom XML 反序列化结构
│   ├── store/
│   │   └── store.go         # JSON 文件读写
│   ├── parser/
│   │   └── expr.go          # 解析 "A or B and C" 搜索表达式
│   └── cache/
│       └── cache.go         # 按查询+日期的本地缓存
└── testdata/                # 测试用 XML fixtures
```

### 职责划分

- `cmd/`：只做参数解析和调用，不含业务逻辑。
- `internal/api/`：HTTP 客户端、查询构建、限速。
- `internal/model/`：数据结构定义。
- `internal/store/`：JSON 文件的读写。
- `internal/parser/`：搜索表达式解析（将用户输入转换为 arXiv API 查询语法）。
- `internal/cache/`：本地缓存管理。

## 数据模型

### Paper 结构体

```go
type Paper struct {
    ID         string   `json:"id"`          // arXiv ID, e.g. "2401.12345"
    Title      string   `json:"title"`
    Authors    []string `json:"authors"`
    Abstract   string   `json:"abstract"`
    Categories []string `json:"categories"`  // e.g. ["cs.AI", "cs.CL"]
    Published  string   `json:"published"`   // RFC3339 UTC, e.g. "2024-01-15T00:00:00Z"
    Updated    string   `json:"updated"`     // RFC3339 UTC
    PDFUrl     string   `json:"pdf_url"`     // https://arxiv.org/pdf/2401.12345
    HTMLUrl    string   `json:"html_url"`    // https://arxiv.org/html/2401.12345
    AbsUrl     string   `json:"abs_url"`     // https://arxiv.org/abs/2401.12345
}
```

> 时间戳使用 RFC3339 UTC 字符串存储。RFC3339 UTC 字符串的字典序与时间顺序一致，可直接用于排序比较，无需解析为 `time.Time`。

### SearchResult 结构体

```go
type SearchResult struct {
    Query        QueryMeta `json:"query"`
    TotalResults int       `json:"total_results"` // API 返回的匹配总数
    ReturnCount  int       `json:"return_count"`  // 本次实际返回的数量
    Papers       []Paper   `json:"papers"`
}

type QueryMeta struct {
    Terms      map[string]string `json:"terms"`       // {"title": "transformer", "author": "vaswani"}
    Subjects   []string          `json:"subjects"`
    Op         string            `json:"op"`          // "and" or "or"
    From       string            `json:"from"`
    To         string            `json:"to"`
    Max        int               `json:"max"`
    SearchedAt string            `json:"searched_at"` // RFC3339 UTC
}
```

### JSON 输出格式

```json
{
  "query": {
    "terms": { "title": "transformer or attention", "author": "vaswani" },
    "subjects": ["cs"],
    "op": "and",
    "from": "2024-01",
    "to": "",
    "max": 50,
    "searched_at": "2025-03-24T10:30:00Z"
  },
  "total_results": 42,
  "papers": [
    {
      "id": "2401.12345",
      "title": "Attention Is All You Need Revisited",
      "authors": ["A. Vaswani", "N. Shazeer"],
      "abstract": "We propose...",
      "categories": ["cs.AI", "cs.CL"],
      "published": "2024-01-15T00:00:00Z",
      "updated": "2024-01-20T00:00:00Z",
      "pdf_url": "https://arxiv.org/pdf/2401.12345",
      "html_url": "https://arxiv.org/html/2401.12345",
      "abs_url": "https://arxiv.org/abs/2401.12345"
    }
  ]
}
```

## 搜索流程

### 表达式解析

用户输入通过 `parser/expr.go` 转换为 arXiv API 查询语法：

| 用户输入 | 转换结果 |
|---------|---------|
| `-k-title "transformer or attention"` | `ti:transformer OR ti:attention` |
| `-k-author "vaswani and hinton"` | `au:vaswani AND au:hinton` |
| `-k-abs "reinforcement learning"` | `abs:reinforcement AND abs:learning`（无运算符默认 AND） |
| `-k "transformer"` | `(ti:transformer OR abs:transformer OR au:transformer)` |
| 多个 `-k-*` + `--op and`（默认） | 各部分用 AND 连接 |
| 多个 `-k-*` + `--op or` | 各部分用 OR 连接 |

### 运算符优先级

在单个 `-k-*` 值内，`and` 优先级高于 `or`：
- `"A or B and C"` → `field:A OR (field:B AND field:C)`

### 分类映射

arXiv API 的 `cat:` 前缀支持顶级分类匹配（如 `cat:cs` 匹配 `cs.*` 下所有子分类）。无需枚举所有子分类。

| 参数值 | API 查询 | 说明 |
|-------|---------|------|
| `cs` | `cat:cs` | 计算机科学所有子分类 |
| `math` | `cat:math` | 数学所有子分类 |
| `physics` | `cat:physics OR cat:astro-ph OR cat:cond-mat OR cat:gr-qc OR cat:hep-ex OR cat:hep-lat OR cat:hep-ph OR cat:hep-th OR cat:math-ph OR cat:nlin OR cat:nucl-ex OR cat:nucl-th OR cat:quant-ph` | 物理学需要枚举，因为它跨多个顶级分类 |
| `q-bio` | `cat:q-bio` | 定量生物学 |
| `q-fin` | `cat:q-fin` | 定量金融 |
| `stat` | `cat:stat` | 统计学 |
| `eess` | `cat:eess` | 电气工程与系统科学 |
| `econ` | `cat:econ` | 经济学 |

多个分类之间用 OR 连接：`-s cs,math` → `(cat:cs OR cat:math)`。

### 日期筛选

- `--from 2024-01` + `--to 2025-03` → `submittedDate:[202401010000+TO+202503312359]`
- `--recent 12m` → 自动计算为最近 12 个月的 from/to 范围
- 不传日期参数 → 不加日期过滤
- `--recent` 与 `--from`/`--to` 互斥，同时传入报错：`error: --recent cannot be used with --from/--to`

### API 调用流程

```
1. 解析 CLI 参数
2. 检查缓存（查询 + 日期匹配 → 返回缓存结果）
3. 构建查询 URL
4. 发送 HTTP GET（带 UA + 限速）
5. 解析 Atom XML → []Paper
6. 写入 JSON 文件
7. 终端输出结果摘要表格
```

## 子命令设计

### `arxs search`

**参数：**

| Flag | 短写 | 说明 | 默认值 |
|------|------|------|--------|
| `--key` | `-k` | 全字段搜索（title OR abstract OR author） | — |
| `--key-title` | `-t` | 标题搜索 | — |
| `--key-abs` | `-b` | 摘要搜索（a**b**stract） | — |
| `--key-author` | `-a` | 作者搜索 | — |
| `--subject` | `-s` | 分类，逗号分隔 | 全部 |
| `--op` | — | 多个 -k-* 之间的关系 | `and` |
| `--from` | — | 起始日期 YYYY[-MM[-DD]] | 不限 |
| `--to` | — | 截止日期 YYYY[-MM[-DD]] | 不限 |
| `--recent` | — | 快捷日期，如 `12m`、`6m`、`1y` | — |
| `--max` | `-m` | 最大结果数 | 50 |
| `--output` | `-o` | 输出 JSON 路径 | `./arxiv-results.json` |
| `--no-cache` | — | 跳过缓存 | false |
| `--sort` | — | 排序方式：`relevance`、`submitted`、`updated` | `submitted` |
| `--order` | — | 排序方向：`asc`、`desc` | `desc` |

**终端输出：**

```
Found 42 papers (5320 total matches), saved to ./arxiv-results.json

 #  Published   Category  Title
 1  2025-03-01  cs.AI     Attention Is All You Need Revisited
 2  2025-02-15  cs.CL     Sparse Attention Mechanisms for ...
 3  2025-01-20  cs.LG     On the Efficiency of Attention ...
...
```

> 当总匹配数 > 返回数时显示 `(N total matches)`，让用户知道可以用 `--max` 获取更多。

### `arxs list`

**参数：**

| Flag | 短写 | 说明 | 默认值 |
|------|------|------|--------|
| `--file` | `-f` | 指定 JSON 文件 | `./arxiv-results.json` |
| `--verbose` | `-v` | 显示摘要 | false |
| `--limit` | `-n` | 显示前 N 条 | 全部 |

### `arxs download`

**位置参数**：论文序号（从 `list` 输出中的 `#` 列），如 `1 3 5`。

**参数：**

| Flag | 短写 | 说明 | 默认值 |
|------|------|------|--------|
| `--abs-only` | — | 只下载摘要（保存为 .txt） | false |
| `--all` | — | 下载全部结果 | false |
| `--dir` | `-d` | 保存目录 | `.`（当前目录） |
| `--file` | `-f` | 指定 JSON 文件 | `./arxiv-results.json` |
| `--overwrite` | — | 覆盖已存在的文件 | false |

**文件命名**：`{arXiv_ID}.pdf` 或 `{arXiv_ID}.txt`。用 arXiv ID 作为唯一文件名，简单且无冲突。

**下载限速**：PDF 下载同样遵守 3 秒间隔。

**大批量下载确认**：`--all` 且结果超过 10 篇时，提示确认 `About to download 42 PDFs (~2 min at 3s interval). Continue? [y/N]`。

**文件已存在**：同名文件已存在时跳过，提示 `skip: 2401.12345.pdf (already exists)`。用 `--overwrite` 强制覆盖。

### `arxs about`

输出工具信息、致谢声明、API 链接和使用协议。

```
arxs - arXiv paper search CLI tool v1.0

Thank you to arXiv for use of its open access interoperability.

API:          https://export.arxiv.org/api/query
Terms of Use: https://arxiv.org/help/api/tou
Rate limit:   3s between requests (enforced)
Source:       https://github.com/xxx/arxs
```

## 使用示例

### 场景 1：快速搜索一个关键词

```bash
arxs search -k "transformer"
```

在标题、摘要、作者中搜索 "transformer"，返回最近 50 篇论文。

### 场景 2：按标题搜索多个关键词（OR）

```bash
arxs search -k-title "transformer or attention or self-attention"
```

标题中含 transformer、attention 或 self-attention 的论文。

### 场景 3：精确搜索特定作者的特定主题

```bash
arxs search -k-title "diffusion model" -k-author "ho and song"
```

标题含 "diffusion" AND "model"，且作者同时含 "ho" AND "song"。两个 `-k-*` 之间默认 AND。

### 场景 4：跨字段 OR 搜索

```bash
arxs search -k-title "RLHF" -k-abs "reward model" --op or
```

标题含 "RLHF" **或者** 摘要含 "reward" AND "model"。

### 场景 5：限定分类和日期

```bash
arxs search -k "large language model" -s cs,stat --from 2024-01 --to 2025-03
```

在计算机科学和统计学分类中，搜索 2024 年 1 月到 2025 年 3 月的论文。

### 场景 6：最近一年的论文

```bash
arxs search -k-abs "quantum computing" -s physics --recent 12m
```

物理学分类中最近 12 个月关于量子计算的论文。

### 场景 7：查看搜索结果

```bash
arxs list
arxs list --verbose    # 带摘要
```

### 场景 8：下载 PDF

```bash
arxs download 1 3 5              # 下载第 1、3、5 篇
arxs download --all               # 下载全部
arxs download 1 --dir ./papers    # 指定目录
```

### 场景 9：只保存摘要

```bash
arxs download --abs-only 2 4
```

将第 2、4 篇的摘要保存为 `.txt` 文件。

### 场景 10：使用不同的结果文件

```bash
arxs search -k "GAN" -o ./gan-papers.json
arxs list -f ./gan-papers.json
arxs download 1 2 3 -f ./gan-papers.json
```

## 错误处理

| 场景 | 行为 |
|------|------|
| 网络不可达 | 报错并提示检查网络 |
| API 返回错误 | arXiv API 错误以 `<entry>` 形式返回，`<id>` 为 `http://arxiv.org/api/errors#...`。检测到此 id 前缀时解析 `<summary>` 作为错误信息展示给用户 |
| JSON 文件不存在 | 提示先运行 `arxs search` |
| 序号超出范围 | 提示有效范围 |
| PDF 下载失败 | 报错但继续下载其他文件，最终汇总失败列表 |
| 无搜索结果 | 提示无结果并建议调整搜索条件 |
| `--recent` 与 `--from`/`--to` 同时使用 | 报错提示互斥 |

## 未来可扩展方向（不在本次实现范围）

- `arxs open 1`：用浏览器打开论文页面
- `arxs bib 1 3`：导出 BibTeX 引用
- `arxs watch -k "xxx"`：定期监控新论文并通知
