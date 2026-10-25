'use strict';
export function invoke(action, input) {
  if (!input.versionId) throw new Error('请先选择一个版本');
  const defaults = host.call('launch.defaults');
  if (action === 'load') return {values: Object.assign({}, defaults, host.call('storage.get'))};
  if (action === 'save') {
    const values = Object.assign({}, defaults, (input.data || {}).values || {});
    // Host validates again; a different plugin cannot bypass the core's invariants.
    host.call('launch.validate', values);
    host.call('storage.set', values);
    return {values};
  }
  if (action === 'launch.configure') {
    const values = Object.assign({}, input.options || defaults, host.call('storage.get'));
    host.call('launch.validate', values);
    return values;
  }
  throw new Error('未知启动参数动作: ' + action);
}
