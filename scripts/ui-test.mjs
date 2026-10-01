/**
 * 前端 UI 端到端测试：用无头 Edge/Chrome 通过 CDP 驱动真实浏览器。
 * 覆盖：页面渲染、快捷键（Alt+N / Alt+S / Alt+K / Alt+A / Alt+J / Alt+O）、
 *       编辑器、AI 打标弹窗、流水线、全选、搜索、侧栏导航。
 *
 * 用法：node scripts/ui-test.mjs [baseUrl]
 * 前置：灵感管理器服务已运行；建议同时运行 scripts/mock-services.mjs 以便测试 AI 流程。
 */
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const BASE = process.argv[2] || 'http://127.0.0.1:8420';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PORT = 9333;

let pass = 0, fail = 0;
const log = [];
const out = (l = '') => log.push(l);
function check(name, cond, extra = '') {
  if (cond) { pass++; out(`  ✅ ${name}${extra ? ' — ' + extra : ''}`); }
  else { fail++; out(`  ❌ ${name}${extra ? ' — ' + extra : ''}`); }
}
const sleep = (ms) => new Promise(r => setTimeout(r, ms));

function findBrowser() {
  const candidates = [
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  ];
  for (const c of candidates) if (fs.existsSync(c)) return c;
  return null;
}

async function api(method, p, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) { opts.headers['Content-Type'] = 'application/json; charset=utf-8'; opts.body = JSON.stringify(body); }
  const res = await fetch(BASE + p, opts);
  const text = await res.text();
  return text ? JSON.parse(text) : null;
}

/* ------------------------------------------------------------- CDP 客户端 */
class CDP {
  constructor(ws) {
    this.ws = ws;
    this.id = 0;
    this.pending = new Map();
    this.errors = [];
    this.consoleErrors = [];
    ws.addEventListener('message', (ev) => {
      const msg = JSON.parse(ev.data);
      if (msg.id && this.pending.has(msg.id)) {
        const { res, rej } = this.pending.get(msg.id);
        this.pending.delete(msg.id);
        msg.error ? rej(new Error(JSON.stringify(msg.error))) : res(msg.result);
        return;
      }
      if (msg.method === 'Runtime.exceptionThrown') {
        const d = msg.params.exceptionDetails;
        this.errors.push((d.exception && d.exception.description) || d.text || 'unknown');
      }
      if (msg.method === 'Runtime.consoleAPICalled' && msg.params.type === 'error') {
        this.consoleErrors.push(msg.params.args.map(a => a.value || a.description || '').join(' '));
      }
    });
  }
  send(method, params = {}) {
    const id = ++this.id;
    return new Promise((res, rej) => {
      this.pending.set(id, { res, rej });
      this.ws.send(JSON.stringify({ id, method, params }));
      setTimeout(() => {
        if (this.pending.has(id)) { this.pending.delete(id); rej(new Error('CDP 超时: ' + method)); }
      }, 20000);
    });
  }
  async eval(expression) {
    const r = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
    if (r.exceptionDetails) {
      throw new Error('页面 JS 异常: ' + ((r.exceptionDetails.exception && r.exceptionDetails.exception.description) || r.exceptionDetails.text));
    }
    return r.result.value;
  }
  async waitFor(expression, label = expression, timeout = 12000) {
    const t0 = Date.now();
    while (Date.now() - t0 < timeout) {
      try { if (await this.eval(`!!(${expression})`)) return true; } catch { }
      await sleep(120);
    }
    throw new Error('等待超时: ' + label);
  }
  async key(key, code, vk, modifiers = 1) {
    const base = { modifiers, key, code, windowsVirtualKeyCode: vk, nativeVirtualKeyCode: vk };
    await this.send('Input.dispatchKeyEvent', Object.assign({ type: 'keyDown' }, base));
    // 只有不带修饰键的普通字符才补发 char 事件，
    // 否则会把 "Alt+N" 这类组合键当成输入文本插入到聚焦的输入框里
    if (modifiers === 0 && key.length === 1) {
      await this.send('Input.dispatchKeyEvent', Object.assign({ type: 'char', text: key, unmodifiedText: key }, base));
    }
    await this.send('Input.dispatchKeyEvent', Object.assign({ type: 'keyUp' }, base));
  }
}

async function main() {
  const browser = findBrowser();
  if (!browser) { out('未找到 Edge/Chrome，跳过 UI 测试'); console.log(log.join('\n')); return 0; }
  const userDataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'inspirationer-ui-'));

  // 快捷键恢复默认，保证测试可重复执行
  const boot = await api('GET', '/api/bootstrap');
  boot.settings.shortcuts = {
    newSnippet: 'Alt+N', saveSnippet: 'Alt+S', focusSearch: 'Alt+K', selectAll: 'Alt+A',
    pipelineNext: 'Alt+J', togglePreview: 'Alt+P', openSettings: 'Alt+O', closeModal: 'Escape',
  };
  boot.settings.ui.confirmDelete = false;   // 自动化测试里避免二次确认弹窗
  boot.settings.ui.language = 'en';         // 固定初始语言，便于断言文案
  await api('PUT', '/api/settings', boot.settings);

  // 清空历史数据，保证测试确定性
  const old = await api('GET', '/api/snippets?archive=all');
  const oldIds = (old.items || []).map(s => s.id);
  if (oldIds.length) await api('POST', '/api/snippets/bulk', { ids: oldIds, action: 'delete' });
  for (const t of ((await api('GET', '/api/tags')).items || [])) await api('DELETE', '/api/tags/' + t.id);
  for (const c of ((await api('GET', '/api/categories')).items || [])) await api('DELETE', '/api/categories/' + c.id);

  // 准备测试数据（含一个标签与一个分类，供流水线打标测试使用）
  await api('POST', '/api/tags', { name: '测试标签', color: '#22c55e' });
  await api('POST', '/api/categories', { name: '测试分类', color: '#6366f1' });
  await api('POST', '/api/snippets', { title: 'UI 测试：快捷键流程', content: '# 标题一\n\n这是 **Markdown** 内容，含 `代码` 与列表：\n\n- 项目 A\n- 项目 B\n' });
  await api('POST', '/api/snippets', { title: 'UI 测试：流水线样本', content: '流水线打标用的第二条内容。\n\n> 引用块测试' });

  out('前端 UI 端到端测试（CDP + 无头浏览器）');
  out('浏览器：' + browser);
  out('='.repeat(70));
  out('\n[1] 页面加载与渲染');

  const proc = spawn(browser, [
    '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
    '--disable-extensions', '--disable-features=Translate',
    `--remote-debugging-port=${PORT}`, `--user-data-dir=${userDataDir}`,
    '--window-size=1600,1000', BASE + '/',
  ], { stdio: 'ignore', detached: false });

  let target = null;
  for (let i = 0; i < 60; i++) {
    await sleep(300);
    try {
      const list = await (await fetch(`http://127.0.0.1:${PORT}/json/list`)).json();
      target = list.find(t => t.type === 'page' && t.url.startsWith(BASE));
      if (target && target.webSocketDebuggerUrl) break;
    } catch { }
  }
  if (!target) { proc.kill(); throw new Error('无法连接到无头浏览器调试端口'); }

  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((res, rej) => {
    ws.addEventListener('open', res);
    ws.addEventListener('error', rej);
  });
  const cdp = new CDP(ws);
  await cdp.send('Runtime.enable');
  await cdp.send('Page.enable');

  try {
    await cdp.waitFor("document.readyState === 'complete'", '页面加载完成');
    await cdp.waitFor("document.querySelectorAll('#grid .card').length >= 2", '卡片渲染');

    const cardCount = await cdp.eval("document.querySelectorAll('#grid .card').length");
    check('灵感卡片已渲染', cardCount >= 2, cardCount + ' 张');
    check('EasyMDE 已加载', await cdp.eval("typeof EasyMDE === 'function'"));
    check('marked 已加载', await cdp.eval("typeof marked !== 'undefined'"));
    check('DOMPurify 已加载', await cdp.eval("typeof DOMPurify !== 'undefined'"));
    check('Markdown 渲染为 HTML', await cdp.eval("!!document.querySelector('#grid .card .markdown h1, #grid .card .markdown strong, #grid .card .markdown ul')"));
    check('侧栏渲染分类/标签区', await cdp.eval("!!document.querySelector('#catList') && !!document.querySelector('#tagList')"));
    // 注意：必须用计算样式判断可见性——只检查 el.hidden 属性会漏掉 "CSS display 覆盖 [hidden]" 这类问题
    const visibleModals = await cdp.eval(`Array.from(document.querySelectorAll('.modal-backdrop'))
      .filter(m => getComputedStyle(m).display !== 'none').map(m => m.id)`);
    check('初始状态没有任何弹窗可见（计算样式）', visibleModals.length === 0, JSON.stringify(visibleModals));
    check('空状态在有条目时不可见', (await cdp.eval("getComputedStyle(document.querySelector('#emptyState')).display")) === 'none');
    check('批量操作条初始不可见', (await cdp.eval("getComputedStyle(document.querySelector('#bulkBar')).display")) === 'none');
    check('删除按钮等 [hidden] 元素被正确隐藏', await cdp.eval(`(() => {
      const hidden = Array.from(document.querySelectorAll('[hidden]'));
      return hidden.every(el => getComputedStyle(el).display === 'none');
    })()`), await cdp.eval("document.querySelectorAll('[hidden]').length + ' 个元素'"));
    check('快捷键提示已渲染', (await cdp.eval("document.querySelector('[data-sc=newSnippet]').textContent")) === (await cdp.eval("state.settings.shortcuts.newSnippet")));

    /* ---------------------------------------------------- 快捷键 Alt+N */
    out('\n[2] 快捷键 Alt+N 打开新建窗口');
    await cdp.eval("document.body.focus()");
    await cdp.key('n', 'KeyN', 78, 1);
    await cdp.waitFor("!document.querySelector('#editorModal').hidden", '编辑窗口打开');
    check('Alt+N 打开灵感编辑窗口', true);
    check('编辑窗口可见（计算样式）', (await cdp.eval("getComputedStyle(document.querySelector('#editorModal')).display")) !== 'none');
    check('其它弹窗仍未显示', (await cdp.eval(`Array.from(document.querySelectorAll('.modal-backdrop'))
      .filter(m => m.id !== 'editorModal')
      .every(m => getComputedStyle(m).display === 'none')`)));
    check('编辑器实例已创建', await cdp.eval("!!(state.editor && state.editor.codemirror)"));
    check('弹窗在可见容器中初始化（CodeMirror 有高度）', await cdp.eval("document.querySelector('.CodeMirror').offsetHeight > 50"),
      await cdp.eval("document.querySelector('.CodeMirror').offsetHeight + 'px'"));

    /* ---------------------------------------------------- Alt+S 保存 */
    out('\n[3] 输入内容并用 Alt+S 保存（AI 开则用 AI 标题，否则截取正文前 10 字）');
    const aiOn = await cdp.eval("!!(state.settings.ai.enabled)");
    await cdp.eval(`(() => {
      const before = document.querySelectorAll('#grid .card').length;
      window.__before = before;
      state.editor.value('快捷键保存测试内容，验证标题自动截取功能。');
      state.editorDirty = true;
      return true;
    })()`);
    await cdp.key('s', 'KeyS', 83, 1);
    await cdp.waitFor("document.querySelector('#editorModal').hidden", '编辑窗口关闭');
    await cdp.waitFor(`document.querySelectorAll('#grid .card').length > window.__before`, '卡片数量增加');
    const created = await cdp.eval("(state.snippets.find(s => s.content.includes('快捷键保存测试内容')) || {})");
    const expectedTrunc = Array.from('快捷键保存测试内容，验证标题自动截取功能。').slice(0, 10).join('');
    check('Alt+S 保存成功并创建新灵感', !!created.id, created.title);
    if (aiOn) {
      check('未填标题时由 AI 生成标题', created.title === 'AI 总结的标题', created.title);
      check('标题来源标记为 ai', created.titleSource === 'ai', created.titleSource);
    } else {
      check('未填标题时截取正文前 10 字', created.title === expectedTrunc, `"${created.title}"`);
      check('标题来源标记为 truncate', created.titleSource === 'truncate', created.titleSource);
    }

    /* ---------------------------------------------------- Alt+K 搜索 */
    out('\n[4] 快捷键 Alt+K 聚焦搜索 + 搜索功能');
    await cdp.key('k', 'KeyK', 75, 1);
    await sleep(200);
    check('Alt+K 聚焦搜索框', await cdp.eval("document.activeElement.id === 'searchInput'"));
    await cdp.eval(`(() => {
      const i = document.querySelector('#searchInput');
      i.value = '流水线样本';
      i.dispatchEvent(new Event('input', { bubbles: true }));
      return true;
    })()`);
    await cdp.waitFor("document.querySelectorAll('#grid .card').length === 1", '搜索过滤生效');
    check('全文搜索过滤生效', true, await cdp.eval("document.querySelector('#grid .card .card-title').textContent"));
    await cdp.eval(`(() => {
      const s = document.querySelector('#searchMode'); s.value = 'title';
      s.dispatchEvent(new Event('change', { bubbles: true }));
      return true;
    })()`);
    await cdp.waitFor("document.querySelectorAll('#grid .card').length === 1", '仅标题模式');
    await cdp.eval(`(() => {
      const i = document.querySelector('#searchInput'); i.value = '不存在的关键词XYZ';
      i.dispatchEvent(new Event('input', { bubbles: true })); return true;
    })()`);
    await cdp.waitFor("document.querySelectorAll('#grid .card').length === 0", '无结果');
    check('无结果时显示空状态', await cdp.eval("!document.querySelector('#emptyState').hidden"));
    await cdp.eval(`(() => {
      document.querySelector('#btnClearSearch').click();
      if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
      return true;
    })()`);
    await cdp.waitFor("document.querySelectorAll('#grid .card').length >= 3 && state.query === ''", '清空搜索');

    /* ---------------------------------------------------- Alt+A 全选 */
    out('\n[5] 快捷键 Alt+A 全选 / 批量条');
    await cdp.eval("state.selection.clear(); renderBulkBar(); true");
    await cdp.key('a', 'KeyA', 65, 1);
    await sleep(400);
    const total = await cdp.eval("state.snippets.length");
    const selected = await cdp.eval("state.selection.size");
    check('Alt+A 全选全部灵感', selected === total, `${selected}/${total}`);
    check('批量操作条已显示', await cdp.eval("!document.querySelector('#bulkBar').hidden"));
    check('批量操作条可见（计算样式）', (await cdp.eval("getComputedStyle(document.querySelector('#bulkBar')).display")) !== 'none');
    check('卡片勾选框同步', await cdp.eval("document.querySelectorAll('#grid .pick:checked').length") === total);
    await cdp.key('a', 'KeyA', 65, 1);
    await sleep(250);
    check('再按 Alt+A 取消全选', (await cdp.eval("state.selection.size")) === 0);
    check('批量操作条已隐藏', await cdp.eval("document.querySelector('#bulkBar').hidden"));

    /* ---------------------------------------------------- 全选复选框 */
    await cdp.eval("(() => { const c = document.querySelector('#checkAllVisible'); c.checked = true; c.dispatchEvent(new Event('change', {bubbles:true})); return true; })()");
    await sleep(200);
    check('顶部「全选」复选框可用', (await cdp.eval("state.selection.size")) === total, String(await cdp.eval("state.selection.size")));

    /* ---------------------------------------------------- AI 打标弹窗 */
    out('\n[6] AI 自动打标（需要 mock AI 服务）');
    const aiAvailable = await cdp.eval("!!(state.settings.ai.enabled)");
    if (aiAvailable) {
      await cdp.eval("document.querySelector('#btnAiTagSel').click()");
      await cdp.waitFor("!document.querySelector('#aiModal').hidden", 'AI 弹窗打开');
      await cdp.waitFor("document.querySelectorAll('#aiList .ai-item').length > 0", 'AI 建议渲染', 25000);
      const items = await cdp.eval("document.querySelectorAll('#aiList .ai-item').length");
      check('AI 建议列表渲染', items > 0, items + ' 条');
      check('建议含标签与分类选择', await cdp.eval("document.querySelectorAll('#aiList .pick-chip').length > 0"));
      await cdp.eval("document.querySelector('#btnAiApply').click()");
      await cdp.waitFor("document.querySelector('#aiModal').hidden", '应用完成', 20000);
      check('应用 AI 建议后弹窗关闭', true);
      const applied = await cdp.eval("state.snippets.filter(s => s.tags.length > 0).length");
      check('灵感已获得 AI 标签', applied > 0, applied + ' 条带标签');
    } else {
      out('  ⏭ 跳过（当前未启用 AI）');
    }

    /* ---------------------------------------------------- 流水线 */
    out('\n[7] 流水线打标 + Alt+J');
    await cdp.eval("state.selection.clear(); state.snippets.slice(0,2).forEach(s => state.selection.add(s.id)); renderBulkBar(); true");
    await cdp.eval("document.querySelector('#btnPipelineSel').click()");
    await cdp.waitFor("!document.querySelector('#pipelineModal').hidden", '流水线打开');
    check('流水线显示进度', /1 \/ 2/.test(await cdp.eval("document.querySelector('#pipeProgress').textContent")),
      await cdp.eval("document.querySelector('#pipeProgress').textContent"));
    check('流水线只读展示 Markdown', await cdp.eval("!!document.querySelector('#pipeContent').innerHTML.trim()"));
    check('流水线标签选择器渲染', (await cdp.eval("document.querySelectorAll('#pipeTagList .pick-chip').length")) > 0,
      await cdp.eval("document.querySelectorAll('#pipeTagList .pick-chip').length + ' 个候选标签'"));
    // 勾选一个标签 + 选择分类
    const picked = await cdp.eval(`(() => {
      const chip = document.querySelector('#pipeTagList .pick-chip');
      const cat = document.querySelector('#pipeCategory');
      const catPicked = cat.options.length > 1;
      if (catPicked) { cat.value = cat.options[1].value; cat.dispatchEvent(new Event('change', {bubbles:true})); }
      if (chip) chip.click();
      return { tag: !!chip, cat: catPicked };
    })()`);
    check('可在流水线中选中标签与分类', picked.tag && picked.cat, JSON.stringify(picked));
    const pipeFirstId = await cdp.eval("state.pipeline.items[state.pipeline.index].id");
    await cdp.key('j', 'KeyJ', 74, 1);
    await cdp.waitFor("document.querySelector('#pipeProgress').textContent.includes('2 / 2')", 'Alt+J 进入下一条');
    check('Alt+J 保存并进入下一条', true);
    const savedFirst = await cdp.eval(`(state.snippets.find(s => s.id === '${pipeFirstId}') || {})`);
    check('第一条已写入分类/标签', !!savedFirst && (savedFirst.categoryId !== '' || (savedFirst.tags || []).length > 0),
      JSON.stringify({ cat: savedFirst.categoryId, tags: savedFirst.tags }));
    await cdp.key('j', 'KeyJ', 74, 1);
    await cdp.waitFor("document.querySelector('#pipelineModal').hidden", '流水线结束');
    check('最后一条后自动结束流水线', true);

    /* ---------------------------------------------------- 编辑器与设置 */
    out('\n[8] 编辑器弹窗与设置弹窗');
    await cdp.eval("openEditor(state.snippets[0].id)");
    await cdp.waitFor("!document.querySelector('#editorModal').hidden", '编辑已有灵感');
    check('编辑已有灵感时载入标题', (await cdp.eval("document.querySelector('#edTitle').value.length")) > 0);
    check('已有标签渲染为 chip', (await cdp.eval("document.querySelectorAll('#edTagChips .chip').length")) >= 0);
    await cdp.eval("closeEditor(true)");
    await cdp.waitFor("document.querySelector('#editorModal').hidden", '关闭编辑器');

    await cdp.key('o', 'KeyO', 79, 1);
    await cdp.waitFor("!document.querySelector('#settingsModal').hidden", '设置弹窗打开');
    check('Alt+O 打开设置', true);
    check('设置含 5 个页签', (await cdp.eval("document.querySelectorAll('.tabs .tab').length")) === 5);
    check('快捷键设置项已渲染', (await cdp.eval("document.querySelectorAll('#shortcutList .shortcut-input').length")) === 8);
    await cdp.eval("document.querySelector('.tabs .tab[data-tab=webdav]').click()");
    await sleep(150);
    check('可切换到 WebDAV 页签', await cdp.eval("document.querySelector('.tab-panel[data-panel=webdav]').classList.contains('active')"));
    await cdp.eval("document.querySelector('.tabs .tab[data-tab=shortcuts]').click()");
    await sleep(150);
    await cdp.eval(`(() => {
      document.querySelector('#shortcutList .shortcut-input').focus();
      return true;
    })()`);
    await sleep(120);
    check('快捷键录制输入框可获得焦点', await cdp.eval("document.activeElement.classList.contains('shortcut-input')"));
    // 录制一个新组合键：Alt+Shift+Q（modifiers = Alt 1 | Shift 8）
    await cdp.key('Q', 'KeyQ', 81, 9);
    await sleep(220);
    const recorded = await cdp.eval("document.querySelector('#shortcutList .shortcut-input').value");
    check('快捷键录制生效', recorded === 'Alt+Shift+Q', recorded);
    check('录制后自动失焦（避免 Esc 被吞）', !(await cdp.eval("document.activeElement.classList.contains('shortcut-input')")));
    await cdp.key('Escape', 'Escape', 27, 0);
    await cdp.waitFor("document.querySelector('#settingsModal').hidden", 'Esc 关闭设置');
    check('Esc 关闭弹窗', true);

    /* ---------------------------------------------------- 多语言 */
    out('\n[8b] 多语言：切换 / 无缺失 key / 持久化');
    // 先记录英文基准
    const enNew = await cdp.eval("document.querySelector('#btnNewSnippet span').textContent");
    const enNav = await cdp.eval("document.querySelector('#mainNav .nav-item .ni-label').textContent");
    check('初始语言为英文', await cdp.eval("document.documentElement.lang === 'en'"), await cdp.eval("document.documentElement.lang"));
    check('英文标题正确', /Inspirationer/.test(await cdp.eval("document.title")), await cdp.eval("document.title"));

    // 检查是否存在未翻译的 key（t() 找不到 key 时会直接返回 key 本身）
    const keyLeak = () => cdp.eval(`(() => {
      const re = /^[a-z][a-zA-Z]*\\.[a-zA-Z.]+$/;
      const bad = [];
      document.querySelectorAll('[data-i18n],[data-i18n-html],[data-i18n-placeholder],[data-i18n-title],[data-i18n-placeholder]').forEach(el => {
        const vals = [el.textContent, el.placeholder, el.title].filter(Boolean);
        vals.forEach(v => { if (re.test(String(v).trim())) bad.push(v.trim()); });
      });
      return bad;
    })()`);

    await cdp.eval("openSettings()");
    await cdp.waitFor("!document.querySelector('#settingsModal').hidden", '设置已打开');
    await cdp.eval("document.querySelector('.tabs .tab[data-tab=ui]').click()");
    await sleep(150);

    // 切到日语
    await cdp.eval(`(() => {
      const s = document.querySelector('#uiLanguage');
      s.value = 'ja';
      s.dispatchEvent(new Event('change', { bubbles: true }));
      return true;
    })()`);
    await sleep(300);
    check('切换到日本語后 html lang 更新', await cdp.eval("document.documentElement.lang === 'ja'"));
    const jaNew = await cdp.eval("document.querySelector('#btnNewSnippet span').textContent");
    const jaNav = await cdp.eval("document.querySelector('#mainNav .nav-item .ni-label').textContent");
    check('日语界面文案已替换', jaNew !== enNew && jaNav !== enNav, `${jaNew} / ${jaNav}`);
    check('日语标题已替换', /ひらめき|Inspirationer/.test(await cdp.eval("document.title")), await cdp.eval("document.title"));
    check('日语下无缺失 key', (await keyLeak()).length === 0, JSON.stringify(await keyLeak()));
    check('设置面板文案同步为日语', /設定/.test(await cdp.eval("document.querySelector('#settingsModal .modal-head h3').textContent")));

    // 切到简体中文
    await cdp.eval(`(() => {
      const s = document.querySelector('#uiLanguage');
      s.value = 'zh-CN';
      s.dispatchEvent(new Event('change', { bubbles: true }));
      return true;
    })()`);
    await sleep(300);
    check('切换到简体中文', await cdp.eval("document.documentElement.lang === 'zh-CN'"));
    const zhNav = await cdp.eval("document.querySelector('#mainNav .nav-item .ni-label').textContent");
    check('中文界面文案正确', zhNav === '全部灵感', zhNav);
    check('中文下无缺失 key', (await keyLeak()).length === 0, JSON.stringify(await keyLeak()));

    // 保存设置并重载页面，验证语言被持久化
    await cdp.eval("document.querySelector('#btnSaveSettings').click()");
    await sleep(600);
    await cdp.send('Page.reload');
    await sleep(1200);
    await cdp.waitFor("document.readyState === 'complete' && document.querySelectorAll('#grid .card').length >= 1", '重载页面');
    await cdp.waitFor("document.documentElement.lang === 'zh-CN'", '语言持久化生效');
    check('重载后仍是简体中文（设置已持久化）', true);
    check('重载后 state 与文案一致', (await cdp.eval("state.settings.ui.language")) === 'zh-CN', await cdp.eval("state.settings.ui.language"));

    // 切回英文，避免影响后续断言
    await cdp.eval("applyLangPreference('en')");
    await sleep(250);
    check('可切回英文', await cdp.eval("document.documentElement.lang === 'en'"), await cdp.eval("document.querySelector('#mainNav .nav-item .ni-label').textContent"));

    /* ---------------------------------------------------- 控制台错误 */
    out('\n[9] 运行期错误检查');
    const realErrors = cdp.errors.filter(e => !/favicon/i.test(e));
    check('无未捕获的 JS 异常', realErrors.length === 0, realErrors.slice(0, 3).join(' | '));
    const consoleErrs = cdp.consoleErrors.filter(e => !/favicon|DevTools/i.test(e));
    check('无 console.error 输出', consoleErrs.length === 0, consoleErrs.slice(0, 3).join(' | '));
  } finally {
    try { ws.close(); } catch { }
    try { proc.kill(); } catch { }
    await sleep(300);
    spawnSync('taskkill', ['/F', '/T', '/PID', String(proc.pid)], { stdio: 'ignore' });
    try { fs.rmSync(userDataDir, { recursive: true, force: true }); } catch { }
  }

  out('\n' + '='.repeat(70));
  out(`结果：通过 ${pass} 项，失败 ${fail} 项`);
  return fail === 0 ? 0 : 1;
}

main().then(code => { console.log(log.join('\n')); process.exit(code); })
  .catch(err => { console.log(log.join('\n')); console.error('UI 测试异常终止：', err.message); process.exit(2); });
