# Antigravity Cloak（CLIProxyAPI 插件）

自 2026 年 11 月起，Google Antigravity 会按系统提示词里的固定句子识别第三方客户端，直接回假的 `429 RESOURCE_EXHAUSTED`（额度其实没用）。这个插件在 CPA 把请求发往 Antigravity 之前，改写系统提示词里的这些特征。

- 只处理发往 Antigravity 的请求，其它渠道原样放行。
- 只改 `systemInstruction`，不动用户消息、工具定义和工具名。
- **只删除或改写成中性英文，不插入零宽空格等隐形字符**：11 月 14–15 日那波 403 封号，社区普遍认为与隐形字符混淆被识别有关，CPA 作者也建议删句或改写而不是混淆。

## 改写了什么

1. 每段系统提示词开头的「You are …」身份句改成 `You are a helpful coding assistant.`（Antigravity 自己的身份句保留）。
2. 已公开复现的触发句：
   - PI-Desktop：`You are PI-Desktop, a coding agent.`
   - pi：`You are a coding assistant running inside pi, a coding agent harness.`
   - oh-my-pi (omp)：`RFC 2119 keywords: …` 整行删除；`Read xd://<tool> for full docs + JSON schema before first use.` 改写
   - Claude Code 主代理 / 子代理身份句、Codex 身份句
   - OpenCode 的 `session_move`
3. 系统提示词里的客户端产品名（PI-Desktop、Claude Code、Codex、OpenCode、omp、Cline、Roo Code、Kilo Code、Hermes Agent、ZCode…）替换为 `assistant`。

## 安装（插件市场）

1. 在 CPA 配置里加上本插件的市场源，并打开插件：

   ```yaml
   plugins:
     enabled: true
     store-sources:
       - "https://raw.githubusercontent.com/Sakuralaaa/cpa-antigravity-cloak/main/registry.json"
   ```

   也可以在管理面板的插件市场设置里添加这个链接。

2. 在插件市场找到 **Antigravity Cloak**，点安装并启用。

装好即用，默认配置不需要改。支持 linux / macOS / Windows 的 amd64 与 arm64（Windows 仅 amd64）。

## 可选配置

```yaml
plugins:
  configs:
    antigravity-cloak:
      enabled: true
      identity: "You are a helpful coding assistant."   # 替换身份句用的中性句
      rewrite-identity: true      # 改写开头的 You are… 句
      neutralize-brands: true     # 产品名替换为 assistant
      extra-brands: []            # 追加要替换的产品名
      rules:                      # 追加的 Go RE2 正则，按顺序执行；replacement 为空即删除
        - pattern: 'Some new trigger sentence\.'
          replacement: ""
```

新的触发句出现时，往 `rules` 里加一条即可，不用等插件更新。

## 风险提示

这只能绕开按内容识别的假 429，**不能**消除账号风险。Antigravity 附加条款禁止通过第三方软件访问，11 月已有多波 `403 TOS_VIOLATION` 封号。建议同时在 CPA 里开启：

```yaml
oauth:
  providers:
    antigravity:
      tos-pause-window: "6h"   # 任一号出现 403 TOS_VIOLATION 就整池暂停，防连坐
routing:
  retry:
    request-retry: 0           # 避免一个 429 把整池账号都试一遍
```

## 构建

```bash
go test ./...
CGO_ENABLED=1 go build -buildmode=c-shared -o antigravity-cloak.so .
```

推送 `v*` 标签后，GitHub Actions 会自动构建各平台并发布 Release（含 `checksums.txt`），插件市场从 Release 安装。

## License

MIT
