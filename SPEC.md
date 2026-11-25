# my-vpn 技术规格与验收标准（SPEC）

* **目标仓库**：`https://github.com/mumu-lhl/my-vpn`

* **目标平台**：Linux（主目标 `niri` / Wayland，兼容主流 Wayland/X11 桌面）与 Android（Android 10+ / API 29+，仅 `arm64-v8a`）

* **开发模式**：Linux 与 Android 双端并行开发

## 1. 项目定位与核心目标

`my-vpn` 是一款面向 Linux 与 Android 的跨平台抗封锁代理与 VPN 客户端。系统以 [MyGo](https://mygo.run/docs)（跟踪含 Android 支持的 `main` 分支）为应用壳与前后端桥接框架，以 Svelte 构建统一响应式前端，底层将 [hiddify-sing-box](https://github.com/hiddify/hiddify-sing-box)（v5，基于 sing-box 1.15）与 [geph5-client](https://github.com/geph-official/geph5) 深度整合，提供开箱即用的多协议抗封锁能力、灵活的一级/二级链式代理、细粒度分流与 DNS 防泄露能力。

核心能力范围：

1. **内置抗封锁协议**：迷雾通（Geph5，支持填写既有 `secret` 并拉取出口节点）、赛风（Psiphon）、Cloudflare WARP（与 Hiddify 参数对齐）。

2. **外部节点扩展**：支持导入外部订阅（sing-box JSON、Clash/Mihomo YAML、Base64/单条分享链接）与自建节点，支持定时自动更新与批量延迟测速。

3. **自由链路编排**：支持一级直出代理与二级链式代理（例如“迷雾通/赛风破壁 + WARP 获取干净出口 IP”），并在第一跳仅支持 TCP 时自动将第二跳 WARP 切换为 MASQUE HTTP/2 模式。

4. **多出口模式**：支持 VPN（TUN 全局/分流接管）、本地 SOCKS5、本地 HTTP（Mixed）代理，支持局域网共享代理。

5. **路由与 DNS**：内置国内直连与广告/跟踪器拦截预设，支持高度自定义路由规则与 `.srs` 规则集，支持简洁 DNS 预设与自定义加密上游。

## 2. 总体技术架构

### 2.1 分层组件选型

| 层级 | 技术选型 | 职责与说明 | 
| ----- | ----- | ----- | 
| **前端界面层** | Svelte + TypeScript + Vite | 运行在系统 WebView 中（Linux 为 WebKitGTK 4.1，Android 为 Android WebView），通过 [MyGo 自动生成的 TS 客户端](https://mygo.run/docs/bindings)调用 Go 后端与订阅事件流。一套响应式 SPA 同时适配桌面侧边栏与移动端底部导航栏。 | 
| **应用宿主与控制层** | Go 1.27+ / [MyGo](https://mygo.run/docs)（`main` 分支） | 管理配置持久化、凭证加密存储（Android 使用 Keystore，见 [MyGo 移动端文档](https://mygo.run/docs/mobile)）、订阅更新、测速调度、核心进程编排与前端 RPC 绑定。 | 
| **核心代理引擎** | [hiddify-sing-box](https://github.com/hiddify/hiddify-sing-box) v5 | 纯 Go 代理核心，负责 TUN/SOCKS5/HTTP 入站、DNS 分流、路由规则与 `.srs` 规则集匹配、`detour` 链式转发，以及原生内置 WARP、Psiphon 与主流代理协议。 | 
| **迷雾通 Sidecar** | [geph5-client](https://lib.rs/crates/geph5-client)（Rust 编译产物） | 独立二进制子进程，由 Go 后端托管生命周期。通过 YAML 配置与 `control_listen` RPC 接口连接 Geph Broker、拉取出口国家/城市列表，并在本地暴露 SOCKS5（支持 UDP，见 [Geph 社区说明](https://community.geph.io/t/topic/22357)）供 `hiddify-sing-box` 接入。 | 
| **Linux 平台适配层** | 独立 `core-helper` + `setcap` + `libayatana-appindicator3` | 无 GUI 纯 Go 核心二进制，避免 GTK `AT_SECURE` 提权拦截；通过 SNI 提供 `niri` / Wayland 系统托盘支持。 | 
| **Android 平台适配层** | MyGo Kotlin 原生模块（`VpnService`） | 通过 [MyGo 移动端插件机制](https://mygo.run/docs/mobile/plugins)注入 Kotlin 代码与 `AndroidManifest.xml` `<service>` 声明，负责 VPN 授权弹窗、TUN `fd` 创建、分应用过滤、前台保活通知栏与网络切换监听。 | 

### 2.2 双端关键实现差异

| 维度 | Linux（`niri` / Wayland） | Android（10+ / `arm64-v8a`） | 
| ----- | ----- | ----- | 
| **WebView 运行时** | 系统 GTK 3 + WebKitGTK 4.1（MyGo 默认关闭 DMA-BUF 渲染器以保障稳定性，见 [MyGo 文档](https://mygo.run/docs)） | 系统 Android WebView（前端映射在 `https://mygo.localhost`，交互统一走 Go 绑定方法或 `fetch`，禁止依赖原生 `<form>` POST 与 `localStorage`） | 
| **代理核心运行方式** | 独立无 GUI Go 二进制 `core-helper`，与主 GUI 通过本地 Unix Domain Socket / IPC 通信 | 直接嵌入 MyGo 编译出的 `arm64-v8a` 共享库，与 `VpnService` 协同接管 TUN `fd` | 
| **迷雾通二进制打包** | 随应用分发 `geph5-client`（`linux/amd64`、`linux/arm64`） | 交叉编译为 `arm64-v8a` 并重命名为 `libgeph5.so` 放入 `jniLibs/arm64-v8a/`，从 `nativeLibraryDir` 拉起以绕过数据目录 `noexec` 限制 | 
| **防流量回环机制** | TUN `auto_route` 配合 `default_mark` / `routing_mark` 标记出站套接字绕过 TUN | `VpnService.Builder.addDisallowedApplication(packageName)` 将本应用（含 Go 核心与 `libgeph5.so` 子进程）自身出站流量整体排除在 TUN 外 | 
| **后台常驻与保活** | SNI 系统托盘常驻，关闭窗口时隐藏/释放 WebView，后台仅保持 Go 逻辑与核心运行 | Android Foreground Service + 常驻通知栏（显示当前节点/链路状态与实时上下行速率） | 

## 3. 协议、订阅与链路编排规格

### 3.1 迷雾通（Geph5）

* **认证方式**：用户输入已有的 Geph `secret`（兼容 Free 与 Plus 账号凭证），由 Go 后端安全存储并在生成运行时 YAML 配置时写入 `credentials.secret`。

* **节点拉取与选择**：通过 `geph5-client` 的 `control_listen` RPC 接口或 Broker 协议拉取可用出口列表（按国家/城市分组），支持用户选择 `auto` 或指定国家/城市（映射为 `exit_constraint`，见 [Geph 5.8+ 命令行文档](https://community.geph.io/t/topic/21989)）。

* **核心接入**：`geph5-client` 仅监听回环地址 `127.0.0.1:<随机空闲端口>` 的 SOCKS5 代理，`hiddify-sing-box` 将其注册为内部 `socks` outbound，不直接对外暴露。

### 3.2 赛风（Psiphon）

* 直接使用 `hiddify-sing-box` 内置的 `"type": "psiphon"` outbound（集成官方 `Psiphon-Labs/psiphon-tunnel-core`，见 [Hiddify Psiphon 文档](https://hiddify.com/core/features/protocols/psiphon/)）。

* **支持配置**：出口国家/区域（`egress_region`，支持 Auto 或指定国家代码）、连接超时（`establish_tunnel_timeout`）、数据目录（`data_directory`）、前置代理（`detour`）。

* **协议约束**：Psiphon 仅传输 TCP，不支持 UDP；当作为单跳出口或第一跳时，系统需在 UI 明确标注“仅 TCP”，并触发二级链路自动适配策略。

### 3.3 Cloudflare WARP

* 直接使用 `hiddify-sing-box` 内置的 `"type": "warp"` endpoint（见 [Hiddify WARP 文档](https://hiddify.com/core/features/protocols/warp/)、[WARP over MASQUE 文档](https://hiddify.com/core/features/protocols/warp-masque/) 与 [Hiddify Chain 配置文档](https://hiddify.com/app/Chain-Extra-security-and-Proxy-unblocker/)）。

* **完整支持 Hiddify 对齐配置**：

  * **账号管理**：免配置自动注册免费 WARP 设备、支持填入 WARP+ License Key、支持复用已有设备密钥或强制重新注册。

  * **隧道模式**：`auto`、`wg`（WireGuard）、`awg`（AmneziaWG）、`masque`（HTTP/3 over QUIC 与 HTTP/2 over TLS/TCP）。

  * **抗封锁与优选参数**：

    * WireGuard：Endpoint 模式（Cloudflare 分配 / 扫描优选 / 自定义 Clean IP + Port）、噪声包（次数、大小、延迟及预设模式 Standard / Random / QUIC / DNS / SIP / STUN）、扫描器参数（最大 RTT、超时、候选数、端口与网段范围）。

    * AmneziaWG：垃圾包数量、最小/最大包长度等参数。

    * MASQUE：`vhttp`（`auto` / `h3` / `h2`）、SNI、IPv6 开关、UDP/Keepalive 参数、TLS Fragment 与 ECH。

  * **通用参数**：MTU（默认 Auto）与前置 `detour`。

### 3.4 外部订阅与自建节点

* **支持格式**：

  * 订阅源：sing-box JSON、Clash / Mihomo YAML、通用 Base64 编码节点列表。

  * 单条分享链接与手动录入：`vless://`、`vmess://`、`trojan://`、`ss://`、`hy2://`、`tuic://`、`wg://`。

* **订阅管理**：支持多订阅源分组、自定义更新间隔（定时自动更新 + 手动立即刷新）、节点列表搜索与筛选。

* **延迟测速**：支持单节点测速与分组并发批量测速（基于 HTTP/HTTPS URL Test，展示 RTT 毫秒数与超时状态），支持按延迟排序。

### 3.5 一级与二级链路编排

* **自由组合模型**：

  * **一级代理**：任意一个内置协议（迷雾通 / 赛风 / WARP）或外部节点直接作为最终出口。

  * **二级链路**：用户可任意指定 **第一跳（前置破壁层）** 与 **第二跳（落地/干净出口层）**，例如 `迷雾通 -> WARP`、`赛风 -> WARP`、`外部节点 -> WARP`、`WARP -> 外部节点` 等，底层通过 `hiddify-sing-box` 的 `detour` 字段串联。

* **自动协议兼容切换**：

  * 当第一跳为仅支持 TCP 的协议（如赛风或未开启 UDP 的外部节点）且第二跳选择 WARP 时，系统自动将第二跳 WARP 的隧道模式约束为 **`masque` + `vhttp: h2`（HTTP/2 over TLS/TCP）**，并在界面向用户展示自动切换提示。

  * 当第一跳支持 UDP（如 Geph5 SOCKS5 UDP 或支持 UDP 的自建节点）时，第二跳 WARP 允许按用户配置使用 `auto` / `wg` / `awg` / `masque`。

## 4. 本地入口、路由规则与 DNS 规格

### 4.1 本地入口（出口服务）

* **VPN 模式（TUN）**：全局或按分流规则接管系统流量（Linux 通过 `core-helper` 创建 TUN 网卡，Android 通过 `VpnService` 传入 TUN `fd`）。

* **本地代理模式**：同时提供 SOCKS5 与 HTTP（或 sing-box `mixed`）本地入站端口，端口可自定义。

* **局域网共享代理**：提供独立开关；开启后 SOCKS5/HTTP 入站监听 `0.0.0.0:<port>`，界面展示本机局域网 IP 与端口，且路由规则自动放行私网段（`10.0.0.0/8`、`172.16.0.0/12`、`192.168.0.0/16`、`fc00::/7`）直连，避免局域网客户端回环。

### 4.2 路由规则系统

采用 **“实用预设 + 高度自定义”** 双层架构，底层编译为 sing-box `route.rules` 与 `route.rule_set`：

* **内置实用预设（一键开关 + 一键/定时更新 `.srs` 规则集）**：

  * 中国大陆直连：`geoip-cn` + `geosite-cn` -> `direct`。

  * 广告与跟踪器拦截：`geosite-category-ads-all` -> `reject`。

  * 私有地址与局域网直连：`ip_is_private` -> `direct`。

* **高度自定义能力**：

  * 支持用户添加、编辑、禁用与拖拽排序自定义规则，动作支持 `proxy`（当前主链路）、`direct`、`reject` 或指定具体节点/分组。

  * 匹配维度：域名（精确、后缀、关键词、正则）、IP CIDR、目标端口、传输协议（TCP/UDP）、进程名/包名。

  * 支持导入第三方远程 `.srs`（二进制 rule-set）URL 并设定自动更新周期。

### 4.3 DNS 配置与防泄露设计

采用 **“简洁预设 + 自定义上游”**：

* **简洁预设模式**：

  * 国内直连域名：走国内直连 DNS（如 `223.5.5.5` / `https://dns.alidns.com/dns-query`），避免解析结果漂移。

  * 代理/海外域名：强制经由当前代理链路走加密 DNS（DoH / DoT，如 `https://1.1.1.1/dns-query`、`https://dns.google/dns-query`），彻底杜绝本地运营商 DNS 污染与泄露。

  * **FakeIP 开关**：可选开启 FakeIP（默认在 TUN 模式下推荐开启以提升响应速度并避免 DNS 泄露），支持自定义 FakeIP 排除域名列表。

* **自定义上游**：允许用户覆盖直连 DNS、代理 DNS、Fallback DNS 地址（支持 UDP / TCP / DoH / DoT / DoQ）。

* **防泄露强制策略**：

  * TUN 模式下强制劫持 53 端口 DNS 流量交由核心 DNS 模块处理（`hijack-dns`）。

  * 所有非直连 UDP 流量（含 WebRTC STUN/TURN）全部经由代理链路转发；当当前出口链不支持 UDP 时（如纯 TCP 赛风单跳），对非直连 UDP 流量执行 `reject` 而非回退 `direct`，防止绕过代理泄露真实公网 IP。

  * IPv6 防泄露：当当前代理出口不支持 IPv6 时，自动拦截非直连 IPv6 流量（直连规则匹配的国内 IPv6 正常放行），防止 IPv6 旁路泄露。

## 5. 平台专属实现细节

### 5.1 Linux（`niri` / Wayland）

1. **普通用户默认运行 + 一次性 TUN 提权（路线 A）**：

   * 默认以普通用户态运行：启动 `core-helper` 仅开启本地 SOCKS5/HTTP 入站，并可选自动写入系统代理（通过 `gsettings` 适配 GNOME/GTK 应用，并提供一键复制 `HTTP_PROXY`/`ALL_PROXY` 环境变量片段以适配 Wayland/终端环境）。

   * 首次开启 TUN 模式时，GUI 检测 `core-helper` 是否已具备 `cap_net_admin,cap_net_raw`：若未具备，弹出一次 `pkexec setcap cap_net_admin,cap_net_raw+ep <core-helper路径>`；授权成功后，后续所有 TUN 启停均完全免密。

   * 若软件升级覆盖了 `core-helper` 二进制，GUI 自动检测 capability 缺失并友好提示重新授权一次。

2. **`niri` / Wayland 桌面体验**：

   * 系统托盘依赖 `libayatana-appindicator3`（SNI 协议），确保在 `niri` 搭配 Waybar / SwayNC 等状态栏中正常显示图标与右键菜单（连接/断开、切换模式、打开主窗口、退出）。

   * 窗口关闭默认隐藏至托盘，主窗口关闭后可选释放非必要前端渲染资源，仅在用户从托盘唤醒时恢复。

### 5.2 Android（Android 10+ / `arm64-v8a`）

1. **MyGo Kotlin `VpnService` 原生插件**：

   * 在 `mygo-plugin.json` 中声明 `android.permission.BIND_VPN_SERVICE`、`FOREGROUND_SERVICE`、`FOREGROUND_SERVICE_SPECIAL_USE`（或系统要求的前台服务类型）、`POST_NOTIFICATIONS` 及自定义 `MyVpnService`。

   * 启动 VPN 前通过 `PluginHost.startForResult` 调用 `VpnService.prepare(context)` 获取用户授权；授权后启动前台服务并调用 `VpnService.Builder.establish()`，将 TUN `fd` 与网络状态回调传入 Go 核心。

2. **分应用代理（Per-App VPN）**：

   * 通过 Kotlin 读取已安装应用列表（名称、包名、图标、系统应用标记）回传前端。

   * 支持“全局 / 仅代理选中应用（白名单）/ 绕过选中应用（黑名单）”三种模式，分别映射至 `addAllowedApplication` 与 `addDisallowedApplication`（本应用自身始终强制排除以防回环）。

3. **网络切换自动重连**：

   * 通过 Android `ConnectivityManager.NetworkCallback` 实时监听默认底层网络变化（Wi-Fi ↔ 4G/5G、休眠唤醒）。

   * 网络切换触发时，立即通知 `hiddify-sing-box` 重置出站接口与失效连接池，并在启用迷雾通时触发 `geph5-client` 快速重连，确保切网后在目标阈值内无感恢复。

4. **后台保活通知栏**：

   * 常驻不可滑动清除的前台服务通知，展示连接状态、当前链路（如 `Geph(JP) → WARP`）、实时上下行速率，并提供“断开”快捷按钮。

## 6. 推荐仓库目录结构

```
my-vpn/
├── cmd/
│   ├── app/                  # MyGo 主应用入口（Linux GUI & Android App）
│   └── core-helper/          # Linux 独立无 GUI 代理核心（用于一次性 setcap 跑 TUN）
├── internal/
│   ├── core/                 # hiddify-sing-box 封装（配置编译、启停、热重载、流量统计）
│   ├── geph/                 # geph5-client 进程托管、YAML 生成、control RPC 节点拉取
│   ├── subscription/         # sing-box / Clash / Base64 / URI 订阅解析与定时更新
│   ├── speedtest/            # 并发 URL Test 延迟测速
│   ├── routing/              # 预设规则集管理（.srs 下载更新）与自定义规则编译
│   ├── ipc/                  # Linux GUI <-> core-helper 的 Unix Socket 通信协议
│   ├── storage/              # 配置持久化与敏感凭证加密存储
│   └── platform/             # 平台抽象（Linux setcap/系统代理/托盘，Android VPN 桥接）
├── plugins/
│   └── android-vpn/          # MyGo Kotlin 原生模块（VpnService、分应用、通知栏、网络监听）
│       ├── mygo-plugin.json
│       └── android/          # Kotlin 源码（MyVpnService.kt, NetworkMonitor.kt 等）
├── frontend/                 # Svelte + TypeScript + Vite 响应式前端
│   ├── src/
│   │   ├── lib/mygo.ts       # MyGo 自动生成的 Go 绑定客户端
│   │   ├── routes/           # 仪表盘、链路编排、节点/订阅、路由与 DNS、设置、日志
│   │   └── components/       # 桌面侧边栏 / 移动端底部导航自适应组件
├── third_party/
│   └── geph5-build/          # geph5-client 交叉编译脚本（Linux + Android arm64-v8a）
├── tests/
│   ├── integration/          # Go 集成测试（链路回环、分流、DNS 与泄露检测）
│   ├── e2e-linux/            # 基于 mygo-automation 的 Linux 桌面 E2E 测试
│   └── android-adb/          # Android 真机 ADB 回归与切网重连测试脚本
├── .github/workflows/        # CI 测试与双端产物自动构建（Linux + Android APK）
├── mygo.config.ts            # MyGo 构建与打包配置
└── SPEC.md                   # 本文档
```

## 7. 双端并行开发里程碑

| 阶段 | 核心目标 | 交付物 | 
| ----- | ----- | ----- | 
| **M0：工程脚手架与工具链** | 跑通 MyGo `main` 分支在 Linux 与 Android（`arm64-v8a`）的最小构建；跑通 `geph5-client` 双端交叉编译与 `hiddify-sing-box` Go 依赖引入。 | Linux 可运行骨架 + Android 可安装骨架 APK + CI 基础流水线 | 
| **M1：核心引擎与全协议接入** | 实现 `hiddify-sing-box` 封装、迷雾通 `secret` 配置与节点拉取、赛风与 WARP（含 Hiddify 参数）配置生成、外部订阅解析与测速、一/二级链路编排与自动切 MASQUE H2。 | Go 核心层 + 全套 L1 单元测试与本地回环集成测试 | 
| **M2：双端 TUN 与平台能力** | Linux 实现 `core-helper` IPC、一次性 `setcap` 提权、系统代理与 `niri`/Wayland 托盘；Android 实现 Kotlin `VpnService`、分应用代理、局域网共享、保活通知栏与切网监听。 | Linux TUN/代理全通 + Android VPN/分应用/通知栏全通 | 
| **M3：路由、DNS、防泄露与 UI 闭环** | 完成 Svelte 响应式全页面（含高级路由编辑、DNS 预设/自定义、WARP 高级参数、实时日志与流量面板），完成防 DNS/IPv6/WebRTC(STUN) 泄露加固。 | 完整可用 Beta 版（Linux + Android） | 
| **M4：自动化测试与发布验收** | 补齐 L1/L2/L3 自动化测试与泄露验证脚本，在 `niri`/Wayland 与 Android 10+ 真机执行全量验收，配置 GitHub Actions 自动构建发布产物。 | 通过全部验收标准的 v1.0 候选版本与测试报告 | 

## 8. 可量化验收标准（Acceptance Criteria）

### 8.1 功能与链路验收

| 编号 | 模块 | 验收标准 | 验证方式 | 
| ----- | ----- | ----- | ----- | 
| **AC-01** | 迷雾通（Geph5） | 填入有效 `secret` 后可成功拉取可用出口国家/城市列表；选择任意国家后能建立连接并通过本地 SOCKS5 正常转发流量；切换国家无需重启主应用。 | 集成测试 + 真网实测 | 
| **AC-02** | 赛风与 WARP | 支持按 Hiddify 同等参数配置赛风（区域、超时）与 WARP（自动免费注册、WARP+ Key、WG/AWG/MASQUE、噪声、扫描器、Clean IP、TLS Fragment/ECH），且均能独立连通。 | 配置单元测试 + 真网实测 | 
| **AC-03** | 外部订阅与测速 | 能正确解析 sing-box JSON、Clash/Mihomo YAML、Base64 及 `vless/vmess/trojan/ss/hy2/tuic/wg` 链接；定时自动更新生效；批量 URL Test 返回有效 RTT 并支持排序。 | CI 单元测试（覆盖全格式样例） | 
| **AC-04** | 一级与二级链路 | 支持迷雾通、赛风、WARP、外部节点任意一/二级组合；当第一跳为 TCP-only（如赛风）且第二跳为 WARP 时，系统自动切换 WARP 为 `MASQUE (HTTP/2)` 并在 UI 提示且成功连通。 | 本地回环集成测试 + 真网二级链路实测 | 
| **AC-05** | 路由与 DNS | 预设国内直连与广告拦截规则生效且 `.srs` 可一键/定时更新；自定义规则（含排序、进程/域名/IP/端口/远程 rule-set）生效；DNS 预设与自定义上游（DoH/DoT/FakeIP）生效。 | CI 路由匹配单测 + 抓包验证 | 

### 8.2 防泄露与网络稳定性红线

| 编号 | 指标项 | 量化红线 | 验证方式 | 
| ----- | ----- | ----- | ----- | 
| **AC-06** | DNS 泄露 | TUN 模式下，所有非直连域名的 DNS 查询均不经过本地 ISP DNS，检测结果 **0 条真实 ISP DNS 记录**。 | 自动化抓包检测 + DNS 泄露检测站点实测 | 
| **AC-07** | IP / IPv6 / WebRTC(STUN) 泄露 | TUN 模式下，公网 IPv4 出口与所选最终跳一致；非直连 STUN/UDP 流量全部走代理或被阻断（**0 字节直连泄露**）；代理不支持 IPv6 时非直连 IPv6 被拦截（**0 真实 IPv6 泄露**）。 | 自动化抓包验证 + STUN/IP 泄露检测实测 | 
| **AC-08** | 切网自动重连（单跳） | Android 真机在 Wi-Fi ↔ 4G/5G 切换后，单跳隧道自动恢复连通耗时 **≤ 5 秒**，无需手动开关 VPN。 | Android ADB 切网自动化回归脚本（连续 10 次取 P95） | 
| **AC-09** | 切网自动重连（二级链路） | Android 真机在 Wi-Fi ↔ 4G/5G 切换后，二级链路（如 `迷雾通 -> WARP`）自动恢复连通耗时 **≤ 10 秒**。 | Android ADB 切网自动化回归脚本（连续 10 次取 P95） | 

### 8.3 平台专属验收

| 编号 | 平台 | 验收标准 | 验证方式 | 
| ----- | ----- | ----- | ----- | 
| **AC-10** | Linux 权限与 TUN | 普通用户态可直接运行 SOCKS5/HTTP + 系统代理；首次开启 TUN 仅弹一次 `pkexec` 为 `core-helper` 授予 capabilities；之后连续启停 TUN 与重启应用均 **0 次密码弹窗**；升级覆盖 `core-helper` 后能检测并引导重授。 | Linux 桌面 E2E + 手动验证 | 
| **AC-11** | Linux `niri` / Wayland | 在 `niri` / Wayland 下窗口渲染正常；SNI 系统托盘图标与菜单在 Waybar/SwayNC 中可正常操作；关闭窗口隐藏至托盘且代理不断开。 | `niri` 真机实测 | 
| **AC-12** | Android 系统能力 | Android 10+（`arm64-v8a`）安装运行正常；分应用白名单/黑名单生效；开启局域网共享后同局域网另一设备可通过手机 SOCKS5/HTTP 正常上网；后台运行期间通知栏常驻且实时刷新速率，进程不被系统异常回收。 | Android 真机 ADB 回归脚本 + 局域网联调 | 

### 8.4 自动化测试与 CI 验收

| 编号 | 测试层级 | 验收标准 | 
| ----- | ----- | ----- | 
| **AC-13** | L1 CI 全自动测试 | GitHub Actions 上 Go 单元测试、本地链路回环集成测试、路由/DNS 规则测试与 Svelte 前端测试通过率 **100%**。 | 
| **AC-14** | L2 Linux 桌面 E2E | 基于 `mygo-automation` 的桌面自动化测试通过：覆盖应用启动、配置保存、模式切换、链路切换与启停。 | 
| **AC-15** | L3 真机回归与构建 | 提供可一键执行的 Android ADB 回归脚本与实网连通验证脚本；GitHub Actions 打标签后自动产出 Linux 安装包与 Android `arm64-v8a` APK。 |
