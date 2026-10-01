/**
 * 灵感管理器 API 冒烟测试（Node 18+，使用内置 fetch）
 * 用法：node scripts/api-smoke.mjs [baseUrl]
 */
const BASE = process.argv[2] || 'http://127.0.0.1:8420';
let pass = 0, fail = 0;
const log = [];
function out(line = '') { log.push(line); }
function check(name, cond, extra = '') {
  if (cond) { pass++; out(`  ✅ ${name}${extra ? ' — ' + extra : ''}`); }
  else { fail++; out(`  ❌ ${name}${extra ? ' — ' + extra : ''}`); }
}

async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json; charset=utf-8';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(BASE + path, opts);
  const text = await res.text();
  let data = null;
  try { data = text ? JSON.parse(text) : null; } catch { data = { raw: text }; }
  return { status: res.status, ok: res.ok, data };
}

/* ---------------------------------------------------------------- 清空环境 */
async function reset() {
  const list = await api('GET', '/api/snippets?archive=all');
  const ids = (list.data.items || []).map(s => s.id);
  if (ids.length) await api('POST', '/api/snippets/bulk', { ids, action: 'delete' });
  const tags = (await api('GET', '/api/tags')).data.items || [];
  for (const t of tags) await api('DELETE', '/api/tags/' + t.id);
  const cats = (await api('GET', '/api/categories')).data.items || [];
  for (const c of cats) await api('DELETE', '/api/categories/' + c.id);
  // 设置恢复默认
  const boot = await api('GET', '/api/bootstrap');
  const s = boot.data.settings;
  s.ui.titleMaxRunes = 10;
  s.shortcuts = { newSnippet: 'Alt+N', saveSnippet: 'Alt+S', focusSearch: 'Alt+K', selectAll: 'Alt+A', pipelineNext: 'Alt+J', togglePreview: 'Alt+P', openSettings: 'Alt+O', closeModal: 'Escape' };
  s.ai.enabled = false;
  s.ai.apiKey = '';
  s.webdav.enabled = false;
  s.webdav.url = '';
  s.webdav.username = '';
  s.webdav.password = '';
  s.ui.confirmDelete = true;
  s.ui.cardPreview = true;
  s.ui.theme = 'dark';
  await api('PUT', '/api/settings', s);
}

/* ---------------------------------------------------------------- 主流程 */
async function main() {
  out('灵感管理器 API 冒烟测试 @ ' + BASE);
  out('='.repeat(70));
  await reset();

  // 1. 无标题创建（未配置 AI）
  out('\n[1] 未填标题 + 未配置 AI → 截取正文前 10 字');
  const content1 = '今天想到一个很有潜力的点子：用 Go 写本地灵感管理器，支持 Markdown 与 AI 打标。';
  const c1 = await api('POST', '/api/snippets', { content: content1 });
  check('创建成功', c1.ok, `HTTP ${c1.status}`);
  check('标题来源为 truncate', c1.data.snippet.titleSource === 'truncate');
  check('标题 = 正文前 10 字', c1.data.snippet.title === Array.from(content1).slice(0, 10).join(''),
    `"${c1.data.snippet.title}" vs "${Array.from(content1).slice(0, 10).join('')}"`);
  check('返回提示信息', Array.isArray(c1.data.warnings) && c1.data.warnings.length > 0, c1.data.warnings[0]);
  const sn1 = c1.data.snippet;

  // 2. 手写标题
  out('\n[2] 手写标题');
  const c2 = await api('POST', '/api/snippets', {
    title: '手动标题：设计评审流程',
    content: '# 设计评审\n\n- 拉上产品\n- 提前一天发文档\n\n> 目标是 30 分钟内结束\n\n```js\nconsole.log("hi")\n```',
  });
  check('标题保持原样', c2.data.snippet.title === '手动标题：设计评审流程');
  check('标题来源 user', c2.data.snippet.titleSource === 'user');
  const sn2 = c2.data.snippet;

  // 3. 标签 / 分类与颜色
  out('\n[3] 标签与分类（含自定义颜色）');
  const t1 = await api('POST', '/api/tags', { name: '产品设计' });
  const t2 = await api('POST', '/api/tags', { name: '技术方案', color: '#22c55e' });
  const t3 = await api('POST', '/api/tags', { name: '灵感收集' });
  const cat1 = await api('POST', '/api/categories', { name: '工作' });
  check('标签 1 创建', t1.data.tag.name === '产品设计');
  check('标签 2 使用自定义颜色', t2.data.tag.color === '#22c55e', t2.data.tag.color);
  check('未指定颜色的标签自动配色', !!t1.data.tag.color && !!t3.data.tag.color, `${t1.data.tag.color} / ${t3.data.tag.color}`);
  check('自动配色互不相同', t1.data.tag.color !== t3.data.tag.color, `${t1.data.tag.color} / ${t3.data.tag.color}`);
  check('分类创建', cat1.data.category.name === '工作');

  // 4. 附加到灵感
  out('\n[4] 给灵感附加 2 个标签 + 1 个分类');
  const p1 = await api('PATCH', '/api/snippets/' + sn1.id, {
    tags: [t1.data.tag.id, t2.data.tag.id],
    categoryId: cat1.data.category.id,
  });
  check('标签数量为 2', p1.data.snippet.tags.length === 2, JSON.stringify(p1.data.snippet.tags));
  check('分类已设置', p1.data.snippet.categoryId === cat1.data.category.id);

  // 5. 列表与统计
  out('\n[5] 再建 2 条 + 统计');
  const c3 = await api('POST', '/api/snippets', { title: '读书笔记：深度工作', content: '关于专注力的笔记，提到 Markdown 与灵感管理', tags: [t1.data.tag.id] });
  const c4 = await api('POST', '/api/snippets', { title: '买菜清单', content: '西红柿、鸡蛋、面条' });
  check('灵感总数 = 4', c4.data.stats.snippets === 4, String(c4.data.stats.snippets));
  check('标签计数生效', c4.data.stats.byTag[t1.data.tag.id] === 2, JSON.stringify(c4.data.stats.byTag));
  check('分类计数生效', c4.data.stats.byCategory[cat1.data.category.id] === 1);
  check('未分类计数 = 3', c4.data.stats.uncategorized === 3);

  // 6. 搜索
  out('\n[6] 搜索：全文 vs 仅标题 vs 仅正文');
  const q1 = await api('GET', '/api/snippets?query=' + encodeURIComponent('Markdown') + '&mode=all');
  check('全文 Markdown = 2 条', q1.data.total === 2, String(q1.data.total));
  const q2 = await api('GET', '/api/snippets?query=' + encodeURIComponent('Markdown') + '&mode=title');
  check('仅标题 Markdown = 0 条', q2.data.total === 0, String(q2.data.total));
  const q3 = await api('GET', '/api/snippets?query=' + encodeURIComponent('Markdown') + '&mode=content');
  check('仅正文 Markdown = 2 条', q3.data.total === 2, String(q3.data.total));
  const q4 = await api('GET', '/api/snippets?query=' + encodeURIComponent('灵感') + '&mode=all');
  check('中文全文搜索「灵感」= 2 条', q4.data.total === 2, String(q4.data.total));
  const q5 = await api('GET', '/api/snippets?query=' + encodeURIComponent('灵感') + '&mode=all&categoryId=' + cat1.data.category.id);
  check('分类内搜索 = 1 条', q5.data.total === 1, String(q5.data.total));
  const q6 = await api('GET', '/api/snippets?query=' + encodeURIComponent('产品设计') + '&mode=all');
  check('按标签名搜索命中 2 条', q6.data.total === 2, String(q6.data.total));
  const q7 = await api('GET', '/api/snippets?tagIds=' + t1.data.tag.id);
  check('标签过滤 = 2 条', q7.data.total === 2, String(q7.data.total));
  const q8 = await api('GET', '/api/snippets?categoryId=__none__');
  check('未分类视图 = 3 条', q8.data.total === 3, String(q8.data.total));

  // 7. 批量操作
  out('\n[7] 批量：加标签 / 改分类 / 归档');
  const b1 = await api('POST', '/api/snippets/bulk', { ids: [sn1.id, sn2.id], action: 'assign', addTags: ['批处理测试'] });
  check('批量加标签影响 2 条', b1.data.affected === 2, JSON.stringify(b1.data));
  const bulkTag = (b1.data.tags || []).find(t => t.name === '批处理测试');
  check('新标签已自动创建', !!bulkTag);
  const b2 = await api('POST', '/api/snippets/bulk', { ids: [c4.data.snippet.id], action: 'archive' });
  check('批量归档成功', b2.data.affected === 1);
  const arch = await api('GET', '/api/snippets?archive=only');
  check('归档视图 = 1 条', arch.data.total === 1, String(arch.data.total));
  const b3 = await api('POST', '/api/snippets/bulk', { ids: [c4.data.snippet.id], action: 'unarchive' });
  check('取消归档成功', b3.data.affected === 1);

  // 8. 标签改名 / 改色 / 删除
  out('\n[8] 标签改名改色与删除');
  const u1 = await api('PUT', '/api/tags/' + t2.data.tag.id, { name: '技术方案v2', color: '#8b5cf6' });
  check('改名改色成功', u1.data.tag.name === '技术方案v2' && u1.data.tag.color === '#8b5cf6');
  const delTag = await api('DELETE', '/api/tags/' + bulkTag.id);
  check('删除标签成功', delTag.ok);
  const sn1After = await api('GET', '/api/snippets/' + sn1.id);
  check('灵感上的该标签已被摘除', !sn1After.data.tags.includes(bulkTag.id));

  // 9. 分类删除
  out('\n[9] 删除分类后灵感变为未分类');
  const delCat = await api('DELETE', '/api/categories/' + cat1.data.category.id);
  check('删除分类成功', delCat.ok);
  const sn1After2 = await api('GET', '/api/snippets/' + sn1.id);
  check('灵感分类已置空', sn1After2.data.categoryId === '');

  // 10. 设置：自定义标题长度 + 快捷键
  out('\n[10] 设置：标题截取字数改为 5、自定义快捷键');
  const boot = await api('GET', '/api/bootstrap');
  const settings = boot.data.settings;
  settings.ui.titleMaxRunes = 5;
  settings.shortcuts.newSnippet = 'Alt+Shift+N';
  const sv = await api('PUT', '/api/settings', settings);
  check('设置保存成功', sv.ok);
  check('快捷键已持久化', sv.data.settings.shortcuts.newSnippet === 'Alt+Shift+N');
  const c5 = await api('POST', '/api/snippets', { content: '一二三四五六七八九十' });
  check('标题按新设置截取 5 字', c5.data.snippet.title === '一二三四五', c5.data.snippet.title);
  // 复原为默认，保证测试可重复执行
  const restore = await api('GET', '/api/bootstrap');
  const rs = restore.data.settings;
  rs.ui.titleMaxRunes = 10;
  rs.shortcuts.newSnippet = 'Alt+N';
  const rv = await api('PUT', '/api/settings', rs);
  check('设置已复原为默认', rv.ok && rv.data.settings.shortcuts.newSnippet === 'Alt+N' && rv.data.settings.ui.titleMaxRunes === 10);
  await api('DELETE', '/api/snippets/' + c5.data.snippet.id);

  // 11. 备份导出 / 导入
  out('\n[11] 备份导出与导入');
  const exp = await fetch(BASE + '/api/backup/export');
  const expText = await exp.text();
  check('导出接口 200', exp.status === 200);
  check('导出内容含 Disposition', (exp.headers.get('content-disposition') || '').includes('inspirationer-'));
  let backup = null;
  try { backup = JSON.parse(expText); } catch { }
  check('导出为合法 JSON 且含灵感', !!backup && Array.isArray(backup.snippets) && backup.snippets.length === 4, backup ? String(backup.snippets.length) : 'parse fail');
  check('导出包含中文内容', expText.includes('产品设计'));
  const impMerge = await api('POST', '/api/backup/import', { mode: 'merge', data: backup });
  check('合并导入成功', impMerge.ok, JSON.stringify(impMerge.data.result));
  const snap = await api('POST', '/api/backup/local');
  check('本地快照创建成功', snap.ok && /snapshot-/.test(snap.data.path || ''), snap.data.path);
  const impReplace = await api('POST', '/api/backup/import', { mode: 'replace', data: backup });
  check('覆盖导入成功', impReplace.ok, JSON.stringify(impReplace.data.result));
  const afterReplace = await api('GET', '/api/snippets?archive=all');
  check('覆盖后灵感数量 = 4', afterReplace.data.total === 4, String(afterReplace.data.total));

  // 12. AI 未配置时的错误处理
  out('\n[12] AI 未启用时的降级与报错');
  const aiTitle = await api('POST', '/api/ai/title', { content: '随便写点内容' });
  check('AI 生成标题返回明确错误', !aiTitle.ok && /未启用/.test(aiTitle.data.error || ''), `${aiTitle.status} ${aiTitle.data.error}`);
  const aiSug = await api('POST', '/api/ai/suggest', { ids: [sn1.id] });
  check('AI 打标返回明确错误', !aiSug.ok && /未启用/.test(aiSug.data.error || ''), `${aiSug.status} ${aiSug.data.error}`);

  // 13. WebDAV 未配置时的错误处理
  out('\n[13] WebDAV 未配置时的报错');
  const davBackup = await api('POST', '/api/webdav/backup');
  check('备份返回明确错误', !davBackup.ok && /WebDAV/.test(davBackup.data.error || ''), `${davBackup.status} ${davBackup.data.error}`);
  const davList = await api('GET', '/api/webdav/list');
  check('列目录返回明确错误', !davList.ok, `${davList.status} ${davList.data.error}`);
  const davTest = await api('POST', '/api/webdav/test', { url: '', username: '', password: '', remoteDir: 'x' });
  check('测试连接返回未配置', davTest.ok && davTest.data.ok === false, JSON.stringify(davTest.data));

  // 14. 单条更新与删除
  out('\n[14] 更新与删除');
  const upd = await api('PUT', '/api/snippets/' + sn2.id, { title: '改过的标题', content: '改过的正文', tags: [], categoryId: '' });
  check('更新成功', upd.data.snippet.title === '改过的标题');
  check('标题来源仍记录', !!upd.data.snippet.titleSource, upd.data.snippet.titleSource);
  const del = await api('DELETE', '/api/snippets/' + sn2.id);
  check('删除成功', del.ok);
  const gone = await api('GET', '/api/snippets/' + sn2.id);
  check('删除后 404', gone.status === 404, String(gone.status));

  out('\n' + '='.repeat(70));
  out(`结果：通过 ${pass} 项，失败 ${fail} 项`);
  return fail === 0 ? 0 : 1;
}

main().then(code => {
  console.log(log.join('\n'));
  process.exit(code);
}).catch(err => {
  console.log(log.join('\n'));
  console.error('测试异常终止：', err);
  process.exit(2);
});
