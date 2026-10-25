'use strict';
// Business logic lives in the plugin. The host supplies only capability-checked I/O.
const manifestURL = 'https://piston-meta.mojang.com/mc/game/version_manifest_v2.json';
export function invoke(action, input) {
  if (action === 'catalog') {
    const manifest = host.call('net.json', {url: manifestURL});
    return {latest: manifest.latest, items: manifest.versions.filter(v => v.type === 'release' || v.type === 'snapshot').map(v => ({id: v.id, label: v.id, type: v.type, date: v.releaseTime, description: v.type === 'release' ? '正式版 · Mojang' : '开发快照 · 请备份存档'}))};
  }
  if (action !== 'install') throw new Error('未知下载动作: ' + action);
  const data = input.data || {};
  if (!data.version) throw new Error('请选择一个 Minecraft 版本');
  const name = String(data.name || data.version).trim();
  if (!name || Array.from(name).length > 80) throw new Error('名称须为 1–80 个字符');
  host.call('progress', {phase: '获取官方版本元数据'});
  const manifest = host.call('net.json', {url: manifestURL});
  const selected = manifest.versions.find(v => v.id === data.version);
  if (!selected) throw new Error('该版本不在 Mojang 官方清单中');
  const meta = host.call('net.json', {url: selected.url, sha1: selected.sha1});
  if (!meta.mainClass || !meta.downloads || !meta.downloads.client) throw new Error('此早期版本没有标准客户端入口，暂不支持安装');
  if (meta.id !== data.version) throw new Error('版本元数据 ID 与请求不一致');
  const instance = host.call('versions.allocate', {version: meta.id, name, isolate: data.isolate !== false});
  const platform = host.call('system.platform');
  const files = [], copies = [];
  function add(path, artifact) {
    if (!artifact || !artifact.url || !artifact.sha1) throw new Error('官方元数据缺少可校验的下载项: ' + path);
    files.push({path, url: artifact.url, sha1: artifact.sha1, size: artifact.size || 0});
  }
  add('versions/' + meta.id + '/' + meta.id + '.json', {url: selected.url, sha1: selected.sha1});
  add('versions/' + meta.id + '/' + meta.id + '.jar', meta.downloads.client);
  for (const lib of meta.libraries || []) {
    if (!allowed(lib.rules, platform)) continue;
    const downloads = lib.downloads || {};
    if (downloads.artifact) add('libraries/' + (downloads.artifact.path || mavenPath(lib.name)), downloads.artifact);
    else if (!downloads.classifiers) throw new Error('依赖缺少校验信息: ' + lib.name);
    if (lib.natives && lib.natives[platform.os]) {
      const key = lib.natives[platform.os].replace('${arch}', platform.bits);
      const native = (downloads.classifiers || {})[key];
      if (!native) throw new Error('不支持当前平台的原生库: ' + lib.name + ' / ' + key);
      add('libraries/' + native.path, native);
    }
  }
  if (meta.assetIndex) {
    host.call('progress', {phase: '解析游戏资源索引'});
    const index = host.call('net.json', {url: meta.assetIndex.url, sha1: meta.assetIndex.sha1});
    add('assets/indexes/' + meta.assetIndex.id + '.json', meta.assetIndex);
    for (const key of Object.keys(index.objects || {})) {
      const asset = index.objects[key];
      if (!/^[a-f0-9]{40}$/.test(asset.hash)) throw new Error('非法资源哈希');
      const path = 'assets/objects/' + asset.hash.slice(0, 2) + '/' + asset.hash;
      add(path, {url: 'https://resources.download.minecraft.net/' + asset.hash.slice(0, 2) + '/' + asset.hash, sha1: asset.hash, size: asset.size});
      if (index.virtual) copies.push({from: path, to: 'assets/virtual/' + meta.assetIndex.id + '/' + key});
      if (index.map_to_resources) copies.push({from: path, to: (instance.gamePath ? instance.gamePath + '/' : '') + 'resources/' + key});
    }
  }
  if (meta.logging && meta.logging.client) {
    const log = meta.logging.client.file;
    add('assets/log_configs/' + log.id, log);
  }
  host.call('files.download', {files, phase: '下载并校验客户端、依赖与资源'});
  if (copies.length) {
    host.call('progress', {phase: '准备旧版资源目录'});
    host.call('files.copy', {items: copies});
  }
  host.call('progress', {phase: '登记完整版本'});
  return host.call('versions.commit');
}
function allowed(rules, platform) {
  if (!rules || !rules.length) return true;
  let result = false;
  for (const rule of rules) {
    const os = rule.os || {};
    if (os.name && os.name !== platform.os) continue;
    if (os.arch && ![platform.arch, platform.arch === 'amd64' ? 'x86_64' : platform.arch === '386' ? 'x86' : platform.arch === 'arm64' ? 'aarch64' : platform.arch].includes(os.arch)) continue;
    if (os.version && !new RegExp(os.version).test(platform.version)) continue;
    if (rule.features && Object.keys(rule.features).some(k => rule.features[k] !== false)) continue;
    result = rule.action === 'allow';
  }
  return result;
}
function mavenPath(name) {
  const spec = name.split('@'), p = spec[0].split(':');
  if (p.length < 3 || p.length > 4) throw new Error('非法 Maven 坐标');
  return p[0].replace(/\./g, '/') + '/' + p[1] + '/' + p[2] + '/' + p[1] + '-' + p[2] + (p[3] ? '-' + p[3] : '') + '.' + (spec[1] || 'jar');
}
