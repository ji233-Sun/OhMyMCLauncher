/* OhMyMC — dependency-free UI. API data is always rendered with text nodes.
   Polls patch stateful views; they never reconstruct an in-progress form. */

const $ = (selector, root = document) => root.querySelector(selector);
const list = value => Array.isArray(value) ? value : [];
const str = value => value == null ? '' : String(value);
const number = value => Number.isFinite(Number(value)) ? Number(value) : 0;
const formatNumber = value => number(value).toLocaleString('zh-CN');
const own = (object, key) => Object.prototype.hasOwnProperty.call(object, key);

function errorMessage(error) {
  return error instanceof Error ? error.message : (typeof error === 'string' ? error : str(error?.message || '操作失败，请重试。'));
}
function unwrap(response) {
  if (response && typeof response === 'object' && response.error) throw new Error(str(response.error));
  return response && typeof response === 'object' && own(response, 'result') ? response.result : response;
}

/** Native MyGO bridge or the authenticated local HTTP development endpoint. */
export async function invoke(method, payload = {}) {
  if (typeof window.mygo?.call === 'function') {
    try { return unwrap(await window.mygo.call('Bridge.Invoke', method, payload)); }
    catch (error) { throw new Error(errorMessage(error)); }
  }
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 45000);
  try {
    let response;
    try {
      response = await fetch('/api', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-OhMyMC-Token': $('meta[name="api-token"]')?.content || '' },
        body: JSON.stringify({ method, payload }),
        signal: controller.signal,
      });
    } catch (error) {
      if (error.name === 'AbortError') throw new Error('请求超时。操作可能仍在后台执行，请刷新状态后确认。');
      throw new Error(`无法连接本地启动器服务。请通过 OhMyMC 或其本地开发服务打开此页面。${errorMessage(error) ? `（${errorMessage(error)}）` : ''}`);
    }
    let data;
    try { data = await response.json(); }
    catch { throw new Error(`服务返回了无效响应（HTTP ${response.status}）。请确认页面由 OhMyMC 本地服务提供。`); }
    if (!response.ok) throw new Error(str(data?.error || `请求失败（HTTP ${response.status}）`));
    if (!data || typeof data !== 'object' || (!own(data, 'result') && !data.error)) throw new Error('服务响应缺少 result，无法读取操作结果。');
    return unwrap(data);
  } finally { clearTimeout(timeout); }
}

const iconPaths = {
  grid: ['M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z'],
  activity: ['M3 12h4l3-8 4 16 3-8h4'],
  terminal: ['M4 4h16v16H4zM7 8l4 4-4 4M13 16h4'],
  puzzle: ['M9 3h4v3a2 2 0 1 0 4 0V3h4v7h-3a2 2 0 1 0 0 4h3v7h-7v-3a2 2 0 1 0-4 0v3H3v-7h3a2 2 0 1 0 0-4H3V3h6'],
  'chevron-right': ['m9 5 7 7-7 7'],
  'arrow-right': ['M4 12h16m-6-6 6 6-6 6'],
  'arrow-left': ['M20 12H4m6-6-6 6 6 6'],
  user: ['M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0M4 21v-2a8 8 0 0 1 16 0v2'],
  info: ['M22 12A10 10 0 1 1 2 12a10 10 0 0 1 20 0M12 11v6M12 7h.01'],
  plus: ['M12 5v14M5 12h14'],
  folder: ['M3 7V5a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z'],
  'folder-open': ['M3 8V5a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2M3 8h17a1 1 0 0 1 1 1l-3 11H3L1 9a1 1 0 0 1 1-1Z'],
  download: ['M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5'],
  search: ['M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0m-2 5 6 6'],
  refresh: ['M20 8A8 8 0 0 0 6 5L3 8m0-5v5h5M4 16a8 8 0 0 0 14 3l3-3m0 5v-5h-5'],
  check: ['m5 12 4 4L19 6'],
  'check-circle': ['M22 12A10 10 0 1 1 2 12a10 10 0 0 1 20 0m-15 0 3 3 7-7'],
  close: ['m6 6 12 12M18 6 6 18'],
  warning: ['m12 3 10 18H2L12 3Zm0 6v5m0 3h.01'],
  disconnected: ['M2 8a17 17 0 0 1 9-3M8 10a11 11 0 0 1 12 2M5 12l2-1M8 16a6 6 0 0 1 8 0M12 20h.01M2 2l20 20'],
  cube: ['m12 2 10 5v10l-10 5-10-5V7l10-5Zm0 10v10M2 7l10 5 10-5M7 4.5l10 5'],
  play: ['m8 4 12 8-12 8V4Z'],
  stop: ['M5 5h14v14H5z'],
  sliders: ['M4 3v5m0 4v9M12 3v9m0 4v5M20 3v2m0 4v12M1 8h6v4H1zM9 12h6v4H9zM17 5h6v4h-6z'],
  copy: ['M9 9h12v12H9zM15 5V3H3v12h2'],
  clock: ['M22 12A10 10 0 1 1 2 12a10 10 0 0 1 20 0M12 6v6l4 2'],
  trash: ['M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7'],
  save: ['M4 3h13l4 4v14H3V3h1Zm3 0v7h10V3M7 21v-7h10v7'],
  shield: ['m12 2 9 4v6c0 5-9 10-9 10S3 17 3 12V6l9-4Zm-5 10 3 3 7-7'],
  code: ['m8 5-7 7 7 7M16 5l7 7-7 7M14 2l-4 20'],
  java: ['M5 9h12v8a4 4 0 0 1-4 4H9a4 4 0 0 1-4-4V9Zm12 1h2a3 3 0 0 1 0 6h-2M8 2v3M12 2v3M16 2v3'],
};
function icon(name) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  for (const [key, value] of Object.entries({ viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-linecap': 'round', 'stroke-linejoin': 'round', class: 'icon', 'aria-hidden': 'true', focusable: 'false' })) svg.setAttribute(key, value);
  for (const d of iconPaths[name] || iconPaths.cube) {
    const path = document.createElementNS(svg.namespaceURI, 'path'); path.setAttribute('d', d); svg.append(path);
  }
  return svg;
}
function h(tag, attributes = {}, ...children) {
  const element = document.createElement(tag);
  for (const [key, value] of Object.entries(attributes || {})) {
    if (value == null) continue;
    if (key.startsWith('on') && typeof value === 'function') element.addEventListener(key.slice(2), value);
    else if (['checked', 'disabled', 'hidden', 'required', 'readOnly', 'multiple', 'autofocus'].includes(key)) element[key] = Boolean(value);
    else if (key === 'value') element.value = str(value);
    else element.setAttribute(key === 'className' ? 'class' : key, str(value));
  }
  for (const child of children.flat(Infinity)) if (child != null && child !== false) element.append(child instanceof Node ? child : document.createTextNode(str(child)));
  return element;
}
function text(element, value) { const next = str(value); if (element.textContent !== next) element.textContent = next; }
function button(label, glyph, className = '', action, attributes = {}) {
  return h('button', { type: 'button', class: `btn ${className}`, ...attributes, onclick: action ? event => action(event.currentTarget, event) : undefined }, glyph ? icon(glyph) : null, h('span', {}, label));
}
function iconButton(label, glyph, action) { return button('', glyph, 'btn-quiet btn-icon', action, { 'aria-label': label, title: label }); }
function link(label, hash, className = '', glyph) { return h('a', { href: hash, class: className }, glyph ? icon(glyph) : null, label); }
function pill(label, status = '') { return h('span', { class: `pill ${status ? `pill-${status}` : ''}` }, label); }
function loading(label = '正在读取…') { return h('div', { class: 'inline-loading', role: 'status' }, h('span', { class: 'spinner', 'aria-hidden': 'true' }), label); }
function inlineError() { return h('div', { class: 'inline-error', role: 'alert', hidden: true }); }
function showError(target, error) { target.hidden = !error; text(target, error ? errorMessage(error) : ''); }
function dateLabel(value, withTime = false, fallback = '—') {
  if (!value || str(value).startsWith('0001-')) return fallback;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return str(value);
  return withTime ? date.toLocaleString('zh-CN', { hour12: false }) : date.toLocaleDateString('zh-CN');
}
function typeLabel(type) { return ({ release: '正式版', snapshot: '快照', old_beta: '远古 Beta', old_alpha: '远古 Alpha' })[type] || str(type || '自定义版本'); }
const statusLabels = { queued: '排队中', running: '进行中', completed: '已完成', failed: '失败', cancelled: '已取消' };
const activeJob = job => job.status === 'queued' || job.status === 'running';
function emptyState(glyph, title, description, action, surface = true) { return h('section', { class: `empty-state ${surface ? 'surface' : ''}` }, icon(glyph), h('h2', {}, title), h('p', {}, description), action); }
function searchControl(label, placeholder, onInput) {
  const input = h('input', { type: 'search', 'aria-label': label, placeholder, autocomplete: 'off', oninput: onInput });
  return { input, node: h('div', { class: 'search-control' }, icon('search'), input) };
}
function field(label, control, hint = '', required = false) {
  const id = control.id || `field-${++uniqueId}`; control.id = id;
  if (hint) control.setAttribute('aria-describedby', `${id}-hint`);
  return h('div', { class: 'field' }, h('label', { for: id }, label, required ? h('span', { class: 'required-mark', 'aria-hidden': 'true' }, '*') : null), control, hint ? h('p', { class: 'field-hint', id: `${id}-hint` }, hint) : null);
}
let uniqueId = 0;

function toast(title, detail = '', kind = 'success') {
  const node = h('div', { class: `toast ${kind === 'error' ? 'toast-error' : ''}`, role: kind === 'error' ? 'alert' : 'status' }, icon(kind === 'error' ? 'warning' : 'check-circle'), h('div', {}, h('strong', {}, title), detail ? h('p', {}, detail) : null));
  node.append(iconButton('关闭通知', 'close', () => node.remove()));
  $('#notifications').append(node);
  // Errors remain available until dismissed. Successful notices pause while focused/hovered.
  if (kind !== 'error') {
    const timer = setInterval(() => { if (!node.matches(':hover') && !node.contains(document.activeElement)) { node.remove(); clearInterval(timer); } }, 6500);
  }
}
async function perform(control, busyLabel, task, { errorTarget, success, detail = '' } = {}) {
  if (control?.dataset.busy) return false;
  let original, wasDisabled;
  if (control) {
    original = [...control.childNodes]; wasDisabled = control.disabled;
    control.dataset.busy = 'true'; control.disabled = true; control.setAttribute('aria-busy', 'true');
    control.replaceChildren(h('span', { class: 'spinner', 'aria-hidden': 'true' }), h('span', {}, busyLabel));
  }
  if (errorTarget) showError(errorTarget, null);
  try {
    await task();
    if (success) toast(success, detail);
    return true;
  } catch (error) {
    if (errorTarget?.isConnected && (!dialog.contains(errorTarget) || dialog.open)) showError(errorTarget, error);
    else toast('操作未完成', errorMessage(error), 'error');
    return false;
  } finally {
    if (control) {
      control.replaceChildren(...original); control.disabled = wasDisabled;
      control.removeAttribute('aria-busy'); delete control.dataset.busy;
    }
    activeView?.update?.();
  }
}
async function copyText(value, control) {
  await perform(control, '复制中', async () => {
    if (navigator.clipboard?.writeText) {
      try { await navigator.clipboard.writeText(str(value)); return; } catch { /* WebView may deny the Clipboard API; try its local selection API. */ }
    }
    const input = h('textarea', { class: 'sr-only', 'aria-label': '待复制文本', value: str(value) });
    const previous = document.activeElement;
    (dialog.open ? dialog : document.body).append(input); input.focus(); input.select();
    let copied = false;
    try { copied = document.execCommand('copy'); } finally { input.remove(); previous?.focus?.(); }
    if (!copied) throw new Error('剪贴板不可用，请选择文本后手动复制。');
  }, { success: '已复制到剪贴板' });
}
function preserveFocusReplace(container, ...children) {
  const focused = container.contains(document.activeElement) ? document.activeElement?.dataset.focusKey : null;
  container.replaceChildren(...children);
  if (focused) [...container.querySelectorAll('[data-focus-key]')].find(node => node.dataset.focusKey === focused)?.focus({ preventScroll: true });
}

let state = { versions: [], plugins: [], jobs: [], processes: [], dataRoot: '', minecraftRoot: '', platform: '', appVersion: '' };
let ready = false, connectionError = '', statePromise = null, dataRevision = 0;
let route = parseRoute(), activeView = null, navigationSignature = '', connectionSignature = '';
const observedJobs = new Map();
const pendingVersions = new Set();
const findVersion = id => state.versions.find(version => version.id === id);
const runningProcess = id => state.processes.find(process => process.versionId === id && process.running);
const versionHash = id => `#version/${encodeURIComponent(id)}`;
const panelHash = (pluginId, panelId, versionId) => versionId ? `${versionHash(versionId)}/panel/${encodeURIComponent(pluginId)}/${encodeURIComponent(panelId)}` : `#panel/${encodeURIComponent(pluginId)}/${encodeURIComponent(panelId)}`;
const logsHash = id => `${versionHash(id)}/logs`;
function globalPanels() { return state.plugins.filter(plugin => plugin.enabled && !plugin.error).flatMap(plugin => list(plugin.panels).filter(panel => panel.scope === 'global').map(panel => ({ plugin, panel }))); }
function versionPanels() { return state.plugins.filter(plugin => plugin.enabled && !plugin.error).flatMap(plugin => list(plugin.panels).filter(panel => panel.scope === 'version').map(panel => ({ plugin, panel }))); }
function downloadDestination() { const target = globalPanels().find(item => item.panel.kind === 'catalog'); return target ? panelHash(target.plugin.id, target.panel.id) : '#plugins'; }
function parseRoute() {
  try {
    const parts = location.hash.slice(1).split('/').map(decodeURIComponent);
    if (parts[0] === 'version' && parts[1]) return { page: 'version', id: parts[1], logs: parts[2] === 'logs', pluginId: parts[2] === 'panel' ? parts[3] : '', panelId: parts[2] === 'panel' ? parts[4] : '' };
    if (parts[0] === 'panel' && parts[1] && parts[2]) return { page: 'panel', pluginId: parts[1], panelId: parts[2] };
    if (parts[0] === 'logs' && parts[1]) return { page: 'version', id: parts[1], logs: true };
    return { page: ['tasks', 'plugins'].includes(parts[0]) ? parts[0] : 'library' };
  } catch { return { page: 'library' }; }
}
function navigate(hash) { if (location.hash === hash) renderRoute(true); else location.hash = hash; }
function commit(change) { dataRevision++; change(state); updateUI(); }
function mergeVersion(version) { if (!version?.id) return; commit(data => { const index = data.versions.findIndex(item => item.id === version.id); if (index < 0) data.versions.push(version); else data.versions[index] = version; }); }
async function refreshState(fresh = false) {
  if (statePromise) return fresh ? statePromise.then(() => refreshState()) : statePromise;
  const revision = dataRevision;
  statePromise = (async () => {
    try {
      const result = await invoke('state.get');
      if (!result || typeof result !== 'object' || !own(result, 'versions')) throw new Error('启动器状态响应不完整，未读取到版本库。');
      if (revision !== dataRevision) return false;
      const next = { ...result, versions: list(result.versions), plugins: list(result.plugins), jobs: list(result.jobs), processes: list(result.processes) };
      for (const job of next.jobs) {
        const before = observedJobs.get(job.id);
        if (before && before !== job.status) {
          if (job.status === 'completed') toast('任务已完成', str(job.title));
          if (job.status === 'failed') toast(`${str(job.title) || '任务'}失败`, str(job.error || '请在下载与任务中查看详情。'), 'error');
        }
        observedJobs.set(job.id, job.status);
      }
      state = next; ready = true; connectionError = ''; updateUI(); return true;
    } catch (error) { connectionError = errorMessage(error); updateUI(); return false; }
  })().finally(() => { statePromise = null; });
  return statePromise;
}
function updateConnection() {
  const signature = `${ready}|${connectionError}`;
  if (signature === connectionSignature) return;
  connectionSignature = signature;
  const status = $('#connection-status');
  status.className = `connection-status ${connectionError ? 'is-offline' : ready ? '' : 'is-pending'}`;
  status.replaceChildren(h('span', { class: 'status-dot' }), h('span', {}, connectionError ? '连接失败' : ready ? '本地服务已连接' : '正在连接'));
  status.title = ready && !connectionError ? '版本、任务与进程状态每 1.5 秒同步' : '未使用模拟数据';
  const alert = $('#connection-alert'); alert.hidden = !connectionError;
  if (connectionError) alert.replaceChildren(icon('disconnected'), h('div', {}, h('strong', {}, ready ? '与启动器的连接已中断，以下为最近一次读取的数据' : '尚未连接到启动器服务'), h('p', {}, connectionError)), button('重试连接', 'refresh', 'btn-sm', control => perform(control, '连接中', () => refreshState(true))));
  else alert.replaceChildren();
}
function updateSidebar() {
  text($('#version-count'), ready ? state.versions.length : '—');
  const tasks = state.jobs.filter(activeJob).length; text($('#task-count'), tasks); $('#task-count').hidden = !tasks;
  text($('#identity-name'), state.playerName || '玩家档案不可用');
  text($('#identity-caption'), state.profileError ? '档案读取失败' : '离线玩家 · 切换档案');
  $('#identity-button').title = state.profileError || `当前玩家：${state.playerName || 'Player'} · 管理玩家档案`;
  text($('#app-version'), state.appVersion ? `v${str(state.appVersion).replace(/^v/, '')}` : 'OhMyMC');
  text($('#platform-label'), state.platform ? str(state.platform).toUpperCase() : 'LOCAL FIRST');
  text($('#workspace-location'), state.minecraftRoot || state.dataRoot || 'OhMyMC / MINECRAFT WORKSPACE');
  const panels = globalPanels();
  const signature = JSON.stringify([ready, panels.map(({ plugin, panel }) => [plugin.id, plugin.name, panel.id, panel.title, panel.kind])]);
  if (signature !== navigationSignature) {
    navigationSignature = signature;
    preserveFocusReplace($('#panel-nav'), ...panels.map(({ plugin, panel }) => h('a', { href: panelHash(plugin.id, panel.id), class: 'nav-link', 'data-panel-key': `${plugin.id}/${panel.id}`, 'data-focus-key': `nav-${plugin.id}/${panel.id}`, title: `${panel.title} · ${plugin.name}` }, h('span', { class: 'nav-icon' }, icon(panel.kind === 'catalog' ? 'download' : panel.kind === 'profiles' ? 'user' : 'sliders')), h('span', { class: 'nav-label' }, panel.title))));
    $('#panel-nav-hint').hidden = Boolean(panels.length);
    text($('#panel-nav-hint'), ready ? '在插件管理中启用扩展面板' : '连接后载入插件面板');
  }
  for (const node of document.querySelectorAll('.sidebar .nav-link')) {
    const selected = node.dataset.panelKey ? route.page === 'panel' && node.dataset.panelKey === `${route.pluginId}/${route.panelId}` : node.dataset.page === (route.page === 'version' ? 'library' : route.page);
    node.classList.toggle('is-active', selected);
    if (selected) node.setAttribute('aria-current', 'page'); else node.removeAttribute('aria-current');
    if (!node.title) node.title = node.querySelector('.nav-label')?.textContent || '';
  }
}
function updateUI() {
  updateConnection(); updateSidebar();
  if (activeView?.awaitReady && ready) renderRoute(false);
  else activeView?.update?.();
}
function setHeader(title, eyebrow, description, actions = [], badge = '') {
  text($('#page-title'), title); text($('#page-eyebrow'), eyebrow); text($('#page-description'), description);
  $('#page-actions').replaceChildren(...actions); text($('#page-title-badge'), badge); $('#page-title-badge').hidden = !badge;
  document.title = `OhMyMC · ${title}`;
}
function renderRoute(focus = false) {
  activeView?.dispose?.(); activeView = null; route = parseRoute();
  const root = $('#view'); root.replaceChildren();
  if (!ready && route.page !== 'library') {
    const title = ({ plugins: '插件管理', tasks: '下载与任务', logs: '运行日志', panel: '插件面板', version: '版本详情' })[route.page];
    setHeader(title, 'LOCAL WORKSPACE', '连接启动器后继续。');
    root.append(emptyState('disconnected', '等待连接本地启动器', '这里只展示实际读取到的内容，不会生成演示版本或任务。', button('重新连接', 'refresh', 'btn-primary', control => perform(control, '连接中', () => refreshState(true)))));
    activeView = { awaitReady: true };
  } else if (route.page === 'library') activeView = renderLibrary(root);
  else if (route.page === 'version') activeView = renderVersion(root, route.id);
  else if (route.page === 'panel') activeView = renderGlobalPanel(root);
  else if (route.page === 'plugins') activeView = renderPlugins(root);
  else if (route.page === 'tasks') activeView = renderTasks(root);
  updateSidebar();
  if (focus && !dialog.open) $('#page-title').focus({ preventScroll: true });
}

function libraryEmpty() {
  const destination = downloadDestination();
  const copy = h('div', { class: 'empty-copy' }, h('div', { class: 'eyebrow' }, '一个新的起点'), h('h2', {}, '你的下一个世界，', h('br'), '从这里开始。'));
  copy.append(h('p', {}, ready ? '启动器会自动识别所在目录中的 .minecraft。' : connectionError ? '连接后，你的版本会在这里呈现。' : '正在连接本地启动器，读取版本与插件。', h('br'), ready ? '也可以让下载插件为你准备第一次冒险。' : '没有演示数据，只有属于你的游戏文件。'));
  if (!ready && !connectionError) copy.append(loading('正在读取工作空间…'));
  else copy.append(h('div', { class: 'empty-actions' }, ready ? link(destination === '#plugins' ? '管理下载插件' : '下载新版本', destination, 'btn btn-primary', 'download') : button('重新连接', 'refresh', '', control => perform(control, '连接中', () => refreshState(true))), ready ? button('重新识别', 'refresh', '', scanVersions) : null));
  copy.append(h('div', { class: 'empty-footnote' }, icon('shield'), '自动登记原路径，保留你的游戏文件'));
  const scene = h('section', { class: 'empty-library surface' }, h('div', { class: 'empty-art' }, h('img', { src: './assets/world.svg', width: '360', height: '300', alt: '' }), h('span', { class: 'art-caption' }, 'A LITTLE SPACE. ENDLESS WORLDS.')), copy);
  const guides = h('div', { class: 'getting-started' },
    h('article', { class: 'guide-card' }, h('span', { class: 'guide-icon' }, icon('folder')), h('div', {}, h('h3', {}, '放在游戏旁，打开就能识别'), h('p', {}, '将启动器放在 .minecraft 所在的文件夹。', h('br'), '启动时自动读取版本，存档与设置留在原处。'), h('p', { class: 'mono' }, state.minecraftRoot || '启动器目录 / .minecraft'))),
    h('article', { class: 'guide-card' }, h('span', { class: 'guide-icon' }, icon('puzzle')), h('div', {}, h('h3', {}, '按自己的方式启动'), h('p', {}, '下载与启动参数由插件提供。', h('br'), '在插件管理中查看来源、能力与权限。'), link('了解已安装插件', '#plugins', 'link-button', 'arrow-right'))));
  return h('div', {}, scene, guides);
}
function renderLibrary(root) {
  setHeader('版本库', 'YOUR COLLECTION', '自动识别启动器旁的游戏，把每一次出发收纳在这里。', [button('重新识别', 'refresh', '', scanVersions), link('下载新版本', downloadDestination(), 'btn btn-primary', 'download')]);
  const count = h('span', { class: 'count-chip' });
  let query = '', sort = 'created', signature = '';
  const search = searchControl('搜索版本名称、Minecraft 版本或备注', '搜索版本名称或版本号', () => { query = search.input.value.trim().toLocaleLowerCase(); update(); });
  const sorting = h('select', { class: 'select', 'aria-label': '版本排序', onchange: event => { sort = event.target.value; update(); } }, h('option', { value: 'created' }, '最近添加'), h('option', { value: 'name' }, '名称排序'), h('option', { value: 'played' }, '最近启动'));
  root.append(h('div', { class: 'library-toolbar' }, h('div', { class: 'section-caption' }, '全部版本', count), h('div', { class: 'toolbar-inputs' }, search.node, sorting)));
  const scanNotice = h('div', { class: 'alert scan-notice', hidden: true });
  const content = h('div'); root.append(scanNotice, content);
  function update() {
    const issues = list(state.scanIssues);
    scanNotice.hidden = !issues.length;
    if (issues.length) scanNotice.replaceChildren(icon('warning'), h('div', {}, h('strong', {}, `${issues.length} 个目录未能识别`), ...issues.map(issue => h('p', {}, `${issue.versionId ? `${issue.versionId}：` : ''}${issue.message}`))));
    text(count, ready ? state.versions.length : '—');
    search.input.disabled = !state.versions.length; sorting.disabled = !state.versions.length;
    const next = JSON.stringify([ready, Boolean(connectionError), state.versions, state.processes, [...pendingVersions], query, sort, downloadDestination()]);
    if (signature === next) return; signature = next;
    if (!state.versions.length) { content.replaceChildren(libraryEmpty()); return; }
    const versions = state.versions.filter(version => [version.name, version.minecraftVersion, version.versionId, version.notes, version.type].some(value => str(value).toLocaleLowerCase().includes(query)));
    versions.sort((a, b) => sort === 'name' ? str(a.name).localeCompare(str(b.name), 'zh-CN') : (Date.parse(sort === 'played' ? b.lastPlayed : b.createdAt) || 0) - (Date.parse(sort === 'played' ? a.lastPlayed : a.createdAt) || 0));
    if (!versions.length) { content.replaceChildren(emptyState('search', '没有找到匹配的版本', '试试其他名称、版本号，或清空搜索条件。', button('清空搜索', 'close', '', () => { query = ''; search.input.value = ''; update(); search.input.focus(); }))); return; }
    const grid = h('div', { class: 'version-grid' });
    for (const [index, version] of versions.entries()) {
      const running = runningProcess(version.id);
      const launch = running ? button('停止', 'stop', 'btn-sm', () => confirmStop(version)) : button('启动游戏', 'play', 'btn-primary btn-sm', control => launchVersion(version, control), { disabled: pendingVersions.has(version.id), 'data-focus-key': `launch-${version.id}` });
      if (!running && pendingVersions.has(version.id)) { launch.setAttribute('aria-busy', 'true'); launch.replaceChildren(h('span', { class: 'spinner', 'aria-hidden': 'true' }), '正在启动'); }
      const card = h('article', { class: 'version-card surface' }, h('div', { class: 'version-card-top' }, h('span', { class: 'version-symbol' }, icon('cube')), running ? pill('运行中', 'running') : pill(typeLabel(version.type))), h('h3', {}, link(version.name || version.id, versionHash(version.id), '', null)), h('p', { class: 'version-subtitle' }, `Minecraft ${version.minecraftVersion || version.versionId || '未知'}`), h('p', { class: 'version-notes' }, version.notes || '还没有备注，给这个世界写一句介绍吧。'), h('p', { class: 'version-card-meta' }, `上次启动 · ${dateLabel(version.lastPlayed, false, '从未启动')}`), h('div', { class: 'version-card-footer' }, link('管理版本', versionHash(version.id), 'link-button', 'sliders'), launch));
      card.style.setProperty('--i', Math.min(index, 6)); grid.append(card);
    }
    preserveFocusReplace(content, grid);
  }
  update(); return { update };
}

const dialog = $('#app-dialog');
let dialogSerial = 0, dialogOpener = null;
dialog.addEventListener('keydown', event => {
  if (event.key !== 'Tab') return;
  const controls = [...dialog.querySelectorAll('button:not(:disabled), input:not(:disabled), textarea:not(:disabled), select:not(:disabled), a[href], [tabindex]:not([tabindex="-1"])')].filter(node => node.getClientRects().length);
  if (!controls.length) { event.preventDefault(); dialog.focus(); return; }
  const first = controls[0], last = controls[controls.length - 1];
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
});
dialog.addEventListener('close', () => { if (!dialog.open) (dialogOpener?.isConnected ? dialogOpener : $('#page-title')).focus({ preventScroll: true }); });
function openDialog(title, description, { wide = false } = {}) {
  const serial = ++dialogSerial;
  if (!dialog.contains(document.activeElement)) dialogOpener = document.activeElement;
  if (dialog.open) dialog.close();
  dialog.classList.toggle('is-wide', wide);
  const body = h('div', { class: 'dialog-body' });
  const footer = h('div', { class: 'dialog-footer' });
  const close = () => { if (serial === dialogSerial && dialog.open) dialog.close(); };
  dialog.replaceChildren(h('header', { class: 'dialog-header' }, h('div', {}, h('h2', { id: 'dialog-title' }, title), h('p', { id: 'dialog-description' }, description)), iconButton('关闭弹窗', 'close', close)), body, footer);
  dialog.setAttribute('aria-describedby', 'dialog-description'); dialog.showModal();
  return { body, footer, close, alive: () => serial === dialogSerial && dialog.open, focus: control => queueMicrotask(() => { if (serial === dialogSerial && dialog.open) control.focus(); }) };
}
function openIdentity() {
  const destination = globalPanels().find(({ panel }) => panel.kind === 'profiles');
  if (destination) { navigate(panelHash(destination.plugin.id, destination.panel.id)); return; }
  const modal = openDialog('本地离线身份', '当前账号能力，保持透明。');
  modal.body.append(h('div', { class: 'alert alert-info' }, icon('user'), h('div', {}, h('strong', {}, '当前仅支持本地离线身份'), h('p', {}, '这不是 Microsoft 或 Minecraft 正版在线登录。离线身份不提供正版服务器的身份验证。'))), h('div', { class: 'import-guide' }, h('strong', {}, '账号功能由插件扩展'), '正版在线登录需等待相应账号插件提供；当前界面没有在线登录或账号绑定功能。可用的启动身份配置以已启用插件面板为准。'));
  const close = button('知道了', 'check', 'btn-primary', modal.close); modal.footer.append(close); modal.focus(close);
}
async function scanVersions(control) {
  await perform(control, '正在识别', async () => {
    const result = await invoke('versions.scan');
    await refreshState(true);
    toast('版本识别完成', `新增 ${list(result?.added).length} 个登记${list(result?.issues).length ? `，${list(result.issues).length} 个目录需要检查` : ''}。`);
  });
}
async function launchVersion(version, control) {
  if (pendingVersions.has(version.id) || runningProcess(version.id)) return;
  pendingVersions.add(version.id);
  await perform(control, '正在启动', async () => {
    activeView?.update?.();
    const process = await invoke('versions.launch', { id: version.id });
    if (process?.versionId) commit(data => { data.processes = data.processes.filter(item => item.versionId !== process.versionId); data.processes.push(process); });
    toast('启动请求已执行', `「${version.name}」的运行状态和输出可在日志中查看。`);
    await refreshState(true);
  });
  pendingVersions.delete(version.id); activeView?.update?.();
}
function confirmation({ title, description, explanation, label, action, success }) {
  const modal = openDialog(title, description);
  const error = inlineError();
  modal.body.append(h('div', { class: 'alert' }, icon('warning'), h('p', {}, explanation)), error);
  const cancel = button('取消', null, '', modal.close);
  modal.footer.append(cancel, button(label, null, 'btn-danger-solid', control => perform(control, '处理中', async () => { await action(); modal.close(); toast(success); await refreshState(true); }, { errorTarget: error })));
  modal.focus(cancel);
}
function confirmStop(version) {
  confirmation({ title: '停止正在运行的游戏？', description: str(version.name), explanation: '这会结束该版本的游戏进程。未保存的游戏进度可能丢失；建议先在游戏内保存并退出。', label: '确认停止', action: async () => { await invoke('versions.stop', { id: version.id }); }, success: '已发送停止请求' });
}
function confirmRemove(version) {
  confirmation({ title: '从版本库移除？', description: str(version.name), explanation: '仅移除版本记录，保留游戏文件、存档和游戏设置。下次启动时不会自动加回；点击版本库的「重新识别」可以恢复当前目录下的登记。', label: '移除记录', action: async () => { await invoke('versions.remove', { id: version.id }); commit(data => { data.versions = data.versions.filter(item => item.id !== version.id); }); navigate('#library'); }, success: '已移除版本记录，磁盘文件保持不变' });
}
function openPreview(version) {
  const modal = openDialog('启动命令预览', '只展示启动器生成的参数，不会启动游戏。', { wide: true });
  let command = '';
  const copy = button('复制预览', 'copy', 'btn-primary', control => copyText(command, control), { disabled: true });
  const refresh = button('重新生成', 'refresh', '', control => perform(control, '生成中', load));
  modal.footer.append(button('关闭', null, '', modal.close), refresh, copy);
  async function load() {
    copy.disabled = true; modal.body.replaceChildren(loading('正在解析 Java 与启动参数…'));
    try {
      const result = await invoke('versions.preview', { id: version.id });
      if (!modal.alive()) return;
      if (!result?.executable || !Array.isArray(result.args)) throw new Error('启动命令响应不完整。');
      command = [JSON.stringify(result.executable), ...result.args.map(argument => JSON.stringify(str(argument)))].join(' ');
      modal.body.replaceChildren(h('div', { class: 'command-meta' }, h('div', { class: 'info-label' }, '工作目录'), h('p', {}, result.workingDirectory || '未提供')), h('div', { class: 'command-meta' }, h('div', { class: 'info-label' }, 'Java 要求'), h('p', {}, result.javaMajor ? `Java ${result.javaMajor}` : '由启动器解析')), h('pre', { class: 'command-preview', tabindex: '0', 'aria-label': '启动命令展示' }, command), h('p', { class: 'field-hint' }, '参数使用 JSON 引号展示，仅用于核对。不同终端的转义规则不同，请勿直接当作终端脚本执行。'));
      for (const warning of list(result.warnings)) modal.body.append(h('p', { class: 'field-hint' }, warning));
      copy.disabled = false;
    } catch (error) { if (modal.alive()) { const notice = inlineError(); showError(notice, error); modal.body.replaceChildren(notice); } }
  }
  perform(refresh, '生成中', load);
}
function openJava() {
  const modal = openDialog('本机 Java 环境', '检测已安装的 Java；不会自动下载或更改环境。', { wide: true });
  const refresh = button('重新检测', 'refresh', '', control => perform(control, '检测中', load));
  modal.footer.append(button('关闭', null, '', modal.close), refresh);
  async function load() {
    modal.body.replaceChildren(loading('正在检测 Java 安装路径…'));
    try {
      const result = list(await invoke('java.detect'));
      if (!modal.alive()) return;
      modal.body.replaceChildren(result.length ? h('div', { class: 'java-list' }, ...result.map(java => h('article', { class: 'java-item' }, h('h3', {}, java.major ? `Java ${java.major}` : 'Java'), h('p', {}, java.version || '未提供版本信息'), h('p', { class: 'mono' }, java.path)))) : emptyState('java', '没有检测到 Java', '请先在本机安装版本所需的 Java，再重新检测。启动参数插件可提供自定义路径设置。', null, false));
    } catch (error) { if (modal.alive()) { const notice = inlineError(); showError(notice, error); modal.body.replaceChildren(notice); } }
  }
  perform(refresh, '检测中', load);
}

function renderVersion(root, id) {
  const version = findVersion(id);
  if (!version) {
    setHeader('版本不可用', 'VERSION WORKSPACE', '该记录不存在或已从版本库移除。');
    root.append(emptyState('cube', '找不到这个版本', '磁盘文件不会因记录移除而被删除，你可以返回版本库重新导入。', link('返回版本库', '#library', 'btn btn-primary', 'arrow-left')));
    return {};
  }
  const launch = button('启动游戏', 'play', 'btn-primary', control => launchVersion(findVersion(id), control));
  const stop = button('停止游戏', 'stop', 'btn-danger', () => confirmStop(findVersion(id)));
  setHeader(version.name || version.id, 'VERSION WORKSPACE', `Minecraft ${version.minecraftVersion || version.versionId || '未知版本'} · ${typeLabel(version.type)}`, [launch, stop], version.minecraftVersion || '');
  root.append(h('nav', { class: 'version-navigation', 'aria-label': '版本导航' }, link('返回版本库', '#library', 'btn btn-quiet btn-sm back-link', 'arrow-left'), h('span', { class: 'navigation-divider', 'aria-hidden': 'true' }), h('span', { class: 'navigation-current' }, version.name || version.id)));
  const tabs = h('nav', { class: 'tabs', 'aria-label': '版本详情选项卡' }); root.append(tabs);
  const body = h('div'); root.append(body); let tabSignature = '';
  const child = route.logs ? renderLogs(body, id) : route.panelId ? renderPluginPanel(body, route.pluginId, route.panelId, id) : renderOverview(body, id);
  function update() {
    const current = findVersion(id);
    if (!current) { renderRoute(false); return; }
    text($('#page-title'), current.name || current.id); document.title = `OhMyMC · ${current.name || current.id}`;
    text($('#page-description'), `Minecraft ${current.minecraftVersion || current.versionId || '未知版本'} · ${typeLabel(current.type)}`);
    const running = Boolean(runningProcess(id)); launch.hidden = running; stop.hidden = !running;
    if (!launch.dataset.busy) launch.disabled = pendingVersions.has(id);
    const panels = versionPanels();
    const signature = JSON.stringify(panels.map(({ plugin, panel }) => [plugin.id, plugin.name, panel.id, panel.title]));
    if (signature !== tabSignature || !tabs.childElementCount) {
      tabSignature = signature;
      const overview = link('基础概览', versionHash(id), `tab ${!route.panelId && !route.logs ? 'is-selected' : ''}`, 'grid');
      if (!route.panelId && !route.logs) overview.setAttribute('aria-current', 'page');
      const logs = link('运行日志', logsHash(id), `tab ${route.logs ? 'is-selected' : ''}`, 'terminal');
      if (route.logs) logs.setAttribute('aria-current', 'page');
      preserveFocusReplace(tabs, overview, ...panels.map(({ plugin, panel }) => {
        const selected = route.pluginId === plugin.id && route.panelId === panel.id;
        return h('a', { href: panelHash(plugin.id, panel.id, id), class: `tab ${selected ? 'is-selected' : ''}`, 'aria-current': selected ? 'page' : null, 'data-focus-key': `tab-${plugin.id}/${panel.id}` }, icon('puzzle'), panel.title, h('span', { class: 'tab-source' }, plugin.name));
      }), logs);
    }
    child?.update?.();
  }
  update(); return { update, poll: () => child?.poll?.(), dispose: () => child?.dispose?.() };
}
function cardHeading(title, description, glyph) { return h('div', { class: 'card-header' }, h('div', {}, h('h2', {}, title), description ? h('p', {}, description) : null), glyph ? h('span', { class: 'card-title-icon' }, icon(glyph)) : null); }
function renderOverview(root, id) {
  const version = findVersion(id); let dirty = false;
  const name = h('input', { type: 'text', value: version.name, required: true, maxlength: '180', autocomplete: 'off' });
  const notes = h('textarea', { rows: '4', placeholder: '记录这个版本的用途、存档或特别之处…', value: version.notes || '' });
  const error = inlineError();
  const status = h('span', { class: 'form-status', role: 'status' }, '修改仅影响此启动器的显示');
  const save = button('保存资料', 'save', 'btn-primary', null, { type: 'submit', disabled: true });
  const form = h('form', { class: 'surface card' }, cardHeading('版本资料', '让每一个版本都容易辨认。', 'cube'), field('显示名称', name, '', true), field('版本说明', notes), error, h('div', { class: 'form-footer' }, status, save));
  const onChange = () => { dirty = true; text(status, '有尚未保存的修改'); status.classList.remove('is-success'); if (!save.dataset.busy) save.disabled = !name.value.trim(); };
  name.addEventListener('input', onChange); notes.addEventListener('input', onChange);
  form.addEventListener('submit', event => {
    event.preventDefault(); if (!name.value.trim() || !form.reportValidity()) return;
    const submitted = { name: name.value.trim(), notes: notes.value };
    perform(save, '保存中', async () => {
      const result = await invoke('versions.update', { id, ...submitted });
      if (!result?.id) throw new Error('保存响应不完整，请刷新版本状态后确认。');
      // An edit made while saving is still a draft, not overwritten by the response.
      dirty = name.value.trim() !== submitted.name || notes.value !== submitted.notes;
      mergeVersion(result);
      text(status, dirty ? '已保存上次提交；仍有新修改' : '资料已保存'); status.classList.toggle('is-success', !dirty);
      await refreshState(true);
    }, { errorTarget: error });
  });
  const info = h('dl', { class: 'info-grid' });
  const rows = {};
  for (const [key, label] of [['minecraftVersion', 'Minecraft 版本'], ['versionId', '底层版本 ID'], ['type', '版本类型'], ['javaMajor', 'Java 要求'], ['createdAt', '添加时间'], ['lastPlayed', '上次启动'], ['playCount', '启动次数'], ['id', '记录 ID']]) {
    rows[key] = h('dd'); info.append(h('div', {}, h('dt', {}, label), rows[key]));
  }
  const pathNodes = {};
  const paths = h('div', { class: 'path-list' });
  for (const [key, label] of [['directory', '版本文件目录'], ['gameDirectory', '游戏工作目录']]) {
    const value = h('p'); pathNodes[key] = value;
    paths.append(h('div', { class: 'path-row' }, h('div', {}, h('div', { class: 'info-label' }, label), value), iconButton(`复制${label}`, 'copy', control => copyText(findVersion(id)?.[key] || '', control))));
  }
  const folder = button('打开游戏目录', 'folder-open', 'btn-sm', control => perform(control, '打开中', () => invoke('desktop.openFolder', { id }), { success: '已请求打开游戏目录' }));
  const metadata = h('section', { class: 'surface card' }, cardHeading('安装信息', '实际文件路径与启动记录。', 'folder'), info, paths, h('div', { class: 'form-footer' }, folder));
  const launchTitle = h('strong'); const launchHint = h('small');
  const launch = button('启动游戏', 'play', 'btn-primary', control => launchVersion(findVersion(id), control));
  const stop = button('停止游戏', 'stop', 'btn-danger', () => confirmStop(findVersion(id)));
  const launchCard = h('section', { class: 'surface card launch-box' }, cardHeading('准备出发', '使用此版本的实际启动配置。'), h('div', { class: 'launch-readiness' }, icon('java'), h('div', {}, launchTitle, launchHint)), launch, stop, h('div', { class: 'launch-tools' }, button('预览命令', 'code', 'btn-quiet btn-sm', () => openPreview(findVersion(id))), button('检测 Java', 'java', 'btn-quiet btn-sm', openJava), link('查看日志', logsHash(id), 'link-button', 'terminal')));
  const player = h('strong', { class: 'current-player' });
  const account = h('section', { class: 'surface card' }, cardHeading('当前玩家档案', '所有版本使用同一个选中档案。', 'user'), player, h('p', { class: 'field-hint' }, '本地离线身份，无法通过正版服务器验证。切换名称会改变离线 UUID，可能影响存档中的玩家归属。'), button('管理玩家档案', 'arrow-right', 'btn-quiet btn-sm', openIdentity));
  root.append(h('div', { class: 'detail-layout' }, h('div', { class: 'stack' }, form, metadata), h('div', { class: 'stack' }, launchCard, account)), h('div', { class: 'danger-zone' }, h('div', {}, h('h3', {}, '不再需要这个版本？'), h('p', {}, '只移除版本库记录，不删除游戏文件与存档。')), button('移除记录', 'trash', 'btn-quiet', () => confirmRemove(findVersion(id)))));
  function update() {
    const current = findVersion(id); if (!current) return;
    text(player, state.playerName || '玩家档案不可用');
    if (!dirty && !save.dataset.busy) { if (name.value !== current.name) name.value = str(current.name); if (notes.value !== str(current.notes)) notes.value = str(current.notes); }
    if (!save.dataset.busy) save.disabled = !dirty || !name.value.trim();
    for (const key of Object.keys(rows)) {
      let value = current[key];
      if (key === 'type') value = typeLabel(value);
      if (key === 'javaMajor') value = value ? `Java ${value}` : '由启动器解析';
      if (key === 'createdAt' || key === 'lastPlayed') value = dateLabel(value, true, key === 'lastPlayed' ? '从未启动' : '—');
      if (key === 'playCount') value = `${formatNumber(value)} 次`;
      text(rows[key], value || '—');
    }
    for (const key of Object.keys(pathNodes)) text(pathNodes[key], current[key] || '未提供');
    const process = runningProcess(id); launch.hidden = Boolean(process); stop.hidden = !process;
    if (!launch.dataset.busy) launch.disabled = pendingVersions.has(id);
    text(launchTitle, process ? `游戏运行中 · PID ${process.pid}` : current.javaMajor ? `需要 Java ${current.javaMajor}` : 'Java 由启动器解析');
    text(launchHint, process ? `开始于 ${dateLabel(process.startedAt, true)}` : '启动前会由后端检查环境与参数');
  }
  update(); return { update };
}

function renderGlobalPanel(root) {
  const plugin = state.plugins.find(item => item.id === route.pluginId);
  const panel = list(plugin?.panels).find(item => item.id === route.panelId);
  setHeader(panel?.title || '插件面板', panel?.kind === 'catalog' ? 'FIND YOUR NEXT WORLD' : 'PLUGIN WORKSPACE', panel?.description || '由已安装插件提供的扩展工作区。');
  return renderPluginPanel(root, route.pluginId, route.panelId);
}
function renderPluginPanel(root, pluginId, panelId, versionId) {
  const plugin = state.plugins.find(item => item.id === pluginId);
  const panel = list(plugin?.panels).find(item => item.id === panelId);
  const unavailable = !plugin?.enabled || Boolean(plugin?.error) || !panel || panel.scope !== (versionId ? 'version' : 'global');
  if (unavailable) {
    root.append(emptyState('puzzle', '此插件面板暂不可用', plugin?.error || '插件可能已停用、重新加载，或此面板不适用于当前版本。', link('查看插件状态', '#plugins', 'btn', 'puzzle')));
    const signature = JSON.stringify(plugin);
    return { update: () => { if (JSON.stringify(state.plugins.find(item => item.id === pluginId)) !== signature) renderRoute(false); } };
  }
  root.append(h('section', { class: 'plugin-source', 'aria-label': '面板来源' }, icon('puzzle'), h('div', {}, h('h2', {}, panel.title), h('p', {}, `由 ${plugin.name} ${plugin.version ? `v${str(plugin.version).replace(/^v/, '')}` : ''} 提供${panel.description ? ` · ${panel.description}` : ''}`)), pill(plugin.builtin ? '内置插件' : '用户插件')));
  const content = h('div'); root.append(content);
  let child;
  if (panel.kind === 'catalog') child = renderCatalog(content, plugin, panel, versionId);
  else if (panel.kind === 'form') child = renderPluginForm(content, plugin, panel, versionId);
  else if (panel.kind === 'launch') child = renderLaunchSettings(content, plugin, panel, versionId);
  else if (panel.kind === 'profiles') child = renderProfiles(content, plugin, panel);
  else { content.append(emptyState('warning', '不支持的插件面板类型', `此插件声明了未知类型：${panel.kind}。请更新或检查插件。`, link('管理插件', '#plugins', 'btn'))); child = {}; }
  const signature = JSON.stringify([plugin.enabled, plugin.error, panel]);
  return { update() {
    const current = state.plugins.find(item => item.id === pluginId);
    const currentPanel = list(current?.panels).find(item => item.id === panelId);
    if (JSON.stringify([current?.enabled, current?.error, currentPanel]) !== signature) { renderRoute(false); return; }
    child?.update?.();
  }, dispose: () => child?.dispose?.() };
}
function invokePanel(plugin, panel, action, versionId, data) {
  if (!action) return Promise.reject(new Error(`插件「${plugin.name}」未声明此面板的操作名称。`));
  const payload = { id: plugin.id, action };
  if (versionId) payload.versionId = versionId;
  if (data !== undefined) payload.data = data;
  return invoke('plugins.invoke', payload);
}
function showJobResult(target, result, completedMessage = '操作已完成') {
  if (result?.jobId) {
    observedJobs.set(result.jobId, 'queued');
    target.hidden = false;
    target.replaceChildren(h('span', {}, '任务已提交，实际进度将在任务工作区更新。'), link('查看下载与任务', '#tasks', '', 'arrow-right'));
    toast('任务已创建', '完成后版本库会自动同步。');
  } else { target.hidden = true; target.replaceChildren(); toast(completedMessage); }
}
function renderCatalog(root, plugin, panel, versionId) {
  let items = [], latest = {}, selected = null, filter = 'release', query = '', limit = 100, loadingNow = false, loadError = '', nameTouched = false, disposed = false;
  const search = searchControl('搜索可安装版本', '搜索 Minecraft 版本…', () => { query = search.input.value.trim().toLocaleLowerCase(); limit = 100; drawList(); });
  const count = h('span', { class: 'catalog-count', role: 'status' });
  const filters = h('div', { class: 'segmented', 'aria-label': '版本类型筛选' });
  for (const [value, label] of [['release', '正式版'], ['snapshot', '快照'], ['all', '全部']]) filters.append(h('button', { type: 'button', 'aria-pressed': value === filter, onclick: event => { filter = value; limit = 100; for (const item of filters.children) item.setAttribute('aria-pressed', String(item === event.currentTarget)); drawList(); } }, label));
  const catalogList = h('div', { class: 'catalog-list', 'aria-label': '可安装版本列表' });
  const more = h('div', { class: 'catalog-more', hidden: true });
  const reload = button('刷新目录', 'refresh', 'btn-sm', control => perform(control, '读取中', load));
  const listError = inlineError();
  const selection = h('div', { class: 'selection-value', 'aria-live': 'polite' }, '尚未选择');
  const name = h('input', { type: 'text', required: true, placeholder: '先在左侧选择一个版本', autocomplete: 'off', maxlength: '180' });
  name.addEventListener('input', () => { nameTouched = Boolean(name.value); updateSubmit(); });
  const isolate = h('input', { type: 'checkbox', checked: true });
  const error = inlineError(); const resultBox = h('div', { class: 'job-link', hidden: true });
  const submit = button(panel.submitLabel || '安装版本', 'download', 'btn-primary', null, { type: 'submit', disabled: true });
  const form = h('form', { class: 'surface card install-card' }, cardHeading('安装设置', '选一个版本，再为它取个名字。', 'download'), h('div', { class: 'selection-box' }, h('div', { class: 'info-label' }, '所选 Minecraft 版本'), selection), field('安装名称', name, '用于在版本库中辨认此安装。', true), h('label', { class: 'checkbox-row' }, isolate, h('span', {}, h('strong', {}, '使用独立游戏目录'), h('small', {}, '隔离存档、设置与资源，避免版本之间相互影响。'))), submit, error, resultBox);
  form.addEventListener('submit', event => {
    event.preventDefault(); if (!selected || !name.value.trim() || !form.reportValidity()) return;
    const data = { version: selected.id, name: name.value.trim(), isolate: isolate.checked };
    perform(submit, '正在提交', async () => { const result = await invokePanel(plugin, panel, panel.submitAction, versionId, data); showJobResult(resultBox, result, '插件安装操作已完成'); await refreshState(true); }, { errorTarget: error });
  });
  root.append(h('div', { class: 'workspace-toolbar' }, h('span', { class: 'section-caption' }, '选择游戏版本'), reload), h('div', { class: 'catalog-layout' }, h('section', { class: 'surface catalog-browser' }, h('div', { class: 'catalog-controls' }, search.node, h('div', { class: 'catalog-filter-row' }, filters, count)), listError, catalogList, more), form));
  function updateSubmit() { if (!submit.dataset.busy) submit.disabled = loadingNow || Boolean(loadError) || !selected || !name.value.trim() || !panel.submitAction; if (!reload.dataset.busy) reload.disabled = loadingNow; }
  function choose(item, control) {
    selected = item; text(selection, item.label || item.id);
    if (!nameTouched) name.value = str(item.id);
    for (const row of catalogList.querySelectorAll('.catalog-item')) row.setAttribute('aria-pressed', String(row === control));
    showError(error, null); updateSubmit();
  }
  function drawList() {
    updateSubmit();
    if (loadingNow) { catalogList.replaceChildren(loading('正在读取插件提供的版本目录…')); text(count, '读取中'); more.hidden = true; return; }
    if (loadError) {
      catalogList.replaceChildren(emptyState('disconnected', '版本目录暂不可用', '请检查网络、插件状态和权限后重试。', button('重新读取', 'refresh', '', control => perform(control, '读取中', load)), false)); text(count, '读取失败'); more.hidden = true; return;
    }
    const filtered = items.filter(item => (filter === 'all' || item.type === filter) && [item.id, item.label, item.description].some(value => str(value).toLocaleLowerCase().includes(query)));
    text(count, `${filtered.length} 个版本`);
    if (!filtered.length) { catalogList.replaceChildren(emptyState('search', items.length ? '没有匹配的版本' : '目录暂时为空', items.length ? '更改搜索词，或切换至「全部」查看其他版本类型。' : '插件未返回可安装版本，请尝试刷新目录。', null, false)); more.hidden = true; return; }
    preserveFocusReplace(catalogList, ...filtered.slice(0, limit).map(item => {
      const detail = [typeLabel(item.type), item.date ? dateLabel(item.date) : '', item.description].filter(Boolean).join(' · ');
      return h('button', { type: 'button', class: 'catalog-item', 'aria-pressed': selected?.id === item.id, 'data-focus-key': `catalog-${item.id}`, onclick: event => choose(item, event.currentTarget) }, icon('cube'), h('div', {}, h('strong', {}, item.label || item.id), h('p', {}, detail)), item.id === latest.release || item.id === latest.snapshot ? pill('最新') : null, h('span', { class: 'selection-indicator', 'aria-hidden': 'true' }, icon('check')));
    }));
    more.hidden = filtered.length <= limit;
    more.replaceChildren(button(`再显示 ${Math.min(100, filtered.length - limit)} 个`, 'plus', 'btn-quiet btn-sm', () => { limit += 100; drawList(); }));
  }
  async function load() {
    if (disposed || loadingNow) return;
    loadingNow = true; loadError = ''; showError(listError, null); drawList();
    try {
      const result = await invokePanel(plugin, panel, panel.loadAction, versionId);
      if (disposed) return;
      if (!result || !Array.isArray(result.items)) throw new Error('插件未返回有效的版本目录（items）。');
      items = result.items; latest = result.latest || {};
      if (selected && !items.some(item => item.id === selected.id)) { selected = null; text(selection, '尚未选择'); }
    } catch (failure) { loadError = errorMessage(failure); if (!disposed) showError(listError, failure); }
    finally { loadingNow = false; if (!disposed) drawList(); }
  }
  perform(reload, '读取中', load);
  return { update: updateSubmit, dispose: () => { disposed = true; } };
}
function renderPluginForm(root, plugin, panel, versionId) {
  let disposed = false, loaded = false, dirty = false, loadingNow = false;
  const controls = new Map(); const error = inlineError();
  const status = h('span', { class: 'form-status', role: 'status' }, '正在读取配置…');
  const fields = h('div'); const resultBox = h('div', { class: 'job-link', hidden: true });
  const save = button(panel.submitLabel || '保存配置', 'save', 'btn-primary', null, { type: 'submit', disabled: true });
  const reload = button('重新读取', 'refresh', 'btn-sm', control => {
    if (dirty) confirmation({ title: '丢弃尚未提交的修改？', description: panel.title, explanation: '重新读取会用插件当前配置覆盖这份未保存的表单。', label: '丢弃并重新读取', action: async () => { await load(); if (!loaded) throw new Error(error.textContent || '未能重新读取配置。'); }, success: '已重新读取配置' });
    else perform(control, '读取中', load);
  });
  const form = h('form', { class: 'surface card plugin-form' }, cardHeading(panel.title, '此表单由插件声明，修改只在提交后生效。', 'sliders'), fields, error, resultBox, h('div', { class: 'form-footer' }, status, reload, save));
  root.append(form);
  const descriptors = list(panel.fields);
  let unsupported = false;
  for (const descriptor of descriptors) {
    if (!['text', 'number', 'textarea', 'checkbox'].includes(descriptor.type) || !descriptor.key || controls.has(descriptor.key)) { unsupported = true; continue; }
    const id = `plugin-field-${++uniqueId}`;
    const control = descriptor.type === 'textarea' ? h('textarea', { id, rows: '5', placeholder: descriptor.placeholder || '', required: descriptor.required }) : h('input', { id, type: descriptor.type, placeholder: descriptor.placeholder || '', required: descriptor.required, autocomplete: 'off' });
    if (descriptor.type === 'number') {
      if (descriptor.min != null) control.min = str(descriptor.min);
      if (descriptor.max != null) control.max = str(descriptor.max);
      control.step = 'any';
    }
    control.disabled = true;
    control.addEventListener('input', () => { dirty = true; text(status, '有尚未提交的修改'); status.classList.remove('is-success'); updateSubmit(); });
    controls.set(descriptor.key, { control, descriptor });
    fields.append(descriptor.type === 'checkbox' ? h('div', { class: 'field' }, h('label', { class: 'checkbox-row', for: id }, control, h('span', {}, h('strong', {}, descriptor.label || descriptor.key, descriptor.required ? ' *' : ''), descriptor.description ? h('small', {}, descriptor.description) : null))) : field(descriptor.label || descriptor.key, control, descriptor.description || (descriptor.type === 'textarea' ? '多行文本原样交给插件处理；参数通常一行一个。' : ''), descriptor.required));
  }
  if (!descriptors.length) fields.append(h('p', { class: 'field-hint' }, '此面板没有可编辑字段。可以执行插件提供的操作。'));
  function updateSubmit() { if (!save.dataset.busy) save.disabled = !loaded || loadingNow || unsupported || !panel.submitAction; if (!reload.dataset.busy) reload.disabled = loadingNow || Boolean(save.dataset.busy); }
  function values() {
    return Object.fromEntries([...controls].map(([key, { control, descriptor }]) => [key, descriptor.type === 'checkbox' ? control.checked : descriptor.type === 'number' ? (control.value === '' ? null : Number(control.value)) : control.value]));
  }
  async function load() {
    if (disposed || loadingNow) return;
    loadingNow = true; updateSubmit(); showError(error, null); text(status, '正在读取配置…');
    for (const { control } of controls.values()) control.disabled = true;
    try {
      const result = await invokePanel(plugin, panel, panel.loadAction, versionId);
      if (disposed) return;
      if (!result?.values || typeof result.values !== 'object') throw new Error('插件未返回有效的表单配置（values）。');
      for (const [key, { control, descriptor }] of controls) {
        const value = own(result.values, key) ? result.values[key] : undefined;
        if (descriptor.type === 'checkbox') control.checked = value === true;
        else control.value = str(value);
      }
      loaded = true; dirty = false; text(status, '已读取插件配置');
      if (unsupported) showError(error, new Error('插件声明了不支持或重复的字段，已禁用提交以防丢失配置。请检查插件。'));
    } catch (failure) { if (!disposed) { showError(error, failure); text(status, '读取失败，可重新读取'); loaded = false; } }
    finally { loadingNow = false; if (!disposed) { for (const { control } of controls.values()) control.disabled = !loaded; updateSubmit(); } }
  }
  form.addEventListener('submit', event => {
    event.preventDefault(); if (!loaded || unsupported || !form.reportValidity()) return;
    const submitted = values();
    perform(save, '正在提交', async () => {
      updateSubmit();
      const result = await invokePanel(plugin, panel, panel.submitAction, versionId, { values: submitted });
      dirty = JSON.stringify(values()) !== JSON.stringify(submitted);
      text(status, result?.jobId ? '操作已提交为后台任务' : dirty ? '已提交；仍有新修改' : '配置已保存'); status.classList.toggle('is-success', !dirty);
      showJobResult(resultBox, result, '插件配置已保存'); await refreshState(true);
    }, { errorTarget: error });
  });
  perform(reload, '读取中', load);
  return { update: updateSubmit, dispose: () => { disposed = true; } };
}

function renderLaunchSettings(root, plugin, panel, versionId) {
  let disposed = false, loaded = false, busy = false, dirty = false, memory = null;
  const controls = new Map();
  const error = inlineError();
  const status = h('span', { class: 'form-status', role: 'status' }, '正在读取配置…');
  function input(key, type = 'number') {
    const descriptor = list(panel.fields).find(item => item.key === key) || {};
    const control = type === 'textarea' ? h('textarea', { rows: '4' }) : h('input', { type, autocomplete: 'off' });
    control.name = key; control.disabled = true;
    if (type === 'number') { control.min = descriptor.min; control.max = descriptor.max; control.step = '1'; control.required = true; }
    control.placeholder = descriptor.placeholder || '';
    control.addEventListener('input', changed);
    controls.set(key, control); return control;
  }
  const java = input('javaPath', 'text');
  const minimum = input('minMemoryMB'), maximum = input('maxMemoryMB');
  const width = input('width'), height = input('height');
  const fullscreen = input('fullscreen', 'checkbox');
  const jvm = input('jvmArgs', 'textarea'), game = input('gameArgs', 'textarea');
  const recommended = h('input', { type: 'checkbox', name: 'useRecommendedMemory', disabled: true, onchange: changed });
  const recommendation = h('small');
  const presets = h('select', { class: 'select', 'aria-label': '内存预设', disabled: true, onchange: () => {
    if (!presets.value) return;
    recommended.checked = false;
    minimum.value = Number(presets.value) >= 4096 ? '1024' : '512'; maximum.value = presets.value;
    changed();
  } }, h('option', { value: '' }, '自定义 / 选择内存预设'), ...[1024, 2048, 4096, 6144, 8192].map(value => h('option', { value }, `${value / 1024} GiB · ${value} MiB`)));
  const memorySummary = h('div', { class: 'memory-summary' });
  const usedBar = h('span', { class: 'memory-used' }), minBar = h('span', { class: 'memory-game-min' }), rangeBar = h('span', { class: 'memory-game-range' });
  const memoryBar = h('div', { class: 'memory-bar', role: 'img', 'aria-label': '内存预算' }, usedBar, minBar, rangeBar);
  const memoryWarning = h('p', { class: 'memory-warning', role: 'status', hidden: true });
  const memoryHint = h('p', { class: 'field-hint' }, '游戏区间表示 JVM 从最小到最大堆内存的预算，并非已经占用。Java 进程还需要额外内存。推荐值会在启动时按可用内存重新计算。');
  const resolution = h('select', { class: 'select', 'aria-label': '窗口尺寸预设', disabled: true, onchange: () => {
    if (!resolution.value) return;
    [width.value, height.value] = resolution.value.split('x'); changed();
  } }, h('option', { value: '' }, '自定义尺寸'), ...[[1280, 720, 'HD'], [1600, 900, 'HD+'], [1920, 1080, 'Full HD'], [2560, 1440, 'QHD'], [1080, 1920, '竖屏']].map(([w, ht, label]) => h('option', { value: `${w}x${ht}` }, `${w} × ${ht} · ${label}`)));
  const dimensions = h('div', { class: 'resolution-inputs' }, field('宽度', width), h('span', { class: 'dimension-times', 'aria-hidden': 'true' }, '×'), field('高度', height), h('span', { class: 'dimension-unit' }, 'px'));
  const save = button(panel.submitLabel || '保存启动配置', 'save', 'btn-primary', null, { type: 'submit', disabled: true });
  const reload = button('重新读取', 'refresh', 'btn-sm', control => {
    if (dirty) confirmation({ title: '丢弃尚未提交的修改？', description: panel.title, explanation: '重新读取将恢复已保存的启动配置。', label: '丢弃并重新读取', action: load, success: '已重新读取配置' });
    else perform(control, '读取中', load, { errorTarget: error });
  });
  const form = h('form', { class: 'surface card launch-form' },
    h('section', { class: 'settings-section' }, cardHeading('Java 环境', '留空时自动选择满足当前版本要求的本机 Java。', 'java'), field('Java 可执行文件', java, '可填写 java.exe / java 的完整路径；不会自动下载 Java。'), button('检测本机 Java', 'search', 'btn-sm', openJava)),
    h('section', { class: 'settings-section' }, cardHeading('游戏内存', '根据物理内存与当前可用量，为系统留出空间。', 'sliders'),
      h('label', { class: 'recommendation-choice' }, recommended, h('span', {}, h('strong', {}, '使用推荐方案'), recommendation), pill('自动计算')),
      h('div', { class: 'memory-preset-row' }, h('span', {}, '常用方案'), presets),
      h('div', { class: 'memory-inputs' }, field('最小内存 · MiB', minimum, '', true), field('最大内存 · MiB', maximum, '', true)),
      h('div', { class: 'memory-visual' }, memorySummary, memoryBar, h('div', { class: 'memory-legend' }, h('span', { class: 'legend-used' }, '系统当前已用'), h('span', { class: 'legend-min' }, '游戏最小值'), h('span', { class: 'legend-range' }, '可增长至最大值'), h('span', { class: 'legend-free' }, '剩余空间')), memoryWarning, memoryHint)),
    h('section', { class: 'settings-section' }, cardHeading('游戏窗口', '宽 × 高，选择常用尺寸或自行输入。', 'grid'), h('div', { class: 'resolution-row' }, dimensions, resolution), h('label', { class: 'checkbox-row' }, fullscreen, h('span', {}, h('strong', {}, '以全屏模式启动'), h('small', {}, '全屏时保留窗口尺寸设置，切回窗口后仍可使用。')))),
    h('section', { class: 'settings-section' }, cardHeading('附加参数', '仅填写你信任的参数。'), field('附加 JVM 参数', jvm, '一行一个完整参数，无需 shell 引号。Java agent 可以执行本机代码。'), field('附加游戏参数', game, '一行一个参数，参数名与值分别占一行。')),
    error, h('div', { class: 'form-footer' }, status, reload, save));
  root.append(form);
  function changed() { dirty = true; text(status, '有尚未保存的修改'); status.classList.remove('is-success'); update(); }
  function values() {
    return { ...Object.fromEntries([...controls].map(([key, control]) => [key, control.type === 'checkbox' ? control.checked : control.type === 'number' ? Number(control.value) : control.value])), useRecommendedMemory: recommended.checked };
  }
  function apply(values) {
    for (const [key, control] of controls) {
      if (control.type === 'checkbox') control.checked = values[key] === true;
      else control.value = str(values[key]);
    }
    recommended.checked = values.useRecommendedMemory === true;
  }
  function update() {
    const currentMemory = state.memory || memory;
    const canEdit = loaded && !busy;
    for (const control of controls.values()) control.disabled = !canEdit;
    recommended.disabled = !canEdit || !currentMemory;
    presets.disabled = !canEdit; resolution.disabled = !canEdit;
    if (recommended.checked && currentMemory) {
      minimum.value = currentMemory.recommendedMinMB; maximum.value = currentMemory.recommendedMaxMB;
    }
    minimum.disabled = maximum.disabled = !canEdit || recommended.checked;
    const size = `${width.value}x${height.value}`;
    resolution.value = [...resolution.options].some(option => option.value === size) ? size : '';
    presets.value = !recommended.checked && [...presets.options].some(option => option.value === maximum.value) ? maximum.value : '';
    text(recommendation, currentMemory ? `${currentMemory.recommendedMinMB}–${currentMemory.recommendedMaxMB} MiB · 根据当前可用内存估算` : '无法读取系统内存，请使用自定义方案');
    if (!save.dataset.busy) save.disabled = !canEdit;
    if (!reload.dataset.busy) reload.disabled = busy;
    memoryBar.hidden = !currentMemory;
    memorySummary.replaceChildren(currentMemory ? h('span', {}, '物理内存 ', h('strong', {}, `${(currentMemory.totalMB / 1024).toFixed(1)} GiB`)) : h('span', {}, '系统物理内存暂不可用'), h('span', {}, `游戏预算 ${formatNumber(minimum.value)}–${formatNumber(maximum.value)} MiB`));
    if (currentMemory) {
      const total = currentMemory.totalMB, used = currentMemory.usedMB, low = number(minimum.value), high = number(maximum.value);
      const percent = value => `${Math.min(100, Math.max(0, value / total * 100))}%`;
      usedBar.style.width = percent(used);
      minBar.style.left = percent(used); minBar.style.width = percent(low);
      rangeBar.style.left = percent(used + low); rangeBar.style.width = percent(Math.max(0, high - low));
      memoryBar.setAttribute('aria-label', `总内存 ${total} MiB，当前已用 ${used} MiB，游戏预算 ${low} 至 ${high} MiB`);
      usedBar.title = `当前已用 ${formatNumber(used)} MiB`;
      minBar.title = `游戏最小值 ${formatNumber(low)} MiB`; rangeBar.title = `可增长至 ${formatNumber(high)} MiB`;
      memoryWarning.hidden = high <= currentMemory.availableMB && low <= high;
      text(memoryWarning, low > high ? '最小内存不能大于最大内存。' : '游戏最大内存超过当前可用量，可能引发系统换页或启动失败。建议降低预算或关闭其他程序。');
      text(memoryHint, `当前已用 ${(used / 1024).toFixed(1)} GiB，可用 ${(currentMemory.availableMB / 1024).toFixed(1)} GiB。游戏区间表示堆内存预算，并非已经占用；Java 进程还需额外内存。推荐值会在启动时重新计算。`);
    }
  }
  async function load() {
    busy = true; update(); showError(error, null);
    try {
      const result = await invokePanel(plugin, panel, panel.loadAction, versionId);
      if (disposed) return;
      if (!result?.values) throw new Error('插件未返回启动配置');
      memory = result.memory; apply(result.values); loaded = true; dirty = false;
      text(status, '已读取启动配置');
      if (result.memoryError) showError(error, result.memoryError);
    } catch (failure) { loaded = false; if (!disposed) { showError(error, failure); text(status, '读取失败，可重新读取'); } throw failure; }
    finally { busy = false; if (!disposed) update(); }
  }
  form.addEventListener('submit', event => {
    event.preventDefault();
    if (!loaded || busy || !form.reportValidity()) return;
    if (number(minimum.value) > number(maximum.value)) { showError(error, '最小内存不能大于最大内存'); return; }
    const submitted = values();
    perform(save, '保存中', async () => {
      busy = true; update();
      try {
        const result = await invokePanel(plugin, panel, panel.submitAction, versionId, {values: submitted});
        if (disposed) return;
        if (!result?.values) throw new Error('插件未返回保存后的配置');
        memory = result.memory; apply(result.values); dirty = false;
        text(status, '启动配置已保存'); status.classList.add('is-success');
        await refreshState(true);
      } finally { busy = false; if (!disposed) update(); }
    }, { errorTarget: error, success: '启动配置已保存' });
  });
  perform(reload, '读取中', load, { errorTarget: error });
  return { update, dispose: () => { disposed = true; } };
}

function renderProfiles(root, plugin, panel) {
  let disposed = false, busy = false, loaded = false;
  const error = inlineError(), cards = h('div', { class: 'profile-grid' });
  const name = h('input', { type: 'text', name: 'playerName', placeholder: '例如 Alex', maxlength: '16', pattern: '[A-Za-z0-9_]{1,16}', required: true, autocomplete: 'off', disabled: true });
  const create = button(panel.submitLabel || '新建玩家档案', 'plus', 'btn-primary', null, { type: 'submit', disabled: true });
  const status = h('p', { class: 'field-hint', role: 'status' });
  const refresh = button('重新读取', 'refresh', 'btn-sm', control => perform(control, '读取中', () => run(panel.loadAction), { errorTarget: error }));
  const form = h('form', { class: 'surface card profile-create' }, cardHeading('新建离线玩家', '创建后自动设为当前档案。', 'user'), field('玩家名称', name, '1–16 位英文字母、数字或下划线。', true), create, status);
  root.append(h('div', { class: 'alert alert-info profile-notice' }, icon('shield'), h('div', {}, h('strong', {}, '当前仅支持本地离线登录'), h('p', {}, '无法通过正版服务器身份验证。切换玩家名称会改变离线 UUID，可能影响存档中的背包、进度和玩家归属；档案管理不会改写这些存档。'))), error, h('div', { class: 'profiles-toolbar' }, h('h2', {}, '已保存的玩家'), refresh), cards, form);
  function show(result) {
    if (!Array.isArray(result?.profiles) || !result.profiles.some(profile => profile.id === result.selectedId)) throw new Error('插件返回的玩家档案不完整');
    preserveFocusReplace(cards, ...result.profiles.map(profile => {
      const selected = profile.id === result.selectedId;
      const select = button(selected ? '正在使用' : '使用此档案', selected ? 'check' : 'user', selected ? 'btn-sm btn-primary' : 'btn-sm', control => perform(control, '切换中', () => run(panel.selectAction, {id: profile.id}), { errorTarget: error, success: '已切换玩家档案' }), {disabled: selected, 'data-focus-key': `profile-${profile.id}`});
      const remove = iconButton(`移除玩家 ${profile.name}`, 'trash', () => confirmation({ title: '移除这个玩家档案？', description: profile.name, explanation: `仅移除启动器中的档案，保留所有游戏存档。${selected ? '移除后将切换到另一个保留的档案。' : ''}`, label: '移除档案', action: () => run(panel.removeAction, {id: profile.id}), success: '玩家档案已移除' }));
      remove.disabled = result.profiles.length <= 1;
      return h('article', { class: `surface profile-card ${selected ? 'is-selected' : ''}` }, h('div', { class: 'profile-card-head' }, h('span', { class: 'profile-avatar' }, icon('user')), h('div', {}, h('h3', {}, profile.name), h('p', {}, '本地离线玩家')), selected ? pill('当前档案', 'running') : null), h('div', { class: 'profile-uuid' }, h('span', {}, '离线 UUID'), h('code', {}, profile.uuid)), h('div', { class: 'profile-card-actions' }, select, remove));
    }));
  }
  function update() {
    name.disabled = !loaded || busy;
    if (!create.dataset.busy) create.disabled = !loaded || busy;
    if (!refresh.dataset.busy) refresh.disabled = busy;
    for (const control of cards.querySelectorAll('button')) control.disabled = busy || control.dataset.fixedDisabled === 'true';
  }
  async function run(action, data) {
    if (busy) throw new Error('请等待当前档案操作完成');
    busy = true; update(); showError(error, null);
    try {
      const result = await invokePanel(plugin, panel, action, '', data);
      if (disposed) return;
      show(result); loaded = true;
      for (const control of cards.querySelectorAll('button')) control.dataset.fixedDisabled = str(control.disabled);
      text(status, `所有版本将使用 ${result.profiles.find(profile => profile.id === result.selectedId).name}。已运行的游戏保留原身份。`);
      await refreshState(true);
    } finally { busy = false; if (!disposed) update(); }
  }
  form.addEventListener('submit', event => {
    event.preventDefault(); if (!loaded || busy || !form.reportValidity()) return;
    const submitted = name.value.trim();
    perform(create, '创建中', async () => { await run(panel.submitAction, {name: submitted}); if (!disposed) name.value = ''; }, { errorTarget: error, success: '玩家档案已创建并选中' });
  });
  perform(refresh, '读取中', () => run(panel.loadAction), { errorTarget: error });
  return { update, dispose: () => { disposed = true; } };
}

function renderPlugins(root) {
  const reload = button('重新扫描插件', 'refresh', 'btn-primary', control => perform(control, '扫描中', async () => { await invoke('plugins.reload'); await refreshState(true); }, { success: '已重新扫描用户插件' }));
  setHeader('插件管理', 'SMALL CORE. OPEN POSSIBILITIES.', '小而专注的核心，按需扩展的能力。', [reload]);
  const directory = h('code'); const directoryCopy = iconButton('复制用户插件目录', 'copy', control => copyText(directory.textContent, control));
  root.append(h('section', { class: 'plugin-directory surface' }, icon('folder'), h('div', {}, h('h2', {}, '你的插件，放在这里'), h('p', {}, '用户插件位于数据目录下的 plugins 文件夹。添加或修改插件后，点击「重新扫描插件」。'), h('div', { class: 'directory-path' }, directory, directoryCopy), h('p', {}, '请只启用可信来源的插件，并留意下方声明的权限。内置插件同样可按需启停。'))));
  const count = h('span', { class: 'count-chip' }); const enabledCount = h('span', { class: 'field-hint' });
  root.append(h('div', { class: 'workspace-toolbar' }, h('div', { class: 'section-caption' }, '已安装插件', count), enabledCount));
  const content = h('div'); root.append(content); let signature = '';
  const toggling = new Set();
  function update() {
    const separator = str(state.dataRoot).includes('\\') ? '\\' : '/';
    text(directory, state.dataRoot ? `${str(state.dataRoot).replace(/[\\/]+$/, '')}${separator}plugins` : '等待数据目录信息'); directoryCopy.disabled = !state.dataRoot;
    text(count, state.plugins.length); text(enabledCount, `${state.plugins.filter(plugin => plugin.enabled).length} 个已启用 · ${state.plugins.filter(plugin => plugin.builtin).length} 个内置`);
    const next = JSON.stringify([state.plugins, [...toggling]]); if (signature === next) return; signature = next;
    if (!state.plugins.length) { content.replaceChildren(emptyState('puzzle', '尚未发现插件', '将可信插件放入上方目录，再重新扫描。这里不会预置虚构的插件记录。')); return; }
    const cards = state.plugins.map(plugin => {
      const busy = toggling.has(plugin.id);
      const toggle = h('input', { type: 'checkbox', class: 'switch', role: 'switch', checked: plugin.enabled, disabled: busy, 'aria-label': `启用 ${plugin.name}`, 'data-focus-key': `toggle-${plugin.id}`, 'aria-busy': busy });
      toggle.addEventListener('change', async () => {
        const restoreFocus = document.activeElement === toggle;
        const enabled = toggle.checked; toggling.add(plugin.id); update();
        try { await invoke('plugins.toggle', { id: plugin.id, enabled }); commit(data => { const current = data.plugins.find(item => item.id === plugin.id); if (current) current.enabled = enabled; }); toast(enabled ? '插件已启用' : '插件已停用', str(plugin.name)); await refreshState(true); }
        catch (error) { toast('插件状态未更改', errorMessage(error), 'error'); }
        finally { toggling.delete(plugin.id); update(); if (restoreFocus && root.isConnected && document.activeElement === document.body) [...content.querySelectorAll('[data-focus-key]')].find(node => node.dataset.focusKey === `toggle-${plugin.id}`)?.focus({ preventScroll: true }); }
      });
      const panels = list(plugin.panels);
      const card = h('article', { class: 'plugin-card surface' }, h('div', { class: 'plugin-card-head' }, h('span', { class: 'plugin-emblem' }, icon(panels.some(panel => panel.kind === 'catalog') ? 'download' : 'puzzle')), h('div', { class: 'plugin-card-title' }, h('h2', {}, plugin.name || plugin.id), h('p', { class: 'mono' }, `${plugin.version ? `v${str(plugin.version).replace(/^v/, '')}` : '版本未提供'} · ${plugin.id}`)), h('label', { class: 'switch-label' }, h('span', {}, busy ? '更新中' : plugin.enabled ? '已启用' : '已停用'), toggle)), h('p', { class: 'plugin-card-description' }, plugin.description || '此插件未提供描述。'));
      if (plugin.error) card.append(h('div', { class: 'inline-error' }, str(plugin.error)));
      if (panels.length) card.append(h('div', { class: 'panel-labels', 'aria-label': '提供的面板' }, ...panels.map(panel => plugin.enabled && !plugin.error && panel.scope === 'global' ? link(`${panel.title} · 全局`, panelHash(plugin.id, panel.id), 'pill') : pill(`${panel.title} · ${panel.scope === 'version' ? '版本内' : '全局'}`))));
      card.append(h('div', { class: 'plugin-permissions' }, h('div', { class: 'info-label' }, '声明的权限'), h('div', { class: 'permission-list' }, list(plugin.permissions).length ? list(plugin.permissions).map(permission => h('span', { class: 'permission-tag' }, permission)) : h('span', { class: 'field-hint' }, '未声明额外权限'))), h('div', { class: 'plugin-card-footer' }, h('span', {}, plugin.builtin ? '内置插件' : '用户插件'), h('span', {}, plugin.author ? `作者 · ${plugin.author}` : '作者未提供')));
      return card;
    });
    preserveFocusReplace(content, h('div', { class: 'plugin-list' }, cards));
  }
  update(); return { update };
}
function renderTasks(root) {
  setHeader('下载与任务', 'EVERY STEP, IN SIGHT.', '真实进度、清晰状态。任务完成后，版本库会自动同步。', [button('刷新状态', 'refresh', '', control => perform(control, '同步中', () => refreshState(true)))]);
  let filter = 'all', signature = '';
  const summary = h('div', { class: 'task-summary' }); const filters = h('div', { class: 'segmented', 'aria-label': '任务筛选' });
  for (const [value, label] of [['all', '全部'], ['active', '进行中'], ['finished', '已结束']]) filters.append(h('button', { type: 'button', 'aria-pressed': value === filter, onclick: event => { filter = value; for (const node of filters.children) node.setAttribute('aria-pressed', String(node === event.currentTarget)); update(); } }, label));
  const content = h('div'); root.append(h('div', { class: 'workspace-toolbar' }, summary, filters), content);
  function update() {
    const next = JSON.stringify([state.jobs, filter]); if (signature === next) return; signature = next;
    summary.replaceChildren(h('span', {}, h('strong', {}, state.jobs.filter(activeJob).length), '进行中'), h('span', {}, h('strong', {}, state.jobs.filter(job => job.status === 'completed').length), '已完成'), h('span', {}, h('strong', {}, state.jobs.filter(job => job.status === 'failed').length), '失败'));
    const jobs = state.jobs.filter(job => filter === 'all' || (filter === 'active' ? activeJob(job) : !activeJob(job))).sort((a, b) => Number(activeJob(b)) - Number(activeJob(a)) || (Date.parse(b.createdAt) || 0) - (Date.parse(a.createdAt) || 0));
    if (!jobs.length) { content.replaceChildren(emptyState('activity', state.jobs.length ? '此筛选下没有任务' : '一切安静，随时出发', state.jobs.length ? '切换筛选查看其他任务。' : '从下载插件提交安装后，任务进度会出现在这里。不会在后台自动开始下载。', link(downloadDestination() === '#plugins' ? '查看插件' : '前往下载面板', downloadDestination(), 'btn', 'download'))); return; }
    preserveFocusReplace(content, h('div', { class: 'task-list' }, jobs.map(job => {
      const total = number(job.total), done = number(job.done), percent = total > 0 ? Math.max(0, Math.min(100, done / total * 100)) : 0;
      const card = h('article', { class: 'task-card surface', 'data-status': job.status }, h('div', { class: 'task-heading' }, icon(job.status === 'completed' ? 'check-circle' : job.status === 'failed' ? 'warning' : 'download'), h('div', {}, h('h2', {}, job.title || '未命名任务'), h('p', {}, job.phase || (job.status === 'queued' ? '等待执行' : statusLabels[job.status] || job.status))), pill(statusLabels[job.status] || job.status, job.status)));
      if (total > 0) {
        const fill = h('div', { class: 'progress-fill' }); fill.style.width = `${percent}%`;
        card.append(h('div', { class: 'task-progress' }, h('div', { class: 'progress-track', role: 'progressbar', 'aria-label': str(job.title), 'aria-valuemin': '0', 'aria-valuemax': str(total), 'aria-valuenow': str(Math.max(0, Math.min(total, done))), 'aria-valuetext': `${formatNumber(done)} / ${formatNumber(total)}` }, fill), h('div', { class: 'task-progress-meta' }, h('span', { class: 'mono' }, `${formatNumber(done)} / ${formatNumber(total)}`), h('span', { class: 'mono' }, `${percent.toFixed(1)}%`))));
      } else if (activeJob(job)) card.append(h('div', { class: 'inline-loading' }, job.status === 'running' ? h('span', { class: 'spinner', 'aria-hidden': 'true' }) : icon('clock'), '等待插件报告可量化进度'));
      if (job.error) card.append(h('div', { class: 'inline-error' }, str(job.error)));
      const footer = h('div', { class: 'task-footer' }, h('span', {}, `创建于 ${dateLabel(job.createdAt, true)}`));
      if (activeJob(job)) footer.append(button('取消任务', 'close', 'btn-quiet btn-sm', () => confirmation({ title: '取消此任务？', description: job.title, explanation: '将请求插件停止当前任务。已写入磁盘的文件是否保留由插件决定。', label: '取消任务', action: () => invoke('jobs.cancel', { id: job.id }), success: '已发送任务取消请求' }), { 'data-focus-key': `cancel-${job.id}` }));
      card.append(footer); return card;
    })));
  }
  update(); return { update };
}
function renderLogs(root, initialId) {
  const id = initialId;
  let disposed = false, busy = false, emptySignature = '', linesText = '', lastResult = null;
  const refresh = button('刷新日志', 'refresh', '', control => perform(control, '读取中', load));
  const folder = button('游戏目录', 'folder-open', '', control => perform(control, '打开中', () => invoke('desktop.openFolder', { id }), { success: '已请求打开游戏目录' }));
  const toolbar = h('div', { class: 'workspace-toolbar' }, h('p', { class: 'field-hint' }, '当前版本的原始输出 · 每 1.5 秒更新'), h('div', { class: 'toolbar-inputs' }, folder, refresh));
  const error = inlineError(); const noVersion = h('div');
  const title = h('span', { class: 'log-title' }); const status = pill('尚未读取');
  const output = h('pre', { class: 'log-output', tabindex: '0', 'aria-label': '游戏日志输出' });
  const copy = button('复制日志', 'copy', 'btn-sm', control => copyText(linesText, control), { disabled: true });
  const follow = h('input', { type: 'checkbox', checked: true, onchange: () => { if (follow.checked) output.scrollTop = output.scrollHeight; } });
  const pause = h('input', { type: 'checkbox' });
  const count = h('span', { role: 'status' }, '0 行'); const path = h('p', { class: 'log-path mono' });
  const workspace = h('section', { class: 'log-workspace' }, h('div', { class: 'log-toolbar' }, icon('terminal'), title, status, copy), output, h('div', { class: 'log-footer' }, count, h('label', { class: 'checkbox-row' }, pause, '暂停自动读取'), h('label', { class: 'checkbox-row' }, follow, '跟随最新输出')));
  root.append(toolbar, error, noVersion, workspace, path);
  function update() {
    const version = findVersion(id); const available = Boolean(version);
    workspace.hidden = !available; path.hidden = !available; noVersion.hidden = available;
    if (!refresh.dataset.busy) refresh.disabled = !available || busy;
    if (!folder.dataset.busy) folder.disabled = !available;
    const nextEmpty = `${Boolean(available)}|${id}|${state.versions.length}`;
    if (!available && nextEmpty !== emptySignature) noVersion.replaceChildren(emptyState('terminal', id ? '此版本记录已不可用' : state.versions.length ? '选择一个版本，查看它的输出' : '还没有可查看的日志', state.versions.length ? '已结束的游戏日志也可以查看，具体内容以启动器保留的日志为准。' : '先导入或安装游戏版本，再启动一次游戏。日志会在这里呈现。', !state.versions.length ? link('返回版本库', '#library', 'btn', 'arrow-left') : null));
    emptySignature = nextEmpty;
    if (available) text(title, version.name || version.id);
  }
  async function load() {
    if (disposed || busy || !findVersion(id)) return;
    busy = true; refresh.disabled = true; output.setAttribute('aria-busy', 'true');
    if (!lastResult) text(output, '正在读取实际游戏日志…');
    try {
      const result = await invoke('logs.get', { id });
      if (disposed) return;
      if (!result || !own(result, 'lines') || (result.lines != null && !Array.isArray(result.lines))) throw new Error('日志响应不完整，未读取到 lines。');
      lastResult = result; showError(error, null);
      const next = list(result.lines).map(str).join('\n');
      const selection = window.getSelection();
      const selecting = selection && !selection.isCollapsed && output.contains(selection.anchorNode);
      if (next !== linesText || !output.dataset.loaded) {
        // Do not destroy text the user is currently selecting for copying.
        if (!selecting) { linesText = next; text(output, linesText || (result.running ? '游戏正在运行，尚未产生日志输出。' : '此版本尚无日志输出。启动后可在这里查看。')); output.dataset.loaded = 'true'; if (follow.checked) output.scrollTop = output.scrollHeight; }
      }
      copy.disabled = !linesText; text(count, `${list(result.lines).length} 行${selecting ? ' · 选择文本时暂缓更新' : ''}`);
      text(status, result.running ? '运行中' : result.exitCode != null ? `已退出 · ${result.exitCode}` : '未运行');
      text(path, result.path ? `日志路径 · ${result.path}` : '日志路径未提供');
    } catch (failure) { if (!disposed) { showError(error, failure); text(status, '读取失败'); if (!lastResult) text(output, '无法读取日志。请查看上方错误并重试。'); } }
    finally { busy = false; output.removeAttribute('aria-busy'); if (!disposed) update(); }
  }
  update(); load();
  return { update, poll: () => { if (!pause.checked) return load(); }, dispose: () => { disposed = true; } };
}

// Startup stays in the user's version library, even on a fresh empty installation.
for (const slot of document.querySelectorAll('[data-icon]')) slot.append(icon(slot.dataset.icon));
$('#identity-button').addEventListener('click', openIdentity);
window.addEventListener('hashchange', () => renderRoute(true));
renderRoute(); refreshState();
setInterval(() => { refreshState(); activeView?.poll?.(); }, 1500);
