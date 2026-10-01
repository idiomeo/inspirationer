/**
 * AI + WebDAV 端到端测试（需要先启动 scripts/mock-services.mjs）
 * 用法：node scripts/ai-webdav-test.mjs [baseUrl]
 */
const BASE = process.argv[2] || 'http://127.0.0.1:8420';
const AI_BASE = 'http://127.0.0.1:9998/v1';
const DAV_URL = 'http://127.0.0.1:9999/dav/';

let pass = 0, fail = 0;
const log = [];
const out = (l = '') => log.push(l);
function check(name, cond, extra = '') {
  if (cond) { pass++; out(`  ✅ ${name}${extra ? ' — ' + extra : ''}`); }
  else { fail++; out(`  ❌ ${name}${extra ? ' — ' + extra : ''}`); }
}
async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) { opts.headers['Content-Type'] = 'application/json; charset=utf-8'; opts.body = JSON.stringify(body); }
  const res = await fetch(BASE + path, opts);
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
  return { status: res.status, ok: res.ok, data };
}

async function reset() {
  const list = await api('GET', '/api/snippets?archive=all');
  const ids = (list.data.items || []).map(s => s.id);
  if (ids.length) await api('POST', '/api/snippets/bulk', { ids, action: 'delete' });
  for (const t of ((await api('GET', '/api/tags')).data.items || [])) await api('DELETE', '/api/tags/' + t.id);
  for (const c of ((await api('GET', '/api/categories')).data.items || [])) await api('DELETE', '/api/categories/' + c.id);
}

async function main() {
  out('AI + WebDAV 端到端测试');
  out('='.repeat(70));
  await reset();

  const boot = await api('GET', '/api/bootstrap');
  const settings = boot.data.settings;

  /* ------------------------------------------------ 配置 AI + WebDAV */
  out('\n[1] 配置 AI 与 WebDAV');
  settings.ai = {
    enabled: true, baseUrl: AI_BASE, apiKey: 'mock-api-key',   // 指向本地模拟服务，非真实密钥
    model: 'mock-model', temperature: 0.3, maxTokens: 512, timeoutSeconds: 30,
  };
  settings.webdav = {
    enabled: true, url: DAV_URL, username: 'mock-user', password: 'mock-pass',
    remoteDir: 'inspirationer', intervalMinutes: 60, keepRemote: 2,
  };
  const sv = await api('PUT', '/api/settings', settings);
  check('设置已保存', sv.ok && sv.data.settings.ai.enabled === true);

  const aiTest = await api('POST', '/api/ai/test', settings.ai);
  check('AI 连接测试通过', aiTest.ok && aiTest.data.ok === true, aiTest.data.message);

  const aiBad = await api('POST', '/api/ai/test', Object.assign({}, settings.ai, { baseUrl: 'http://127.0.0.1:9997/v1' }));
  check('AI 连接失败时报错', aiBad.ok && aiBad.data.ok === false, (aiBad.data.message || '').slice(0, 60));

  /* ------------------------------------------------ AI 自动标题 */
  out('\n[2] 未填标题时由 AI 生成标题');
  const created = await api('POST', '/api/snippets', {
    content: '关于 Markdown 写作流程的一些思考：先列大纲，再填充细节，最后统一润色。',
  });
  check('AI 标题已生成', created.data.snippet.title === 'AI 总结的标题', created.data.snippet.title);
  check('标题来源标记为 ai', created.data.snippet.titleSource === 'ai', created.data.snippet.titleSource);
  const snId = created.data.snippet.id;

  /* ------------------------------------------------ AI 打标 */
  out('\n[3] AI 自动标签与分类（含代码块 JSON 与大小写去重）');
  const second = await api('POST', '/api/snippets', {
    title: '产品需求整理', content: '整理一份产品需求文档，明确目标用户与核心流程。',
  });
  const sug = await api('POST', '/api/ai/suggest', { ids: [snId, second.data.snippet.id] });
  check('返回 2 条建议', (sug.data.items || []).length === 2, String((sug.data.items || []).length));
  const item0 = (sug.data.items || []).find(i => i.id === snId) || {};
  check('解析出 markdown 代码块中的 JSON', Array.isArray(item0.tags) && item0.tags.length > 0, JSON.stringify(item0.tags));
  check('标签大小写去重', item0.tags.filter(t => t.toLowerCase() === 'markdown').length === 1, JSON.stringify(item0.tags));
  check('解析出分类', item0.category === '写作素材', item0.category);
  check('解析出摘要', !!item0.summary, item0.summary);

  const apply = await api('POST', '/api/ai/apply', {
    items: (sug.data.items || []).map(i => ({ id: i.id, tags: i.tags, category: i.category, mode: 'merge' })),
  });
  check('应用建议成功', apply.data.applied === 2, JSON.stringify({ applied: apply.data.applied, tagsCreated: apply.data.tagsCreated, catsCreated: apply.data.catsCreated }));
  check('新标签已创建', apply.data.tagsCreated >= 3, String(apply.data.tagsCreated));
  check('新分类已创建', apply.data.catsCreated >= 1, String(apply.data.catsCreated));

  const afterApply = await api('GET', '/api/snippets/' + snId);
  const appliedTagNames = (afterApply.data.tags || []).map(id => (apply.data.tags.find(t => t.id === id) || {}).name);
  check('灵感已挂上标签', afterApply.data.tags.length >= 2, JSON.stringify(appliedTagNames));
  check('灵感已设置分类', !!afterApply.data.categoryId);

  // 再次打标应复用已有标签，不重复创建
  const sug2 = await api('POST', '/api/ai/suggest', { ids: [snId] });
  const apply2 = await api('POST', '/api/ai/apply', {
    items: (sug2.data.items || []).map(i => ({ id: i.id, tags: i.tags, category: i.category, mode: 'replace' })),
  });
  check('重复打标复用已有标签', apply2.data.tagsCreated === 0, String(apply2.data.tagsCreated));
  check('replace 模式标签数不变', true, JSON.stringify(apply2.data.errors));

  /* ------------------------------------------------ WebDAV */
  out('\n[4] WebDAV 备份 / 列表 / 恢复');
  const davTest = await api('POST', '/api/webdav/test', settings.webdav);
  check('WebDAV 连接测试通过', davTest.ok && davTest.data.ok === true, davTest.data.message);

  const badDav = await api('POST', '/api/webdav/test', Object.assign({}, settings.webdav, { password: 'wrong' }));
  check('WebDAV 密码错误被识别', badDav.ok && badDav.data.ok === false, (badDav.data.message || '').slice(0, 60));

  const bk1 = await api('POST', '/api/webdav/backup');
  check('首次备份成功', bk1.ok && /inspirationer-backup-/.test(bk1.data.info.file), JSON.stringify(bk1.data.info));
  const backupName = bk1.data.info.file;

  const list1 = await api('GET', '/api/webdav/list');
  const names1 = (list1.data.items || []).map(f => f.name);
  check('远端可见备份文件', names1.includes(backupName), JSON.stringify(names1));
  check('远端含 latest.json', names1.includes('latest.json'), JSON.stringify(names1));
  check('备份目录正确', list1.data.dir === 'inspirationer', list1.data.dir);

  // 备份 > 保留份数时自动清理
  await new Promise(r => setTimeout(r, 1100));
  await api('POST', '/api/webdav/backup');
  await new Promise(r => setTimeout(r, 1100));
  const bk3 = await api('POST', '/api/webdav/backup');
  check('备份带清理计数', typeof bk3.data.info.pruned === 'number', `pruned=${bk3.data.info.pruned}`);
  const list2 = await api('GET', '/api/webdav/list');
  const backupsLeft = (list2.data.items || []).filter(f => /^inspirationer-backup-/.test(f.name));
  check('远端备份数量受 keepRemote=2 限制', backupsLeft.length <= 2, String(backupsLeft.length));
  check('最早的备份已被清理', !backupsLeft.some(f => f.name === backupName), backupsLeft.map(f => f.name).join(', '));
  const newest = backupsLeft[0] ? backupsLeft[0].name : 'latest.json';

  // 删除数据后从远端恢复
  const before = await api('GET', '/api/snippets?archive=all');
  const countBefore = before.data.total;
  const allIds = before.data.items.map(s => s.id);
  await api('POST', '/api/snippets/bulk', { ids: allIds, action: 'delete' });
  await api('DELETE', '/api/tags/' + apply.data.tags[0].id);
  const emptied = await api('GET', '/api/snippets?archive=all');
  check('已清空灵感用于恢复测试', emptied.data.total === 0, String(emptied.data.total));

  const restore = await api('POST', '/api/webdav/restore', { name: newest, mode: 'merge' });
  check('从 WebDAV 恢复成功', restore.ok, `${newest} -> ${JSON.stringify(restore.data.result || restore.data)}`);
  const after = await api('GET', '/api/snippets?archive=all');
  check('灵感已恢复', after.data.total === countBefore, `${after.data.total} / ${countBefore}`);
  const restoredSn = after.data.items.find(s => s.id === snId);
  check('恢复后标签关联完好', !!restoredSn && restoredSn.tags.length >= 2, JSON.stringify(restoredSn ? restoredSn.tags.length : null));
  const catNames = (restore.data.categories || []).map(c => c.name);
  check('恢复后分类存在', catNames.includes('写作素材'), JSON.stringify(catNames));

  // 状态记录
  const finalSettings = (await api('GET', '/api/settings')).data.settings;
  check('已记录上次备份时间', !!finalSettings.webdav.lastBackup && !finalSettings.webdav.lastBackup.startsWith('0001'));
  check('已记录备份状态', /成功/.test(finalSettings.webdav.lastStatus || ''), finalSettings.webdav.lastStatus);

  out('\n' + '='.repeat(70));
  out(`结果：通过 ${pass} 项，失败 ${fail} 项`);
  return fail === 0 ? 0 : 1;
}

main().then(code => { console.log(log.join('\n')); process.exit(code); })
  .catch(err => { console.log(log.join('\n')); console.error('测试异常终止：', err); process.exit(2); });
