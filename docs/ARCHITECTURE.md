# 架构与边界

```text
MyGO 桌面窗口（标准系统边框，纯 WebUI）
  └─ URL: "/"：嵌入的 HTML/CSS/JavaScript 工作区
       └─ window.mygo.call('Bridge.Invoke', method, payload)
            └─ app.Service
                 ├─ core：版本登记、元数据、Java、启动计划、进程与日志
                 ├─ plugins.Manager：清单/授权/声明式 UI/动作/钩子
                 │    └─ moejs Runtime → host.call → 受限宿主能力
                 └─ jobs / transfer / safefile：后台任务与可复用 I/O 基础设施
```

浏览器开发模式通过回环 HTTP 调用同一个 Service，不启动另一个业务后端；桌面模式不开放 HTTP 端口。

## 核心职责

`internal/core` 不导入插件管理器、不访问网络、不获取版本目录、不决定安装哪些文件。它维护本地登记、解析标准版本元数据，组合启动计划并以参数数组调用实际可执行文件。

- `store.go`：JSON 原子持久化、导入、基础元数据与插件配置；写入失败回滚内存。
- `metadata.go`：递归继承/循环检测、旧式参数与现代增量合并、库覆盖、Maven 坐标、OS/架构/feature 规则。
- `java.go`：检测 JAVA_HOME、PATH 和常见安装目录，探测 Java 版本/架构；只选择与启动器及原生库架构一致的 JVM，优先匹配元数据主版本。
- `launch.go`：离线身份、旧式/现代参数、文件检查、原生库解压、ZIP-slip 防护。
- `process.go`：启动/停止、输出与错误流、退出码、有限内存日志和磁盘日志。

下载缺失文件不是启动流程的隐式副作用；核心返回明确缺失提示，用户再调用下载插件补全。

## 目录模型

- CLI 从 `os.Executable()` 推导默认 `<EXE 目录>/.minecraft`，`--games` 可独立覆盖；`--data` 仍控制用户配置目录。源码运行脚本显式指定项目内 `.minecraft`，避免 Go 临时二进制路径。
- `app.NewWithMinecraftRoot` / `core.NewStoreWithMinecraftRoot` 显式分离两个根目录；`New` / `NewStore` 保留旧的配置目录内 `minecraft` 默认值，供兼容调用方与测试使用，CLI 不使用该默认值。
- `Version.Directory` 指向该版本所属的共享资源根目录，`Version.GameDirectory` 指向其运行数据目录，不能拿新的默认根目录替换旧登记中的这两个路径。更改默认根目录不迁移文件。
- 导入接受启动器父目录、Minecraft 根目录、`versions` 或单版本目录；读取标准 JSON，按版本本地游戏数据标记识别 PCL/HMCL 风格隔离，不导入第三方账号或私有设置。原版下载插件继续分配 `instances/<实例 ID>` 作为独立游戏数据目录。
- 插件文件能力以 `Store.MinecraftRoot()` 为边界；配置、第三方插件、进程日志及原生库缓存使用 `Store.Root()`。导入外部安装不会自动扩大插件文件权限。

## 插件职责

`plugins/vanilla` 决定官方版本目录、下载计划、平台依赖、资源映射、安装后登记；`plugins/launch-settings` 决定启动参数表单、保存逻辑和钩子行为。它们不是 Go 内核里的两个开关。

插件目录包含清单和单文件 ESM。`internal/plugins` 读取不可变源码快照，验证声明，运行时只暴露显式宿主能力。每次调用新建 runtime，避免跨调用/跨插件 JS 状态泄漏；持久状态归属于宿主。

`internal/transfer` 是通用受限下载服务，不包含 Minecraft 版本选择逻辑。`internal/jobs` 提供后台状态和取消。插件不直接使用操作系统 I/O。

第三方插件由用户显式启用；授权键绑定清单与脚本内容摘要。禁用时不执行钩子、不显示贡献面板，已保存配置保留。后台任务结束前不能热重载/禁用。

## GUI 职责

- `internal/desktop/window.go`：直接加载 WebUI 的 MyGO 窗口与服务桥接；不创建 NativeUI 视图、原生工具栏、状态栏或系统玻璃背景。系统目录选择器和打开目录仍通过受限桥接调用。
- `web/`：无框架静态 HTML/CSS/ESM；左侧导航、版本卡片、每版本工作区、通用插件表单/目录、任务和日志。
- `assets.go` / `plugins/builtin.go`：嵌入静态资源与内置插件，发布 EXE 不依赖源码目录。
- 插件 UI 只能贡献声明式数据，不执行插件提供的 HTML。轮询只更新必要状态，不应覆盖正在编辑的表单。

## 本地接口

`state.get` 返回版本、插件、任务、进程和平台，并分别提供 `dataRoot`（配置）与 `minecraftRoot`（托管游戏）；版本操作为 `versions.import/update/remove/launch/preview/stop`，另有 `logs.get`、`java.detect`、`plugins.toggle/reload/invoke`、`jobs.cancel`。

桌面绑定额外处理 `desktop.pickFolder` / `desktop.openFolder`。Web 模式明确拒绝这两项，不伪装成原生操作。

HTTP `/api` 仅开发使用：回环 IP 监听、精确 Host/Origin 验证、每服务随机页面令牌、无 CORS；不面向互联网。桌面窗口通过 MyGO 内建绑定直连 Go。

## 扩展原则

后续 Forge/Fabric 安装、Mods、截图、整合包管理应各自做插件；必要的通用文件/压缩/列表能力经过独立权限与路径约束后加入宿主。不要将具体功能塞进 `core` 或在 WebUI 中按插件 ID 分支实现整个业务。

当前文件能力限于启动器管理的共享游戏树（默认 EXE 旁 `.minecraft`，或 `--games` 指定的位置），尚不支持编辑该树之外导入版本的 Mods 等场景。增加该能力时应基于实例 ID 和限定的逻辑目录，而不是暴露任意绝对路径读写。

## 安全与验证边界

这是同进程 alpha JS 引擎集成，不是恶意代码隔离产品。时间/内存预算、路径限制、禁用动态代码、JSON DTO 与批准权限是防护层，但不能代替 OS 级 worker 隔离与审计。Java 路径/JVM 参数/下载的游戏代码同样需要信任。

测试覆盖状态与启动计划、实际 OS 子进程、实际 Java 夹具、moejs 调用/权限/取消、受控下载与校验失败、HTTP 授权。真实 Minecraft 兼容性还取决于版本、Java、显卡驱动和网络，不应将夹具测试描述为所有游戏版本通过。
