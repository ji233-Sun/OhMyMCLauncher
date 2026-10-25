'use strict';
export function invoke(action, input) {
  if (!input.versionId) throw new Error('请先选择一个版本');
  const defaults = host.call('launch.defaults');
  const stored = host.call('storage.get');
  const recommended = typeof stored.useRecommendedMemory === 'boolean' ? stored.useRecommendedMemory : !Object.prototype.hasOwnProperty.call(stored, 'maxMemoryMB');
  let memory = null;
  let memoryError = '';
  try { memory = host.call('system.memory'); } catch (error) { memoryError = String(error.message || error); }
  function settings(source) {
    const values = Object.assign({}, defaults, {useRecommendedMemory: recommended}, source);
    // Player identity belongs to the global profile provider, never this storage.
    delete values.playerName;
    if (typeof values.useRecommendedMemory !== 'boolean') throw new Error('推荐内存选项必须为布尔值');
    if (values.useRecommendedMemory) {
      if (!memory) throw new Error(memoryError || '无法读取系统内存，请改用自定义方案');
      values.minMemoryMB = memory.recommendedMinMB;
      values.maxMemoryMB = memory.recommendedMaxMB;
    }
    host.call('launch.validate', Object.assign({}, defaults, values));
    return values;
  }
  if (action === 'load') {
    // A failed system probe still leaves the form available for custom settings.
    const source = Object.assign({}, stored);
    if (!memory) source.useRecommendedMemory = false;
    return {values: settings(source), memory, memoryError};
  }
  if (action === 'save') {
    const values = settings((input.data || {}).values || {});
    host.call('storage.set', values);
    return {values, memory, memoryError};
  }
  if (action === 'launch.configure') {
    return Object.assign({}, input.options || defaults, settings(stored));
  }
  throw new Error('未知启动参数动作: ' + action);
}
