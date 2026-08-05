<div align="right">
  <a href="./README.en.md">English</a> | <b>简体中文</b>
</div>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/hero-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assets/hero-light.svg">
  <img src="assets/hero-light.svg" alt="LeakMap — cross-worktree leak provenance" width="880">
</picture>

<p align="center"><sub>让跑在并行 git worktree 里的多个 coding agent 泄漏无所遁形——实时归因谁把谁的 <code>.env</code> 带进了谁的 commit。</sub></p>

<p align="center">
  <b>并行 worktree 里 agent 的密钥/文件渗漏，第一次有了实时归因的 leak-map。</b>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/SuperMarioYL/leakmap?color=blue" alt="license"></a>
  <a href="https://github.com/SuperMarioYL/leakmap/releases"><img src="https://img.shields.io/github/v/release/SuperMarioYL/leakmap?label=release" alt="release"></a>
  <a href="https://github.com/SuperMarioYL/leakmap/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/SuperMarioYL/leakmap/ci.yml?label=ci" alt="ci"></a>
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white" alt="go">
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux-6366f1" alt="platform">
</p>

---

## <img src="https://api.iconify.design/tabler:topology-star-3.svg?color=%230071E3&width=24" width="24" align="top"> 架构

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="assets/atlas-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="assets/atlas-light.svg">
  <img src="assets/atlas-light.svg" alt="LeakMap architecture" width="880">
</picture>

单二进制、单进程、内存态。LeakMap 发现仓库的全部 git worktree，指纹每个 worktree 的密钥面（`.env`、`*.key`、credentials），对所有 worktree 根目录开 fsnotify 写监听；当 worktree B 的写入里出现来自 worktree A 的指纹值，立即发出一条带归因的 **LeakEvent**——「Agent A 的 `.env` 中 `DB_TOKEN` 出现在 Agent B 对 worktree B 的提交里」——写入 `leakmap.jsonl` 并渲染为 leak-map（TUI / 本地 HTML / Markdown）。

## <img src="https://api.iconify.design/tabler:bulb.svg?color=%230071E3&width=24" width="24" align="top"> 为什么需要它

git worktree 是 git 的构造，不是隔离边界：并行跑在多个 worktree 里的 coding agent 共享同一台机的文件系统、环境变量、网络出口与构建缓存。Agent A 读到的密钥、Agent B 写的文件、Agent C 用过的 token，都可能静默渗进另一个 worktree 的运行——而你看不到任何审计痕迹。

[fletch.sh 的那篇《Git worktrees are not an isolation boundary for coding agents》](https://fletch.sh/blog/git-worktrees-vs-clones-for-ai-agents/) 把这个结构性问题讲透了：今天 Claude Code / Cursor / Codex（以及国内的 Trae、通义灵码）都鼓励在 N 个 worktree 里并行跑 agent 会话，但跨 worktree 的密钥渗漏既不可见、也无归因。LeakMap 把 secret-scanning 从「静态扫文件」升级成「实时跨 worktree 追溯归因」——第一次让你能回答「Agent A 的 `.env` 是不是渗进了 Agent D 的 commit」。

<details>
<summary>与 gitleaks / trufflehog 的区别</summary>

| 维度 | gitleaks / trufflehog | LeakMap |
|---|---|---|
| 作用域 | 单仓库静态扫文件找密钥 | 跨 worktree 实时归因渗漏 |
| 时机 | 扫描快照 | 写入即匹配（fsnotify） |
| 归因 | 无 | source → target worktree + agent PID |
| 输出 | 密钥清单 | leak-map（节点 + 渗漏边）+ JSONL 审计流 |

gitleaks 扫「文件里有没有密钥」；LeakMap 回答「谁把谁的密钥带进了谁的 commit」——同一套 secret-scan 原语，换了跨 agent 边界的作用域。
</details>

## <img src="https://api.iconify.design/tabler:rocket.svg?color=%230071E3&width=24" width="24" align="top"> 安装与快速上手

```bash
go install github.com/SuperMarioYL/leakmap@latest      # 单二进制，<30s
cd repo && git worktree add ../wt-b branch-b           # 你已有 2+ worktree 跑 agent
leakmap watch                                          # 自动发现 worktree、指纹、开监听
```

首条 leak event 在 Agent B 把 Agent A 的 `.env` 值写进 wt-b 时起爆，落到 `leakmap.jsonl`。从冷启动到首枚归因 <2 分钟。

<details>
<summary>示例输出（leakmap scan）</summary>

```
== ./lm
   agent pid: 60283
   2 secret surface entr(y|ies)
   DB_TOKEN               secret     31941df1f1b913cb  super-…7890
   API_KEY                secret     ef3531b166010dcf  ak_liv…stuv

== ../lm-wt-b
   0 secret surface entr(y|ies)
   (none)

2 worktree(s), 2 fingerprint(s)
```
</details>

## <img src="https://api.iconify.design/tabler:terminal-2.svg?color=%230071E3&width=24" width="24" align="top"> 用法

```bash
# 1) 指纹每个 worktree 的密钥面，打印 per-worktree 秘密清单（m1）
leakmap scan --repo .

# 2) 监听跨 worktree 写入，匹配其它 worktree 的指纹 → LeakEvent JSONL（m2）
leakmap watch --repo .

# 3) 把累积的 leak-map 渲染成终端 TUI（节点 + 渗漏边）
leakmap map --repo .

# 4) 渲染成自包含的本地 HTML 页
leakmap map --repo . --html leakmap.html

# 5) 导出 Markdown 泄漏摘要（按 secret 严重度排序）
leakmap report --repo . -m REPORT.md
```

常用子命令与标志：

| 命令 | 作用 |
|---|---|
| `scan [--json]` | 发现 worktree、指纹密钥文件、分类（正则优先，国产模型分类模糊值为可选 seam） |
| `watch [--jsonl PATH]` | 开 fsnotify 跨 worktree 监听，匹配即发 LeakEvent |
| `map [--html PATH]` | 读 `leakmap.jsonl` 渲染 leak-map（TUI 或 HTML） |
| `report [-m PATH] [--html PATH]` | 导出 Markdown（可选 HTML）摘要 |

全局标志：`--repo`（仓库根，默认 `.`）、`--jsonl`（审计流路径，默认 `leakmap.jsonl`）、`-v` 诊断输出。完整示例见 [`examples/quickstart.sh`](./examples/quickstart.sh)。

## <img src="https://api.iconify.design/tabler:photo.svg?color=%230071E3&width=24" width="24" align="top"> Demo

![demo](assets/demo.gif)

> 录制脚本见 [`docs/demo.tape`](./docs/demo.tape)（[vhs](https://github.com/charmbracelet/vhs) 渲染）。CI 在 `.github/workflows/demo.yml` 里手动重渲染 gif。

## <img src="https://api.iconify.design/tabler:building.svg?color=%230071E3&width=24" width="24" align="top"> 商业版 / 定价

v0.1 是免费的 OSS CLI——归因 detector 永久免费、本地运行。当 DevOps / compliance 把「agent 是否跨 worktree 泄露 secret」写进 checklist，付费意愿出现在其上的留存与报告层：

| 层级 | 内容 | 定价 |
|---|---|---|
| Free（v0.1） | 本地 leak-map CLI + JSONL 审计流 | 免费 |
| Team（中期） | 加密 `leakmap.jsonl` 留存 N 天 + 团队合规报告导出（SOC2 风格，复用 GLM-4）+ SSO | ¥99–299 / 席位 / 月 |
| Enterprise | 多团队 dashboard、审计留存策略、SOC2 报告 | 按需报价 |

对标 GitGuardian internal monitoring ~$25/席位；LeakMap 的 detector 永久免费，付费层只覆盖留存与合规报告。不提供 hosted SaaS 的检测器——inner circle 要本地/自托管。需求闸门：在 LOC > 2k 前完成 5 位并行-agent dev-team lead 访谈，若不足 2 位确认愿为留存/报告付费，则推迟商业化。

## <img src="https://api.iconify.design/tabler:route.svg?color=%230071E3&width=24" width="24" align="top"> 路线图

- [x] **m1** per-worktree 秘密指纹 + 分类（`leakmap scan`）
- [x] **m2** fsnotify 跨 worktree 写入匹配 → LeakEvent JSONL（`leakmap watch`）
- [ ] **m3** leak-map TUI + 本地 HTML + GLM-4 摘要散文（基础 TUI/HTML/Markdown 已随发，模型散文待补）
- [ ] **v0.2** 网络出口渗漏检测（eBPF）+ 实时 env-var 读拦截（ptrace/eBPF）
- [ ] **中期** 审计日志留存 + 团队合规报告 + SSO（enterprise tier）
- [ ] **未来** IDE 插件 / 编辑器集成、Windows 支持

## <img src="https://api.iconify.design/tabler:share.svg?color=%230071E3&width=24" width="24" align="top"> 分享

```
LeakMap — 让并行 worktree 里的 coding agent 泄漏无所遁形。实时归因谁把谁的 .env 带进了谁的 commit。github.com/SuperMarioYL/leakmap
```

> 推送后建议设置仓库 topics：`gh repo edit --add-topic security --add-topic secret-scanning --add-topic coding-agent --add-topic worktree`

## <img src="https://api.iconify.design/tabler:scale.svg?color=%230071E3&width=24" width="24" align="top"> License & 贡献

[MIT](./LICENSE) © 2026 SuperMarioYL。欢迎在 [Issues](https://github.com/SuperMarioYL/leakmap/issues) 报 bug 或提 PR；设计 partner 请直接开 issue 标注 `design-partner`。

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
