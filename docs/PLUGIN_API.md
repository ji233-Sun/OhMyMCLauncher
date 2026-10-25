# 插件协议 v1

本协议由 **OhMyMC 定义**，不是 moejs 上游提供的插件标准。moejs 负责编译与执行 ESM；目录扫描、权限、UI 贡献和启动钩子由启动器实现。

## 1. 安装与信任

将插件放到 `<数据目录>/plugins/<任意目录>/`，包含 `manifest.json` 和同目录 JS 入口。进入“插件”页重新扫描，再检查权限并启用。

- `id` 使用 3–64 位小写字母、数字、`.`、`_`、`-`，首位须为字母/数字；不得覆盖已加载 ID。
- 入口是单个 `.js` 文件，不允许子目录或路径逃逸。清单 ≤128 KiB，入口 ≤1 MiB。
- 用户插件默认禁用；对清单原始字节及 JS 内容计算 SHA-256，内容改变即需要重新授权。
- 内置插件默认启用；都可以禁用。活动后台任务须先取消/结束才能禁用或重新扫描。
- 删除插件目录后重新扫描即可卸载其贡献项；版本配置不会自动迁移或清除。

## 2. 最小可用插件：每版本便笺

`manifest.json`：

```json
{
  "apiVersion": 1,
  "id": "example.notepad",
  "name": "版本便笺",
  "version": "1.0.0",
  "author": "Your name",
  "description": "演示每版本声明式面板和独立存储",
  "entry": "main.js",
  "permissions": ["storage.version"],
  "actions": {"load": {}, "save": {}},
  "hooks": [],
  "panels": [{
    "id": "note",
    "title": "便笺",
    "description": "此便笺只属于当前版本。",
    "scope": "version",
    "kind": "form",
    "loadAction": "load",
    "submitAction": "save",
    "submitLabel": "保存便笺",
    "fields": [{"key": "note", "label": "内容", "type": "textarea"}]
  }]
}
```

`main.js`：

```js
export function invoke(action, input) {
  if (!input.versionId) throw new Error('请选择一个版本');
  if (action === 'load') return {values: host.call('storage.get')};
  if (action === 'save') {
    const note = String((input.data.values || {}).note || '');
    if (note.length > 2000) throw new Error('便笺最多 2000 字符');
    const values = {note};
    host.call('storage.set', values);
    return {values};
  }
  throw new Error('未知动作');
}
```

将该插件启用后，每个版本管理页会自动出现“便笺”。不需要修改前端或核心。

## 3. 执行约定

```js
export function invoke(action, input) { /* 返回 JSON 值，失败时 throw */ }
```

普通动作的 `input`：

```js
{
  versionId: "",         // 全局动作可为空；注意这是启动器实例 ID，不是 MC 版本号
  version: {/* Version */}, // 有 versionId 时提供
  data: {/* UI 传入 */}
}
```

`version` 包括 `id/name/minecraftVersion/versionId/type/notes/directory/gameDirectory/createdAt/lastPlayed/playCount/javaMajor`。此对象只是数据快照，不暴露任意目录访问能力。

- 清单的 `actions` 必须声明可由 UI 调用的动作。`{"background":true}` 交给任务服务运行，立即返回 `{jobId}`；每个插件最多一个活动后台任务。
- **同步 JSON 协议**：不允许返回 Promise，不提供 `fetch`、Node `require`、进程 API 或定时器。长 I/O 通过同步 `host.call` 在宿主后台执行；不要写 `async invoke`。
- 每次调用都创建新的 runtime，顶层变量不跨调用持久化。需要保存的数据用宿主存储。
- 外部静态/动态模块导入不可用，禁用 `eval` / `Function`；开发时可使用自己的打包器输出单文件 ESM。
- 编译预算 2 秒；JS 采样执行预算约 8 秒（只排除真正的宿主工作，参数/输出转换仍计时）；普通动作墙钟 45 秒，后台动作 45 分钟。
- moejs 分配预算 128 MiB；JSON 转换上限 16 MiB / 100 万节点。不是总进程 RSS 硬限制，也不是操作系统级沙箱。
- Go 宿主按 context 取消下载等工作。脚本死循环由 `Interrupt` 中断。

## 4. UI 贡献

`panels` 最多 32 项，每项有唯一 `id`、`title`、`description`，以及清单中已声明的 `loadAction` / `submitAction`。

### form

- `scope: "version"`：显示为版本管理页中的插件面板。
- `scope: "global"`：显示为全局侧栏入口；全局动作没有版本存储上下文。
- `fields` 最多 100 项：`key/label/type/description/placeholder/min/max/required`。
- `type` 支持 `text`、`textarea`、`number`、`checkbox`。字段 key 不得重复或使用 `__proto__`、`constructor`、`prototype`。
- load 返回 `{values: {字段key: 值}}`。
- submit 收到 `input.data = {values: {字段key: 值}}`，建议返回 `{values: ...}`。
- 浏览器约束只改善体验，插件必须验证业务数据；敏感选项宿主会再次验证。

### catalog

首版目录组件面向版本安装类操作。load 返回：

```js
{
  latest: {release: "版本 ID", snapshot: "版本 ID"},
  items: [{id: "版本 ID", label: "显示名称", type: "release", date: "ISO 时间", description: "说明"}]
}
```

UI 提供搜索、正式版/快照筛选与安装确认。submit 收到 `{version: 选中项ID, name: 用户名称, isolate: 布尔值}`。下载动作必须使用后台任务，以便显示进度/取消。

插件不提供 HTML/DOM/远程脚本，文本由宿主转义渲染。若以后添加 Mod 列表、截图网格等，需先扩展带版本号的声明式组件协议，而不是绕过桥接加载任意页面。

## 5. 宿主能力

调用形式：`host.call(method, payload)`，失败会抛出异常。每个有权限要求的方法再次检查插件当前启用状态与清单权限。

| 方法 | 权限 | 参数 / 结果 |
| --- | --- | --- |
| `system.platform` | 无 | 返回 `{os, arch, version, bits}`；`os` 是 `windows/linux/osx`。 |
| `progress` | 无 | `{phase, done, total}`；更新当前后台任务，普通动作时无效果。 |
| `net.json` | `network.minecraft` | `{url, sha1?}`；受限 HTTPS JSON 请求，可校验 SHA-1，返回解码后的 JSON。 |
| `files.download` | `network.minecraft` + `files.minecraft` | `{files:[{path,url,sha1,size}], phase}`；8 个 worker，每个文件最多尝试 3 次，成功后原子提交，已有正确缓存复用。 |
| `files.copy` | `files.minecraft` | `{items:[{from,to}]}`；托管游戏根目录内相对路径复制，用于旧版虚拟资源。 |
| `files.writeJSON` | `files.minecraft` | `{path, value}`；只能写 `.json`，原子提交。 |
| `storage.get` | `storage.version` | 当前 **实例 ID + 插件 ID** 的 JSON 配置，缺失时 `{}`。 |
| `storage.set` | `storage.version` | payload 本身就是配置，≤64 KiB，不可选择别的插件或版本的存储空间。 |
| `launch.defaults` | `launch.configure` | 返回核心默认 `LaunchOptions`。 |
| `launch.validate` | `launch.configure` | payload 为完整 `LaunchOptions`；校验后返回 null，失败抛错。 |
| `versions.allocate` | `versions.add` | `{version, name, isolate}`；本次调用分配一个待登记实例，返回 `{id,gamePath}`，尚不加入版本库。 |
| `versions.commit` | `versions.add` | 检查当前分配版本的元数据/JAR 后登记；每个调用最多一次。下载插件必须先完成全部文件校验再 commit。 |

文件路径统一使用 `/` 分隔、相对 `Store.MinecraftRoot()`（默认 `<启动器 EXE 目录>/.minecraft`，可由 `--games` 覆盖），而非相对配置目录 `--data`。不允许 `..`、绝对路径、Windows ADS 或越界符号链接。`files.minecraft` 能改写该共享树中的内容，是较高风险权限，**不是每插件独立文件沙箱**。如果用户把已有游戏树设为托管根目录，权限也涵盖其中的已有文件；不能借此访问该树之外的导入版本。

`versions.allocate` 的 `gamePath` 同样相对托管根目录：隔离时为 `instances/<实例 ID>`，不隔离时为空字符串。`storage.get/set` 的配置始终保存在 `--data` 下；切换 `--games` 不改变已登记版本的 `directory/gameDirectory`，也不迁移现有文件。

网络只允许 HTTPS/443 和以下官方域名，重定向同样受检查：

```text
piston-meta.mojang.com
piston-data.mojang.com
launchermeta.mojang.com
launcher.mojang.com
libraries.minecraft.net
resources.download.minecraft.net
```

批量下载最多 50,000 项，单项 ≤2 GiB，声明的总量 ≤32 GiB；SHA-1 必填，用于与官方清单比对而非签名认证。未知大小也受单文件读取限制。没有通用 HTTP、任意 shell 或任意文件系统能力。

## 6. 启动钩子

清单同时声明权限 `launch.configure` 和 `hooks:["launch.configure"]`。不必在 `actions` 重复声明钩子。

启动和命令预览都会调用：

```js
invoke('launch.configure', {versionId, version, options})
```

返回完整或部分 `LaunchOptions` 对象；返回字段合并到已有选项。启用插件按 ID 排序依次执行，后面的显式字段覆盖前面的；任何异常阻止启动并向用户显示来源。最后核心再次验证。

```js
{
  playerName: "Player", javaPath: "",
  minMemoryMB: 512, maxMemoryMB: 2048,
  width: 1280, height: 720, fullscreen: false,
  jvmArgs: "", gameArgs: ""
}
```

`jvmArgs` / `gameArgs` 为一行一个参数，不经过 shell。**这不意味着可安全接收恶意参数**：JVM agent、Java 路径等可以执行本机代码。只授予可信插件此权限。

当前不提供 `afterExit`、长期后台服务、跨插件消息、任意目录/进程/账户令牌访问。新能力应在 Go 宿主实现权限检查与 DTO，升级协议后再开放。
