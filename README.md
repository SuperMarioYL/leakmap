[English](./README.en.md) · [Website](https://leakmap.lei6393.com) · [GitHub](https://github.com/SuperMarioYL/leakmap)

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/hero-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/hero-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/hero-dark.svg">
  <img src="./assets/presentation/hero-light.svg" width="960" alt="Hero diagram">
</picture>

# LeakMap

**发现跨工作树出现的密钥值。**

LeakMap 在 Git 工作树中索引符合规则的密钥文件，在受监控文件出现其他工作树的值时输出事件。

## 为什么需要它

并行工作树共享主机文件系统。内容从源到目标的匹配，可以在资料意外出现在另一工作区时提供明确的文件对供调查。

- **保留文件来源** — 事件包含源与目标路径。
- **本地检查** — JSONL 可生成终端、HTML 与 Markdown 视图。
- **事件省略原值** — 序列化事件记录字段与匹配类别。

## 架构

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/architecture-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/architecture-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/architecture-dark.svg">
  <img src="./assets/presentation/architecture-light.svg" width="960" alt="Architecture diagram">
</picture>

worktree discovery 枚举根目录，secret 按文件规则扫描并建立内存原值索引；fsnotify 事件将文件内容交给精确/模糊匹配器。事件保留源与目标路径，但不序列化原值；渲染器从 JSONL 生成本地报告。

| 组件 | 职责 |
| --- | --- |
| `Worktree discovery` | internal/worktree |
| `Secret index` | internal/secret |
| `Write matcher` | internal/leak/detect.go |
| `JSONL events` | internal/leak/event.go |
| `Local reports` | internal/render |

## 安装与快速上手

使用仓库清单声明的运行时版本。以下源码安装步骤可复现随仓示例。

```bash
git clone https://github.com/SuperMarioYL/leakmap.git
cd leakmap
go build ./cmd/leakmap
```

Go 示例对一个完整虚构 .env 值运行，并比较跨工作树与同工作树匹配。

```bash
go run ./examples/presentation
```

## 实际运行示例

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/process-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/process-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/process-dark.svg">
  <img src="./assets/presentation/process-light.svg" width="960" alt="Process diagram">
</picture>

One DB_TOKEN exact match crosses worktree-a to worktree-b; the same-worktree check yields zero events.

```text
fingerprints: 1
worktree-a -> worktree-b: field=DB_TOKEN match=exact severity=secret
same-worktree matches: 0
```

完整命令与输出保存在 [docs/demo-results.json](./docs/demo-results.json). 输入和复现代码均随仓提供。

![已有终端录制](./assets/demo.gif)

保留已有录制供参考；上方文字示例给出当前可复现的操作。

## 用法

安装后在仓库根目录运行以下命令；处理自己的数据时替换相应路径。

```bash
go run ./cmd/leakmap scan --repo . --json
go run ./cmd/leakmap watch --repo . --jsonl leakmap.jsonl
go run ./cmd/leakmap map --repo . --html leakmap.html
go run ./cmd/leakmap report --repo . -m REPORT.md
```

## 配置

--repo 指定 Git 仓库，--jsonl 指定事件文件，--verbose 开启诊断，watch 的 --tui 在会话结束（Ctrl-C）后直接渲染本次累积的 leak-map TUI。扫描覆盖 .env 变体、key/PEM 与 credential 文件，跳过构建和依赖目录。少于八字节的值不做跨树匹配；人类可读扫描输出会掩码原值，事件报告仍包含可能敏感的路径。

## 集成与职责分工

<picture>
  <source media="(max-width: 600px) and (prefers-color-scheme: dark)" srcset="./assets/presentation/integrations-mobile-dark.svg">
  <source media="(max-width: 600px)" srcset="./assets/presentation/integrations-mobile-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./assets/presentation/integrations-dark.svg">
  <img src="./assets/presentation/integrations-light.svg" width="960" alt="Integrations diagram">
</picture>

根据工作流选择输入与输出路径。本文本地示例验证其中明确说明的子流程。

| 路径 | 已实现职责 |
| --- | --- |
| Git worktrees | Source and target roots |
| .env / key files | Pattern-based fingerprinting |
| fsnotify | Watched file writes |
| JSONL | Path and match evidence |
| TUI / HTML / Markdown | Local inspection exports |

## 限制与后续方向

- 内容匹配不能证明哪个进程复制、哪个 Agent 导致或是否已提交。PID 归因只是尽力识别，可能未知。
- 索引与递归监听在启动时建立；新建子目录与后续密钥变化可能不在覆盖范围内。这是检测工具，不是隔离或阻断边界。
- 示例只验证扫描与匹配，使用假 token；未读取用户密钥，也未验证实时监听、网络出口或环境读取截获。

网络/环境截获、模型摘要、更完整文件系统覆盖和托管保留是后续方向；本仓库未交付团队服务。

## 许可与贡献

许可见 [LICENSE](./LICENSE). 反馈问题时请提供最小输入、执行命令和实际输出。
