# arxs

arXiv 论文搜索命令行工具。通过 arXiv 官方 API 快速搜索、浏览和下载学术论文。

> Thank you to arXiv for use of its open access interoperability.

[English](README.md)

## 安装

```bash
go install github.com/joejiang/arxs@latest
```

或从源码编译：

```bash
git clone https://github.com/host452b/arxs.git
cd arxs
go build -o arxs .
```

## 快速开始

```bash
# 搜索关键词
arxs search -k "transformer"

# 查看结果
arxs list

# 下载论文
arxs download 1 3 5
```

## 使用指南

### search — 搜索论文

#### 基本搜索

```bash
# 在标题、摘要、作者中搜索 "transformer"
arxs search -k "transformer"

# 仅搜索标题
arxs search -t "transformer"

# 仅搜索摘要
arxs search -b "reinforcement learning"

# 仅搜索作者
arxs search -a "vaswani"
```

#### 使用 or / and 组合搜索词

在同一个字段内，可以用 `or` 和 `and` 组合多个关键词（类似 pytest `-k` 的用法）：

```bash
# 标题含 "transformer" 或 "attention"
arxs search -t "transformer or attention"

# 作者同时含 "vaswani" 和 "hinton"
arxs search -a "vaswani and hinton"

# 标题含 "transformer" 或 "attention" 或 "self-attention"
arxs search -t "transformer or attention or self-attention"
```

**运算符优先级**：`and` 优先级高于 `or`：
- `"A or B and C"` 等价于 `"A or (B and C)"`

**隐式 AND**：多个词之间没有运算符时默认为 AND：
- `"reinforcement learning"` 等价于 `"reinforcement and learning"`

#### 组合多个字段

多个 `-k-*` 参数之间默认用 AND 连接：

```bash
# 标题含 "diffusion model" 且 作者含 "ho" 和 "song"
arxs search -t "diffusion model" -a "ho and song"
```

用 `--op or` 切换为 OR：

```bash
# 标题含 "RLHF" 或者 摘要含 "reward model"
arxs search -t "RLHF" -b "reward model" --op or
```

#### 按学科分类筛选

支持的分类：`cs`（计算机科学）、`math`（数学）、`physics`（物理学）、`q-bio`（定量生物学）、`q-fin`（定量金融）、`stat`（统计学）、`eess`（电气工程与系统科学）、`econ`（经济学）

```bash
# 只搜索计算机科学
arxs search -k "LLM" -s cs

# 搜索计算机科学和统计学
arxs search -k "machine learning" -s cs,stat
```

#### 按日期筛选

```bash
# 指定日期范围
arxs search -k "LLM" --from 2024-01 --to 2025-03

# 最近 12 个月
arxs search -k "LLM" --recent 12m

# 最近 1 年
arxs search -k "LLM" --recent 1y
```

> `--recent` 和 `--from`/`--to` 不能同时使用。

#### 其他选项

```bash
# 最多返回 100 条结果（默认 50，上限 2000）
arxs search -k "GAN" --max 100

# 按相关性排序（默认按提交日期降序）
arxs search -k "GAN" --sort relevance

# 指定输出文件
arxs search -k "GAN" -o ./gan-papers.json

# 跳过缓存（同一天同一查询默认使用缓存）
arxs search -k "GAN" --no-cache
```

### list — 查看结果

```bash
arxs list                       # 查看搜索结果列表
arxs list --verbose             # 显示详细信息（含摘要）
arxs list -n 10                 # 只显示前 10 条
arxs list -f ./gan-papers.json  # 查看指定文件
```

### download — 下载论文

```bash
arxs download 1 3 5            # 下载第 1、3、5 篇的 PDF
arxs download --all             # 下载全部 PDF
arxs download --abs-only 2 4    # 只保存摘要为 .txt 文件
arxs download 1 --dir ./papers  # 指定保存目录
arxs download 1 -f ./gan.json   # 从指定结果文件下载
arxs download 1 --overwrite     # 覆盖已存在的文件
```

文件命名格式：`{arXiv_ID}.pdf` 或 `{arXiv_ID}.txt`

### about — 工具信息

```bash
arxs about
```

## 参数速查

### search

| 参数 | 短写 | 说明 | 默认值 |
|------|------|------|--------|
| `--key` | `-k` | 全字段搜索 | — |
| `--key-title` | `-t` | 标题搜索 | — |
| `--key-abs` | `-b` | 摘要搜索 | — |
| `--key-author` | `-a` | 作者搜索 | — |
| `--subject` | `-s` | 学科分类 | 全部 |
| `--op` | — | 字段间关系 | `and` |
| `--from` | — | 起始日期 | 不限 |
| `--to` | — | 截止日期 | 不限 |
| `--recent` | — | 最近时间段 | — |
| `--max` | `-m` | 最大结果数 | 50 |
| `--output` | `-o` | 输出文件 | `arxiv-results.json` |
| `--sort` | — | 排序方式 | `submitted` |
| `--order` | — | 排序方向 | `desc` |
| `--no-cache` | — | 跳过缓存 | false |

### list

| 参数 | 短写 | 说明 | 默认值 |
|------|------|------|--------|
| `--file` | `-f` | 结果文件 | `arxiv-results.json` |
| `--verbose` | `-v` | 显示摘要 | false |
| `--limit` | `-n` | 显示前 N 条 | 全部 |

### download

| 参数 | 短写 | 说明 | 默认值 |
|------|------|------|--------|
| `--file` | `-f` | 结果文件 | `arxiv-results.json` |
| `--dir` | `-d` | 保存目录 | `.` |
| `--abs-only` | — | 只下载摘要 | false |
| `--all` | — | 下载全部 | false |
| `--overwrite` | — | 覆盖已有文件 | false |

## 合规说明

- 使用 arXiv 官方 API（`export.arxiv.org`），不爬取网页
- 请求间隔 ≥ 3 秒，硬编码不可绕过
- 同一查询同一天内使用缓存，减少不必要的请求
- 所有请求携带 User-Agent 标识
- 遵守 [arXiv API Terms of Use](https://arxiv.org/help/api/tou)

## 使用规则与文明公约

### 你必须遵守的

1. **尊重 arXiv 服务**：arXiv 是由学术社区和志愿者维护的非营利开放获取平台。它的存在依赖于所有人的善意使用。
2. **不要滥用 API**：工具已内置 3 秒限速，请不要尝试绕过或修改此限制。大量无意义的请求会影响其他用户。
3. **不要批量下载整个分类**：只下载你真正需要阅读的论文。arXiv 不是你的本地硬盘备份。
4. **尊重作者权益**：下载的论文仅供个人学术研究使用。不得将下载内容用于未经授权的商业用途或二次分发。
5. **遵守 arXiv 使用条款**：详见 [arXiv API Terms of Use](https://arxiv.org/help/api/tou)。

### 如果你基于此工具二次开发

1. **保留致谢声明**：arXiv API 使用条款要求在产品中包含致谢（工具已内置）。
2. **保持限速逻辑**：不要移除或降低 3 秒请求间隔。
3. **标识你的 User-Agent**：修改 `client.go` 中的 UA 字符串为你自己的项目名和联系方式。
4. **不要伪装身份**：不要伪造 User-Agent 或试图绕过 arXiv 的访问控制。

### 互联网文明

> 我们使用开放的学术资源，也应当成为开放学术社区的良好公民。

- arXiv 每年服务数百万研究者，运营成本依赖 [捐赠和会员机构支持](https://arxiv.org/about/donate)
- 如果 arxs 对你的研究有帮助，请考虑向 arXiv 捐赠
- 引用论文时请使用正确的引用格式，尊重原作者的劳动成果
- 发现论文中的问题？通过正规渠道反馈，而不是公开攻击

## License

MIT
