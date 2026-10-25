'use strict';

export function invoke(action, input) {
  const saved = host.call('storage.global.get');
  const profiles = Array.isArray(saved.profiles) ? saved.profiles : [{id: 'Player', name: 'Player'}];
  if (!profiles.length || profiles.length > 64) throw new Error('玩家档案数量须为 1–64 个');
  const seen = Object.create(null);
  for (const profile of profiles) {
    const identity = host.call('identity.offline', {name: profile.name});
    const key = identity.name.toLowerCase();
    if (seen[key] || profile.id !== profile.name) throw new Error('玩家档案数据无效或名称重复');
    seen[key] = true;
    profile.uuid = identity.uuid;
  }
  let selectedId = saved.selectedId || profiles[0].id;
  if (!profiles.some(function(profile) { return profile.id === selectedId; })) throw new Error('当前玩家档案不存在');
  if (action === 'identity.configure') {
    const selected = profiles.find(function(profile) { return profile.id === selectedId; });
    return {playerName: selected.name};
  }
  if (action === 'create') {
    const name = String((input.data || {}).name || '').trim();
    const identity = host.call('identity.offline', {name});
    if (profiles.length >= 64) throw new Error('最多保存 64 个玩家档案');
    if (seen[name.toLowerCase()]) throw new Error('已有同名玩家档案');
    profiles.push({id: name, name, uuid: identity.uuid});
    selectedId = name;
  } else if (action === 'select' || action === 'remove') {
    const id = (input.data || {}).id;
    const index = profiles.findIndex(function(profile) { return profile.id === id; });
    if (index < 0) throw new Error('玩家档案不存在');
    if (action === 'select') selectedId = id;
    else {
      if (profiles.length === 1) throw new Error('至少保留一个玩家档案');
      profiles.splice(index, 1);
      if (selectedId === id) selectedId = profiles[0].id;
    }
  } else if (action !== 'load') throw new Error('未知玩家档案动作: ' + action);
  if (action !== 'load') host.call('storage.global.set', {profiles, selectedId});
  return {profiles, selectedId};
}
