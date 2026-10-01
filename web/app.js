/* ==========================================================================
 * 灵感管理器 · 前端逻辑（原生 JS，无构建步骤）
 * ========================================================================== */
'use strict';

const $ = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

/* ------------------------------------------------------------------ 状态 */
const state = {
  version: '',
  dataDir: '',
  settings: null,
  tags: [],
  categories: [],
  stats: {},
  snippets: [],
  selection: new Set(),
  view: 'all',          // all | uncategorized | pinned | archived | category | tag
  viewId: null,
  query: '',
  mode: 'all',
  sort: 'updated',
  editor: null,
  editingId: null,
  editorTags: new Set(),
  editorDirty: false,
  pipeline: { items: [], index: 0, drafts: {} },
  tagEditor: { kind: 'tag', id: null, color: '#3b82f6' },
  aiItems: [],
};

const SHORTCUT_DEFS = [
  { key: 'newSnippet', label: '新建灵感', desc: '在任意位置打开灵感编辑窗口' },
  { key: 'saveSnippet', label: '保存灵感', desc: '编辑窗口中保存并关闭' },
  { key: 'focusSearch', label: '聚焦搜索', desc: '跳到搜索框并全选' },
  { key: 'selectAll', label: '全选 / 取消全选', desc: '选中当前列表中的全部灵感' },
  { key: 'pipelineNext', label: '流水线下一条', desc: '流水线中保存当前条目并进入下一条' },
  { key: 'togglePreview', label: '切换预览', desc: '编辑窗口中切换 Markdown 预览' },
  { key: 'openSettings', label: '打开设置', desc: '打开设置面板' },
  { key: 'closeModal', label: '关闭弹窗', desc: '关闭当前弹窗' },
];

const DEFAULT_SHORTCUTS = {
  newSnippet: 'Alt+N', saveSnippet: 'Alt+S', focusSearch: 'Alt+K', selectAll: 'Alt+A',
  pipelineNext: 'Alt+J', togglePreview: 'Alt+P', openSettings: 'Alt+O', closeModal: 'Escape',
};

/* ------------------------------------------------------------------ 工具 */
async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  const text = await res.text();
  let data = null;
  if (text) { try { data = JSON.parse(text); } catch (_) { data = { error: text }; } }
  if (!res.ok) throw new Error((data && data.error) || ('请求失败 HTTP ' + res.status));
  return data;
}

function toast(message, kind = 'info', ms = 3200) {
  const el = document.createElement('div');
  el.className = 'toast ' + kind;
  el.textContent = message;
  $('#toasts').appendChild(el);
  setTimeout(() => { el.style.opacity = '0'; el.style.transition = 'opacity .3s'; }, ms - 300);
  setTimeout(() => el.remove(), ms);
}

function escapeHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function mdToHtml(text) {
  const src = String(text || '');
  let html = '';
  try {
    if (window.marked) {
      html = typeof window.marked.parse === 'function' ? window.marked.parse(src) : window.marked(src);
    } else if (window.EasyMDE && EasyMDE.prototype.markdown) {
      html = EasyMDE.prototype.markdown(src);
    }
  } catch (err) {
    html = '';
  }
  if (!html) return '<p>' + escapeHtml(src).replace(/\n/g, '<br>') + '</p>';
  if (window.DOMPurify) {
    html = window.DOMPurify.sanitize(html, { ADD_ATTR: ['target', 'rel'] });
  }
  return html;
}

function plainExcerpt(text, n = 260) {
  const clean = String(text || '')
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/`([^`]*)`/g, '$1')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/^[>#\-*+\d.\s]+/gm, '')
    .replace(/[*_~]/g, '')
    .replace(/\s+/g, ' ')
    .trim();
  return clean.length > n ? clean.slice(0, n) + '…' : clean;
}

function fmtTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return '';
  const diff = (Date.now() - d.getTime()) / 1000;
  if (diff < 60) return '刚刚';
  if (diff < 3600) return Math.floor(diff / 60) + ' 分钟前';
  if (diff < 86400) return Math.floor(diff / 3600) + ' 小时前';
  if (diff < 86400 * 7) return Math.floor(diff / 86400) + ' 天前';
  return d.toLocaleDateString('zh-CN');
}

function fmtFull(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return isNaN(d.getTime()) ? '' : d.toLocaleString('zh-CN');
}

// Go 的零值时间会序列化成 0001-01-01，这里当作「未设置」
function isRealTime(iso) {
  if (!iso) return false;
  const d = new Date(iso);
  return !isNaN(d.getTime()) && d.getFullYear() > 2000;
}

function debounce(fn, ms) {
  let t = null;
  return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
}

const tagById = (id) => state.tags.find(t => t.id === id);
const catById = (id) => state.categories.find(c => c.id === id);
const catColor = (id) => (catById(id) || {}).color || 'var(--border)';

/* ------------------------------------------------------------------ 模态框 */
function openModal(id) { const el = $('#' + id); if (el) el.hidden = false; }
function closeModal(id) { const el = $('#' + id); if (el) el.hidden = true; }
function anyModalOpen() { return $$('.modal-backdrop').some(m => !m.hidden); }
function topModal() { const open = $$('.modal-backdrop').filter(m => !m.hidden); return open.length ? open[open.length - 1] : null; }

function confirmDialog(text, title = '确认操作') {
  return new Promise((resolve) => {
    $('#confirmTitle').textContent = title;
    $('#confirmText').textContent = text;
    openModal('confirmModal');
    const done = (val) => {
      closeModal('confirmModal');
      $('#btnConfirmYes').onclick = null;
      $('#btnConfirmNo').onclick = null;
      resolve(val);
    };
    $('#btnConfirmYes').onclick = () => done(true);
    $('#btnConfirmNo').onclick = () => done(false);
  });
}

/* ------------------------------------------------------------------ 数据加载 */
async function bootstrap() {
  const data = await api('GET', '/api/bootstrap');
  state.version = data.version;
  state.dataDir = data.dataDir;
  state.settings = data.settings;
  state.tags = data.tags || [];
  state.categories = data.categories || [];
  state.stats = data.stats || {};
  applySettingsToUI();
  $('#appVersion').textContent = 'v' + data.version;
  $('#dataDirText').value = data.dataDir;
}

function applySettingsToUI() {
  const s = state.settings;
  document.documentElement.dataset.theme = s.ui.theme || 'dark';
  updateShortcutBadges();
}

function updateShortcutBadges() {
  const sc = (state.settings && state.settings.shortcuts) || DEFAULT_SHORTCUTS;
  $$('[data-sc]').forEach(el => {
    const k = el.dataset.sc;
    if (sc[k]) el.textContent = sc[k];
    else el.hidden = true;
  });
}

function applyTaxonomy(data) {
  if (data.tags) state.tags = data.tags;
  if (data.categories) state.categories = data.categories;
  if (data.stats) state.stats = data.stats;
  renderSidebar();
  fillDatalist();
}

async function loadSnippets() {
  const params = new URLSearchParams();
  if (state.query) { params.set('query', state.query); params.set('mode', state.mode); }
  if (state.view === 'category') params.set('categoryId', state.viewId);
  if (state.view === 'uncategorized') params.set('categoryId', '__none__');
  if (state.view === 'tag') params.set('tagIds', state.viewId);
  if (state.view === 'archived') params.set('archive', 'only');
  params.set('sort', state.sort);

  const data = await api('GET', '/api/snippets?' + params.toString());
  let items = data.items || [];
  if (state.view === 'pinned') items = items.filter(s => s.pinned);
  state.snippets = items;
  // 清理已不存在的选择
  const visible = new Set(items.map(s => s.id));
  state.selection.forEach(id => { if (!visible.has(id)) state.selection.delete(id); });
  renderSnippets();
}

/* ------------------------------------------------------------------ 侧栏渲染 */
function renderSidebar() {
  const st = state.stats || {};
  $('#countAll').textContent = st.snippets || 0;
  $('#countUncat').textContent = st.uncategorized || 0;
  $('#countArchived').textContent = st.archived || 0;
  $('#countPinned').textContent = st.pinned || 0;

  // 批量操作里的分类下拉随分类变化同步
  const bulkCat = $('#bulkCategory');
  if (bulkCat) {
    const cur = bulkCat.value;
    bulkCat.innerHTML = '<option value="">— 设置分类 —</option><option value="__none__">（未分类）</option>' +
      state.categories.map(c => `<option value="${c.id}">${escapeHtml(c.name)}</option>`).join('');
    bulkCat.value = cur;
  }

  $$('#mainNav .nav-item').forEach(btn => {
    const v = btn.dataset.view;
    btn.classList.toggle('active', state.view === v);
  });

  // 分类
  const catList = $('#catList');
  catList.innerHTML = '';
  state.categories.forEach(c => {
    const count = (st.byCategory || {})[c.id] || 0;
    const btn = document.createElement('button');
    btn.className = 'cat-item' + (state.view === 'category' && state.viewId === c.id ? ' active' : '');
    btn.innerHTML = `<span class="dot" style="--c:${escapeHtml(c.color)};background:${escapeHtml(c.color)}"></span>
      <span class="name">${escapeHtml(c.name)}</span><span class="count">${count}</span>`;
    btn.onclick = () => { state.view = 'category'; state.viewId = c.id; state.selection.clear(); loadSnippets(); renderSidebar(); };
    btn.oncontextmenu = (e) => { e.preventDefault(); openTagEditor('category', c.id); };
    catList.appendChild(btn);
  });
  if (!state.categories.length) {
    catList.innerHTML = '<div class="hint" style="padding:4px 9px">还没有分类，点右上角 ＋ 新建</div>';
  }

  // 标签
  const tagList = $('#tagList');
  tagList.innerHTML = '';
  state.tags.forEach(t => {
    const count = (st.byTag || {})[t.id] || 0;
    const chip = document.createElement('span');
    chip.className = 'chip clickable' + (state.view === 'tag' && state.viewId === t.id ? ' active' : '');
    chip.style.setProperty('--c', t.color);
    chip.innerHTML = `<span class="chip-dot"></span>${escapeHtml(t.name)} <span style="opacity:.6">${count}</span>`;
    chip.onclick = () => {
      if (state.view === 'tag' && state.viewId === t.id) { state.view = 'all'; state.viewId = null; }
      else { state.view = 'tag'; state.viewId = t.id; }
      state.selection.clear(); loadSnippets(); renderSidebar();
    };
    chip.oncontextmenu = (e) => { e.preventDefault(); openTagEditor('tag', t.id); };
    tagList.appendChild(chip);
  });
  if (!state.tags.length) {
    tagList.innerHTML = '<div class="hint">还没有标签</div>';
  }
}

function fillDatalist() {
  const dl = $('#tagSuggest');
  dl.innerHTML = state.tags.map(t => `<option value="${escapeHtml(t.name)}"></option>`).join('');
}

/* ------------------------------------------------------------------ 卡片渲染 */
function cardHtml(sn) {
  const cat = catById(sn.categoryId);
  const tags = (sn.tags || []).map(tagById).filter(Boolean);
  const meta = [];
  if (cat) meta.push(`<span class="chip cat-chip" style="--c:${escapeHtml(cat.color)}"><span class="chip-dot"></span>${escapeHtml(cat.name)}</span>`);
  tags.forEach(t => meta.push(`<span class="chip" style="--c:${escapeHtml(t.color)}"><span class="chip-dot"></span>${escapeHtml(t.name)}</span>`));

  const preview = state.settings.ui.cardPreview
    ? `<div class="card-content markdown">${mdToHtml(sn.content)}</div>`
    : `<div class="card-content plain">${escapeHtml(plainExcerpt(sn.content))}</div>`;

  const srcLabel = sn.titleSource === 'ai' ? 'AI 标题' : (sn.titleSource === 'truncate' ? '自动标题' : '');
  return `
    <article class="card ${state.selection.has(sn.id) ? 'selected' : ''}" data-id="${sn.id}" style="--cat-color:${escapeHtml(cat ? cat.color : 'var(--border)')}">
      <div class="card-head">
        <input type="checkbox" class="pick" ${state.selection.has(sn.id) ? 'checked' : ''}>
        <h3 class="card-title">${escapeHtml(sn.title || '未命名灵感')}</h3>
        <div class="card-badges">${sn.pinned ? '<span class="pin-badge">📌</span>' : ''}${sn.archived ? '<span class="pin-badge">🗄️</span>' : ''}</div>
        <div class="card-actions">
          <button class="icon-btn act-pin" title="${sn.pinned ? '取消置顶' : '置顶'}">${sn.pinned ? '📍' : '📌'}</button>
          <button class="icon-btn act-delete" title="删除">🗑</button>
        </div>
      </div>
      ${meta.length ? `<div class="card-meta">${meta.join('')}</div>` : ''}
      ${preview}
      <div class="card-foot">
        <span title="${escapeHtml(fmtFull(sn.updatedAt))}">更新于 ${escapeHtml(fmtTime(sn.updatedAt))}</span>
        ${srcLabel ? `<span class="src-badge">${srcLabel}</span>` : ''}
      </div>
    </article>`;
}

function renderSnippets() {
  const grid = $('#grid');
  const items = state.snippets;
  grid.innerHTML = items.map(cardHtml).join('');
  $('#emptyState').hidden = items.length > 0;
  $('#viewCount').textContent = items.length + ' 条';

  const titles = {
    all: '全部灵感', uncategorized: '未分类', pinned: '置顶', archived: '归档',
  };
  let title = titles[state.view];
  if (state.view === 'category') title = '分类：' + ((catById(state.viewId) || {}).name || '');
  if (state.view === 'tag') title = '标签：' + ((tagById(state.viewId) || {}).name || '');
  $('#viewTitle').textContent = title || '灵感';

  const hint = $('#searchHint');
  if (state.query) {
    hint.hidden = false;
    hint.textContent = `搜索「${state.query}」（${state.mode === 'title' ? '仅标题' : state.mode === 'content' ? '仅正文' : '全文'}）`;
  } else hint.hidden = true;

  // 卡片事件
  $$('#grid .card').forEach(card => {
    const id = card.dataset.id;
    card.addEventListener('click', (e) => {
      if (e.target.closest('.pick') || e.target.closest('.card-actions')) return;
      openEditor(id);
    });
    $('.pick', card).addEventListener('change', (e) => {
      e.stopPropagation();
      toggleSelect(id, e.target.checked);
    });
    $('.act-pin', card).addEventListener('click', async (e) => {
      e.stopPropagation();
      const sn = state.snippets.find(s => s.id === id);
      await api('PATCH', '/api/snippets/' + id, { pinned: !sn.pinned });
      await loadSnippets(); renderSidebar();
    });
    $('.act-delete', card).addEventListener('click', async (e) => {
      e.stopPropagation();
      await deleteSnippets([id]);
    });
  });

  // 全选状态
  const allSelected = items.length > 0 && items.every(s => state.selection.has(s.id));
  $('#checkAllVisible').checked = allSelected;
  renderBulkBar();
}

function toggleSelect(id, on) {
  if (on) state.selection.add(id); else state.selection.delete(id);
  const card = $(`#grid .card[data-id="${id}"]`);
  if (card) card.classList.toggle('selected', on);
  $('#checkAllVisible').checked = state.snippets.length > 0 && state.snippets.every(s => state.selection.has(s.id));
  renderBulkBar();
}

function renderBulkBar() {
  const n = state.selection.size;
  $('#bulkBar').hidden = n === 0;
  $('#bulkCount').textContent = `已选 ${n} 条`;
}

async function deleteSnippets(ids) {
  if (state.settings.ui.confirmDelete) {
    const ok = await confirmDialog(`确定删除这 ${ids.length} 条灵感吗？此操作不可撤销。`, '删除灵感');
    if (!ok) return;
  }
  await api('POST', '/api/snippets/bulk', { ids, action: 'delete' });
  ids.forEach(id => state.selection.delete(id));
  toast(`已删除 ${ids.length} 条灵感`, 'ok');
  await loadSnippets(); renderSidebar();
}

/* ------------------------------------------------------------------ 编辑器 */
function ensureEditor() {
  if (state.editor) return state.editor;
  const tb = (name, action, cls, title, extra) => Object.assign({ name, action, className: cls, title }, extra || {});
  state.editor = new EasyMDE({
    element: $('#edContent'),
    autoDownloadFontAwesome: false,
    spellChecker: false,
    autofocus: false,
    placeholder: '随手写下你的灵感…支持 Markdown：标题、列表、引用、代码块、表格、图片链接…',
    status: ['lines', 'words'],
    minHeight: '260px',
    toolbar: [
      tb('bold', EasyMDE.toggleBold, 'tb-bold', '加粗 (Ctrl+B)'),
      tb('italic', EasyMDE.toggleItalic, 'tb-italic', '斜体 (Ctrl+I)'),
      tb('strikethrough', EasyMDE.toggleStrikethrough, 'tb-strikethrough', '删除线'),
      '|',
      tb('heading', EasyMDE.toggleHeadingSmaller, 'tb-heading', '标题'),
      tb('quote', EasyMDE.toggleBlockquote, 'tb-quote', '引用'),
      tb('unordered-list', EasyMDE.toggleUnorderedList, 'tb-unordered-list', '无序列表'),
      tb('ordered-list', EasyMDE.toggleOrderedList, 'tb-ordered-list', '有序列表'),
      '|',
      tb('link', EasyMDE.drawLink, 'tb-link', '插入链接 (Ctrl+K)'),
      tb('image', EasyMDE.drawImage, 'tb-image', '插入图片'),
      tb('code', EasyMDE.toggleCodeBlock, 'tb-code', '代码块'),
      tb('table', EasyMDE.drawTable, 'tb-table', '表格'),
      tb('horizontal-rule', EasyMDE.drawHorizontalRule, 'tb-horizontal-rule', '分割线'),
      '|',
      tb('preview', EasyMDE.togglePreview, 'tb-preview no-disable', '预览 (Alt+P)'),
      tb('side-by-side', EasyMDE.toggleSideBySide, 'tb-side-by-side no-disable no-mobile', '并排预览'),
      tb('fullscreen', EasyMDE.toggleFullScreen, 'tb-fullscreen no-disable no-mobile', '全屏 (F11)'),
      '|',
      tb('guide', 'https://marked.js.org/using_advanced', 'tb-guide no-disable no-mobile', 'Markdown 语法'),
    ],
    renderingConfig: { singleLineBreaks: true, codeSyntaxHighlighting: false },
    previewRender: (txt) => mdToHtml(txt),
  });
  state.editor.codemirror.on('change', () => { state.editorDirty = true; updateEditorMeta(); });
  return state.editor;
}

function updateEditorMeta() {
  const content = state.editor ? state.editor.value() : '';
  const chars = content.length;
  const tags = state.editorTags.size;
  const cat = catById($('#edCategory').value);
  $('#edMeta').textContent = `共 ${chars} 字 · 标签 ${tags} 个 · 分类 ${cat ? cat.name : '未分类'}`;
}

function renderEditorTags() {
  const box = $('#edTagChips');
  box.innerHTML = '';
  state.editorTags.forEach(id => {
    const t = tagById(id);
    if (!t) return;
    const chip = document.createElement('span');
    chip.className = 'chip';
    chip.style.setProperty('--c', t.color);
    chip.innerHTML = `<span class="chip-dot"></span>${escapeHtml(t.name)}<span class="x" title="移除">✕</span>`;
    $('.x', chip).onclick = () => { state.editorTags.delete(id); state.editorDirty = true; renderEditorTags(); updateEditorMeta(); };
    box.appendChild(chip);
  });
}

function fillEditorCategorySelect(selected) {
  const sel = $('#edCategory');
  sel.innerHTML = '<option value="">未分类</option>' +
    state.categories.map(c => `<option value="${c.id}">${escapeHtml(c.name)}</option>`).join('');
  sel.value = selected || '';
}

async function openEditor(id) {
  // 先显示弹窗：CodeMirror 在隐藏容器里初始化会算错尺寸
  openModal('editorModal');
  const ed = ensureEditor();
  state.editorTags = new Set();
  state.editorDirty = false;

  if (id) {
    const sn = await api('GET', '/api/snippets/' + id);
    state.editingId = sn.id;
    $('#edHeading').textContent = '编辑灵感';
    $('#edTitle').value = sn.title || '';
    ed.value(sn.content || '');
    fillEditorCategorySelect(sn.categoryId);
    (sn.tags || []).forEach(t => state.editorTags.add(t));
    $('#btnEdDelete').hidden = false;
  } else {
    state.editingId = null;
    $('#edHeading').textContent = '新建灵感';
    $('#edTitle').value = '';
    ed.value('');
    fillEditorCategorySelect(state.view === 'category' ? state.viewId : '');
    $('#btnEdDelete').hidden = true;
  }
  renderEditorTags();
  updateEditorMeta();
  updateShortcutBadges();
  setTimeout(() => {
    if (state.editor && state.editor.codemirror) state.editor.codemirror.refresh();
    $('#edTitle').focus();
  }, 60);
}

async function closeEditor(force) {
  if (!force && state.editorDirty && (state.editor.value().trim() || $('#edTitle').value.trim())) {
    const ok = await confirmDialog('当前内容尚未保存，确定要关闭吗？', '放弃编辑');
    if (!ok) return;
  }
  closeModal('editorModal');
  state.editingId = null;
  state.editorDirty = false;
}

async function addTagByName(name) {
  const clean = String(name || '').trim().replace(/^#/, '');
  if (!clean) return null;
  let t = state.tags.find(x => x.name.toLowerCase() === clean.toLowerCase());
  if (!t) {
    const res = await api('POST', '/api/tags', { name: clean });
    state.tags = res.items || state.tags;
    state.stats = res.stats || state.stats;
    fillDatalist(); renderSidebar();
    t = res.tag;
    toast(`已新建标签「${clean}」`, 'ok');
  }
  return t;
}

async function saveEditor() {
  const ed = state.editor;
  const payload = {
    title: $('#edTitle').value.trim(),
    content: ed.value(),
    tags: Array.from(state.editorTags),
    categoryId: $('#edCategory').value,
    archived: false,
    pinned: false,
  };
  if (!payload.content.trim() && !payload.title) {
    toast('内容或标题至少填一个', 'err');
    return;
  }
  try {
    if (state.editingId) {
      const cur = state.snippets.find(s => s.id === state.editingId);
      payload.pinned = cur ? cur.pinned : false;
      payload.archived = cur ? cur.archived : false;
      const res = await api('PUT', '/api/snippets/' + state.editingId, payload);
      toast('已保存', 'ok');
      state.stats = res.stats || state.stats;
    } else {
      const res = await api('POST', '/api/snippets', payload);
      toast('灵感已创建：' + (res.snippet.title || ''), 'ok');
      (res.warnings || []).forEach(w => toast(w, 'warn', 5000));
      state.stats = res.stats || state.stats;
    }
    state.editorDirty = false;
    closeModal('editorModal');
    state.editingId = null;
    await loadSnippets(); renderSidebar();
  } catch (err) {
    toast(err.message, 'err', 5000);
  }
}

/* ------------------------------------------------------------------ 设置 */
function openSettings() {
  fillSettingsForm();
  renderShortcutEditor();
  openModal('settingsModal');
}

function fillSettingsForm() {
  const s = state.settings;
  $('#aiEnabled').checked = !!s.ai.enabled;
  $('#aiBaseUrl').value = s.ai.baseUrl || '';
  $('#aiApiKey').value = s.ai.apiKey || '';
  $('#aiModel').value = s.ai.model || '';
  $('#aiTemp').value = s.ai.temperature;
  $('#aiMaxTokens').value = s.ai.maxTokens;
  $('#aiTimeout').value = s.ai.timeoutSeconds;

  $('#davEnabled').checked = !!s.webdav.enabled;
  $('#davUrl').value = s.webdav.url || '';
  $('#davUser').value = s.webdav.username || '';
  $('#davPass').value = s.webdav.password || '';
  $('#davDir').value = s.webdav.remoteDir || '';
  $('#davInterval').value = s.webdav.intervalMinutes || 60;
  $('#davKeep').value = s.webdav.keepRemote || 10;
  $('#davStatus').textContent = isRealTime(s.webdav.lastBackup)
    ? `上次备份：${fmtFull(s.webdav.lastBackup)} — ${s.webdav.lastStatus || ''}`
    : '尚未备份过';

  $('#uiTheme').value = s.ui.theme || 'dark';
  $('#uiTitleRunes').value = s.ui.titleMaxRunes || 10;
  $('#uiCardPreview').checked = !!s.ui.cardPreview;
  $('#uiConfirmDelete').checked = !!s.ui.confirmDelete;
  $('#dataDirText').value = state.dataDir;
  $('#uiTheme').onchange = () => {
    document.documentElement.dataset.theme = $('#uiTheme').value;
  };
}

function collectSettings() {
  const cur = state.settings;
  return {
    ai: {
      enabled: $('#aiEnabled').checked,
      baseUrl: $('#aiBaseUrl').value.trim(),
      apiKey: $('#aiApiKey').value.trim(),
      model: $('#aiModel').value.trim(),
      temperature: parseFloat($('#aiTemp').value) || 0,
      maxTokens: parseInt($('#aiMaxTokens').value, 10) || 512,
      timeoutSeconds: parseInt($('#aiTimeout').value, 10) || 45,
    },
    webdav: {
      enabled: $('#davEnabled').checked,
      url: $('#davUrl').value.trim(),
      username: $('#davUser').value.trim(),
      password: $('#davPass').value,
      remoteDir: $('#davDir').value.trim() || 'inspirationer',
      intervalMinutes: parseInt($('#davInterval').value, 10) || 60,
      keepRemote: parseInt($('#davKeep').value, 10) || 10,
      lastBackup: cur.webdav.lastBackup,
      lastStatus: cur.webdav.lastStatus,
    },
    shortcuts: Object.assign({}, cur.shortcuts),
    ui: {
      theme: $('#uiTheme').value,
      titleMaxRunes: parseInt($('#uiTitleRunes').value, 10) || 10,
      cardPreview: $('#uiCardPreview').checked,
      confirmDelete: $('#uiConfirmDelete').checked,
      autoBackupHint: false,
    },
  };
}

async function saveSettings() {
  const payload = collectSettings();
  const res = await api('PUT', '/api/settings', payload);
  state.settings = res.settings;
  applySettingsToUI();
  toast('设置已保存', 'ok');
  closeModal('settingsModal');
  await loadSnippets();
  renderSidebar();
}

function renderShortcutEditor() {
  const wrap = $('#shortcutList');
  wrap.innerHTML = '';
  const sc = state.settings.shortcuts;
  SHORTCUT_DEFS.forEach(def => {
    const row = document.createElement('div');
    row.className = 'shortcut-row';
    row.innerHTML = `
      <div class="label">${escapeHtml(def.label)}<div class="desc">${escapeHtml(def.desc)}</div></div>
      <input class="shortcut-input" readonly data-key="${def.key}" value="${escapeHtml(sc[def.key] || '')}">`;
    wrap.appendChild(row);
  });

  $$('#shortcutList .shortcut-input').forEach(input => {
    input.addEventListener('focus', () => input.classList.add('recording'));
    input.addEventListener('blur', () => input.classList.remove('recording'));
    input.addEventListener('keydown', (e) => {
      e.preventDefault();
      e.stopPropagation();
      if (e.key === 'Backspace' || e.key === 'Delete') {
        input.value = '';
        state.settings.shortcuts[input.dataset.key] = '';
        markDuplicateShortcuts();
        return;
      }
      const spec = shortcutFromEvent(e);
      if (!spec) return;
      input.value = spec;
      state.settings.shortcuts[input.dataset.key] = spec;
      markDuplicateShortcuts();
      // 录制完成后立即失焦，避免焦点留在录制框里吞掉 Esc 等按键
      input.blur();
    });
  });
  markDuplicateShortcuts();
}

function markDuplicateShortcuts() {
  const inputs = $$('#shortcutList .shortcut-input');
  const counts = {};
  inputs.forEach(i => { if (i.value) counts[i.value] = (counts[i.value] || 0) + 1; });
  inputs.forEach(i => i.classList.toggle('dupe', !!i.value && counts[i.value] > 1));
}

/* ------------------------------------------------------------------ 快捷键 */
function shortcutFromEvent(e) {
  const parts = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey) parts.push('Meta');
  let key = e.key;
  if (key === 'Control' || key === 'Alt' || key === 'Shift' || key === 'Meta') return '';
  if (e.altKey && e.code && /^Key[A-Z]$/.test(e.code)) key = e.code.slice(3);
  else if (e.code === 'Space') key = 'Space';
  else if (key === ' ') key = 'Space';
  else if (key && key.length === 1) key = key.toUpperCase();
  else if (key === 'Escape') key = 'Escape';
  else if (key === 'Enter') key = 'Enter';
  else if (key === 'Tab') key = 'Tab';
  else if (key === 'ArrowUp' || key === 'ArrowDown' || key === 'ArrowLeft' || key === 'ArrowRight') key = key;
  if (!key) return '';
  parts.push(key);
  return parts.join('+');
}

function matchShortcut(e, spec) {
  if (!spec) return false;
  const parts = spec.split('+').map(s => s.trim().toLowerCase()).filter(Boolean);
  if (!parts.length) return false;
  const target = parts[parts.length - 1];
  const need = {
    ctrl: parts.includes('ctrl'),
    alt: parts.includes('alt'),
    shift: parts.includes('shift'),
    meta: parts.includes('meta') || parts.includes('cmd') || parts.includes('super'),
  };
  if (!!e.ctrlKey !== need.ctrl) return false;
  if (!!e.altKey !== need.alt) return false;
  if (!!e.shiftKey !== need.shift) return false;
  if (!!e.metaKey !== need.meta) return false;

  const key = (e.key || '').toLowerCase();
  const code = e.code || '';
  const alias = {
    esc: 'escape', escape: 'escape', space: ' ', spacebar: ' ', ' ': ' ',
    enter: 'enter', return: 'enter', tab: 'tab', del: 'delete',
    up: 'arrowup', down: 'arrowdown', left: 'arrowleft', right: 'arrowright',
    plus: '+', comma: ',', period: '.', slash: '/', semicolon: ';', backquote: '`',
  };
  const wantKey = alias[target] || target;
  if (key === wantKey) return true;
  if (wantKey.length === 1 && code === 'Key' + wantKey.toUpperCase()) return true;
  if (key === ' ' && wantKey === 'space') return true;
  if (target === 'delete' && (key === 'backspace' || key === 'delete')) return true;
  return false;
}

function handleKeydown(e) {
  const sc = (state.settings && state.settings.shortcuts) || DEFAULT_SHORTCUTS;
  const editorOpen = !$('#editorModal').hidden;
  const pipelineOpen = !$('#pipelineModal').hidden;
  const settingsOpen = !$('#settingsModal').hidden;

  // 1) 弹窗内的快捷键优先
  if (editorOpen) {
    if (matchShortcut(e, sc.saveSnippet)) { e.preventDefault(); saveEditor(); return; }
    if (matchShortcut(e, sc.togglePreview)) {
      e.preventDefault();
      if (state.editor) EasyMDE.togglePreview(state.editor);
      return;
    }
  }
  if (pipelineOpen && matchShortcut(e, sc.pipelineNext)) {
    e.preventDefault();
    pipelineSaveNext();
    return;
  }
  if (anyModalOpen() && (e.key === 'Escape' || matchShortcut(e, sc.closeModal))) {
    // EasyMDE 全屏时先退出全屏，再关弹窗
    if (editorOpen && state.editor && typeof state.editor.isFullscreenActive === 'function' && state.editor.isFullscreenActive()) {
      e.preventDefault();
      EasyMDE.toggleFullScreen(state.editor);
      return;
    }
    e.preventDefault();
    if (!$('#confirmModal').hidden) { $('#btnConfirmNo').click(); return; }
    if (editorOpen) { closeEditor(false); return; }
    if (pipelineOpen) { finishPipeline(); return; }
    topModal().hidden = true;
    return;
  }

  // 2) 有弹窗时不再触发全局快捷键（避免误操作）
  if (anyModalOpen()) return;

  if (matchShortcut(e, sc.newSnippet)) { e.preventDefault(); openEditor(null); return; }
  if (matchShortcut(e, sc.focusSearch)) {
    e.preventDefault();
    $('#searchInput').focus();
    $('#searchInput').select();
    return;
  }
  if (matchShortcut(e, sc.openSettings)) { e.preventDefault(); openSettings(); return; }
  if (matchShortcut(e, sc.selectAll)) {
    e.preventDefault();
    const all = state.snippets.length > 0 && state.snippets.every(s => state.selection.has(s.id));
    toggleSelectAll(!all);
    return;
  }
}

function toggleSelectAll(on) {
  state.snippets.forEach(s => { if (on) state.selection.add(s.id); else state.selection.delete(s.id); });
  $$('#grid .card').forEach(card => {
    const on2 = state.selection.has(card.dataset.id);
    card.classList.toggle('selected', on2);
    const pick = $('.pick', card);
    if (pick) pick.checked = on2;
  });
  $('#checkAllVisible').checked = on;
  renderBulkBar();
}

/* ------------------------------------------------------------------ 标签/分类编辑 */
function openTagEditor(kind, id) {
  state.tagEditor.kind = kind;
  state.tagEditor.id = id || null;
  const isTag = kind === 'tag';
  $('#tagEditorHeading').textContent = (id ? '编辑' : '新建') + (isTag ? '标签' : '分类');
  const cur = id ? (isTag ? tagById(id) : catById(id)) : null;
  $('#tagNameInput').value = cur ? cur.name : '';
  state.tagEditor.color = cur ? cur.color : '#6366f1';
  $('#tagColorInput').value = state.tagEditor.color;
  $('#btnTagDelete').hidden = !id;
  renderPalette();
  openModal('tagEditorModal');
  setTimeout(() => $('#tagNameInput').focus(), 50);
}

function renderPalette() {
  const palette = ['#ef4444', '#f97316', '#f59e0b', '#eab308', '#84cc16', '#22c55e', '#10b981', '#14b8a6',
    '#06b6d4', '#0ea5e9', '#3b82f6', '#6366f1', '#8b5cf6', '#a855f7', '#d946ef', '#ec4899'];
  const box = $('#colorPalette');
  box.innerHTML = '';
  palette.forEach(c => {
    const b = document.createElement('button');
    b.className = 'swatch' + (c.toLowerCase() === state.tagEditor.color.toLowerCase() ? ' active' : '');
    b.style.background = c;
    b.onclick = () => { state.tagEditor.color = c; $('#tagColorInput').value = c; renderPalette(); };
    box.appendChild(b);
  });
}

async function saveTagEditor() {
  const name = $('#tagNameInput').value.trim();
  if (!name) { toast('请输入名称', 'err'); return; }
  const color = $('#tagColorInput').value || state.tagEditor.color;
  const isTag = state.tagEditor.kind === 'tag';
  try {
    if (state.tagEditor.id) {
      const res = await api('PUT', `/${isTag ? 'api/tags' : 'api/categories'}/${state.tagEditor.id}`, { name, color });
      applyTaxonomy(res);
    } else {
      const res = await api('POST', isTag ? '/api/tags' : '/api/categories', { name, color });
      applyTaxonomy(res);
    }
    toast('已保存', 'ok');
    closeModal('tagEditorModal');
    await loadSnippets();
  } catch (err) { toast(err.message, 'err'); }
}

async function deleteTagEditor() {
  const isTag = state.tagEditor.kind === 'tag';
  const id = state.tagEditor.id;
  if (!id) return;
  const ok = await confirmDialog(
    isTag ? '删除标签后，所有灵感上的该标签都会被移除。确定吗？' : '删除分类后，相关灵感会变为「未分类」。确定吗？',
    isTag ? '删除标签' : '删除分类');
  if (!ok) return;
  const res = await api('DELETE', `/${isTag ? 'api/tags' : 'api/categories'}/${id}`);
  applyTaxonomy(res);
  if ((isTag && state.view === 'tag' && state.viewId === id) || (!isTag && state.view === 'category' && state.viewId === id)) {
    state.view = 'all'; state.viewId = null;
  }
  closeModal('tagEditorModal');
  toast('已删除', 'ok');
  await loadSnippets();
}

/* ------------------------------------------------------------------ AI 打标 */
async function runAiSuggest(ids) {
  if (!state.settings.ai.enabled) {
    toast('请先在「设置 → AI」中启用并配置 AI API', 'err', 5000);
    return;
  }
  if (!ids.length) { toast('请先勾选要打标的灵感', 'warn'); return; }
  openModal('aiModal');
  $('#aiStatus').textContent = `正在调用 AI 分析 ${ids.length} 条灵感…`;
  $('#aiList').innerHTML = '<div class="hint">AI 正在阅读内容并生成建议，请稍候…</div>';
  $('#btnAiApply').disabled = true;
  try {
    const res = await api('POST', '/api/ai/suggest', { ids });
    state.aiItems = res.items || [];
    renderAiList();
    const okCount = state.aiItems.filter(i => !i.error).length;
    $('#aiStatus').textContent = `已生成 ${okCount} 条建议${okCount < ids.length ? '（部分失败）' : ''}`;
    $('#btnAiApply').disabled = okCount === 0;
  } catch (err) {
    $('#aiStatus').textContent = '';
    $('#aiList').innerHTML = `<div class="ai-err">${escapeHtml(err.message)}</div>`;
  }
}

function renderAiList() {
  const box = $('#aiList');
  if (!state.aiItems.length) { box.innerHTML = '<div class="hint">没有可用的建议。</div>'; return; }
  box.innerHTML = state.aiItems.map((item, idx) => {
    if (item.error) {
      return `<div class="ai-item"><div class="ai-item-head"><span class="t">${escapeHtml(item.title || item.id)}</span></div>
        <div class="ai-err">AI 分析失败：${escapeHtml(item.error)}</div></div>`;
    }
    const tagChips = (item.tags || []).map(t => {
      const exists = state.tags.some(x => x.name.toLowerCase() === t.toLowerCase());
      return `<label class="pick-chip" style="--c:${exists ? escapeHtml((state.tags.find(x => x.name.toLowerCase() === t.toLowerCase()) || {}).color || '#64748b') : '#64748b'}">
        <input type="checkbox" class="ai-tag" data-idx="${idx}" value="${escapeHtml(t)}" checked>${escapeHtml(t)}${exists ? '' : ' <span style="opacity:.7">新</span>'}</label>`;
    }).join('');
    const catExists = item.category && state.categories.some(c => c.name.toLowerCase() === item.category.toLowerCase());
    return `<div class="ai-item">
      <div class="ai-item-head">
        <input type="checkbox" class="ai-item-on" data-idx="${idx}" checked>
        <span class="t">${escapeHtml(item.title || '未命名')}</span>
        ${item.summary ? `<span class="src-badge">${escapeHtml(item.summary)}</span>` : ''}
      </div>
      <div class="ai-suggest-row">
        <span class="hint">标签：</span>${tagChips || '<span class="hint">无</span>'}
      </div>
      <div class="ai-suggest-row">
        <span class="hint">分类：</span>
        ${item.category
          ? `<label class="pick-chip" style="--c:${catExists ? escapeHtml((state.categories.find(c => c.name.toLowerCase() === item.category.toLowerCase()) || {}).color || '#64748b') : '#64748b'}">
               <input type="checkbox" class="ai-cat" data-idx="${idx}" value="${escapeHtml(item.category)}" checked>${escapeHtml(item.category)}${catExists ? '' : ' <span style="opacity:.7">新</span>'}</label>`
          : '<span class="hint">无</span>'}
      </div>
    </div>`;
  }).join('');
}

async function applyAiSuggestions() {
  const items = [];
  $$('#aiList .ai-item').forEach((el) => {
    const on = $('.ai-item-on', el);
    if (!on) return;               // 失败项
    const idx = parseInt(on.dataset.idx, 10);
    if (!on.checked) return;
    const item = state.aiItems[idx];
    if (!item) return;
    const tags = $$('.ai-tag', el).filter(c => c.checked).map(c => c.value);
    const catEl = $('.ai-cat', el);
    items.push({
      id: item.id,
      tags,
      category: catEl && catEl.checked ? catEl.value : '',
      mode: $('#aiMode').value,
    });
  });
  if (!items.length) { toast('没有勾选任何建议', 'warn'); return; }
  $('#btnAiApply').disabled = true;
  $('#aiStatus').textContent = '正在应用…';
  try {
    const res = await api('POST', '/api/ai/apply', { items });
    state.tags = res.tags || state.tags;
    state.categories = res.categories || state.categories;
    state.stats = res.stats || state.stats;
    toast(`已为 ${res.applied} 条灵感添加标签/分类（新增标签 ${res.tagsCreated}、分类 ${res.catsCreated}）`, 'ok', 4500);
    closeModal('aiModal');
    renderSidebar(); fillDatalist();
    await loadSnippets();
  } catch (err) {
    toast(err.message, 'err', 5000);
  } finally {
    $('#btnAiApply').disabled = false;
  }
}

/* ------------------------------------------------------------------ 流水线 */
function openPipeline(ids) {
  if (!ids.length) { toast('请先勾选要处理的灵感', 'warn'); return; }
  // 按当前列表顺序排列
  const order = state.snippets.map(s => s.id);
  const sorted = ids.slice().sort((a, b) => order.indexOf(a) - order.indexOf(b));
  state.pipeline.items = sorted.map(id => state.snippets.find(s => s.id === id) || { id });
  state.pipeline.index = 0;
  state.pipeline.drafts = {};
  state.pipeline.items.forEach(it => { state.pipeline.drafts[it.id] = { categoryId: it.categoryId || '', tags: new Set(it.tags || []) }; });
  openModal('pipelineModal');
  renderPipeline();
}

function fillPipelineCategorySelect(selected) {
  const sel = $('#pipeCategory');
  sel.innerHTML = '<option value="">未分类</option>' +
    state.categories.map(c => `<option value="${c.id}">${escapeHtml(c.name)}</option>`).join('');
  sel.value = selected || '';
}

function renderPipeline() {
  const p = state.pipeline;
  if (!p.items.length || p.index >= p.items.length) { finishPipeline(); return; }
  const item = p.items[p.index];
  const draft = p.drafts[item.id];
  $('#pipeProgress').textContent = `${p.index + 1} / ${p.items.length}`;
  $('#pipeTitle').textContent = item.title || '未命名灵感';
  $('#pipeContent').innerHTML = mdToHtml(item.content || '');
  fillPipelineCategorySelect(draft.categoryId);

  const box = $('#pipeTagList');
  box.innerHTML = '';
  state.tags.forEach(t => {
    const on = draft.tags.has(t.id);
    const chip = document.createElement('span');
    chip.className = 'pick-chip';
    chip.style.setProperty('--c', t.color);
    chip.style.opacity = on ? '1' : '.55';
    chip.innerHTML = `${on ? '✓' : '＋'} ${escapeHtml(t.name)}`;
    chip.onclick = () => {
      if (draft.tags.has(t.id)) draft.tags.delete(t.id); else draft.tags.add(t.id);
      renderPipeline();
    };
    box.appendChild(chip);
  });
  $('#pipeTagCount').textContent = draft.tags.size;
  $('#btnPipeSaveNext').innerHTML = (p.index === p.items.length - 1 ? '保存并完成 ' : '保存并进入下一条 ') +
    `<kbd data-sc="pipelineNext">${escapeHtml((state.settings.shortcuts || DEFAULT_SHORTCUTS).pipelineNext || '')}</kbd>`;
}

async function pipelineSaveNext() {
  const p = state.pipeline;
  const item = p.items[p.index];
  if (!item) { finishPipeline(); return; }
  const draft = p.drafts[item.id];
  try {
    const res = await api('PATCH', '/api/snippets/' + item.id, {
      categoryId: draft.categoryId,
      tags: Array.from(draft.tags),
    });
    const idx = state.snippets.findIndex(s => s.id === item.id);
    if (idx >= 0) state.snippets[idx] = res.snippet;
    state.tags = res.tags || state.tags;
    state.stats = res.stats || state.stats;
  } catch (err) {
    toast('保存失败：' + err.message, 'err');
    return;
  }
  p.index++;
  if (p.index >= p.items.length) { finishPipeline(); return; }
  renderPipeline();
}

function finishPipeline() {
  closeModal('pipelineModal');
  toast('流水线已完成 🎉', 'ok');
  renderSidebar(); fillDatalist();
  loadSnippets();
}

/* ------------------------------------------------------------------ WebDAV / 数据 */
async function webdavTest() {
  const cfg = collectSettings().webdav;
  $('#davStatus').textContent = '正在测试连接…';
  $('#davStatus').className = 'test-result block';
  try {
    const res = await api('POST', '/api/webdav/test', cfg);
    $('#davStatus').textContent = res.message;
    $('#davStatus').className = 'test-result block ' + (res.ok ? 'ok' : 'err');
  } catch (err) {
    $('#davStatus').textContent = err.message;
    $('#davStatus').className = 'test-result block err';
  }
}

async function webdavBackupNow() {
  const payload = collectSettings();
  const saved = await api('PUT', '/api/settings', payload);
  state.settings = saved.settings;
  applySettingsToUI();
  $('#davStatus').textContent = '正在上传备份…';
  $('#davStatus').className = 'test-result block';
  try {
    const res = await api('POST', '/api/webdav/backup');
    $('#davStatus').textContent = `备份成功：${res.info.file}（${res.info.bytes} 字节）`;
    $('#davStatus').className = 'test-result block ok';
    $('#davStatus').textContent += res.info.pruned ? `，清理旧备份 ${res.info.pruned} 份` : '';
    toast('WebDAV 备份完成', 'ok');
  } catch (err) {
    $('#davStatus').textContent = '备份失败：' + err.message;
    $('#davStatus').className = 'test-result block err';
    toast('备份失败：' + err.message, 'err', 5000);
  }
}

async function webdavListRemote() {
  const box = $('#davFiles');
  box.hidden = false;
  box.innerHTML = '<div class="remote-item"><span class="name">正在读取远端目录…</span></div>';
  try {
    const res = await api('GET', '/api/webdav/list');
    const items = res.items || [];
    if (!items.length) { box.innerHTML = '<div class="remote-item"><span class="name">远端目录暂无备份文件</span></div>'; return; }
    box.innerHTML = items.filter(f => !f.isDir && /\.json$/i.test(f.name)).map(f => `
      <div class="remote-item">
        <span class="name" title="${escapeHtml(f.name)}">${escapeHtml(f.name)}</span>
        <span class="meta">${f.size ? Math.round(f.size / 1024) + ' KB · ' : ''}${escapeHtml(fmtFull(f.modTime) || '')}</span>
        <button class="btn tiny" data-restore="${escapeHtml(f.name)}" data-mode="merge">合并恢复</button>
        <button class="btn tiny danger" data-restore="${escapeHtml(f.name)}" data-mode="replace">覆盖恢复</button>
      </div>`).join('');
    $$('#davFiles [data-restore]').forEach(btn => {
      btn.onclick = () => restoreFromRemote(btn.dataset.restore, btn.dataset.mode);
    });
  } catch (err) {
    box.innerHTML = `<div class="remote-item"><span class="name err">读取失败：${escapeHtml(err.message)}</span></div>`;
  }
}

async function restoreFromRemote(name, mode) {
  const ok = await confirmDialog(
    `将从 WebDAV 恢复「${name}」，方式：${mode === 'replace' ? '覆盖（清空现有数据）' : '合并'}。恢复前会自动创建本地快照。继续吗？`,
    '从 WebDAV 恢复');
  if (!ok) return;
  toast('正在恢复…', 'info');
  try {
    const res = await api('POST', '/api/webdav/restore', { name, mode });
    toast(`恢复完成：灵感 +${res.result.snippets}，标签 +${res.result.tags}，分类 +${res.result.categories}`, 'ok', 5000);
    await bootstrap();
    await loadSnippets();
    fillSettingsForm();
  } catch (err) {
    toast('恢复失败：' + err.message, 'err', 6000);
  }
}

async function localSnapshot() {
  try {
    const res = await api('POST', '/api/backup/local');
    toast('已创建本地快照：' + res.path, 'ok', 5000);
  } catch (err) { toast(err.message, 'err'); }
}

async function importBackup() {
  const file = $('#importFile').files[0];
  if (!file) { toast('请先选择备份文件', 'warn'); return; }
  const mode = $('#importMode').value;
  if (mode === 'replace') {
    const ok = await confirmDialog('覆盖导入会清空当前全部灵感、标签与分类，确定继续吗？', '覆盖导入');
    if (!ok) return;
  }
  try {
    const text = await file.text();
    const data = JSON.parse(text);
    const res = await api('POST', '/api/backup/import', { mode, data });
    toast(`导入完成：灵感 +${res.result.snippets}，标签 +${res.result.tags}，分类 +${res.result.categories}`, 'ok', 5000);
    await bootstrap();
    await loadSnippets();
    fillSettingsForm();
  } catch (err) {
    toast('导入失败：' + err.message, 'err', 6000);
  }
}

/* ------------------------------------------------------------------ 事件绑定 */
function bindEvents() {
  // 侧栏
  $('#btnNewSnippet').onclick = () => openEditor(null);
  $$('#mainNav .nav-item').forEach(btn => {
    btn.onclick = () => {
      state.view = btn.dataset.view;
      state.viewId = null;
      state.selection.clear();
      loadSnippets(); renderSidebar();
    };
  });
  $('#btnNewCategory').onclick = () => openTagEditor('category', null);
  $('#btnNewTag').onclick = () => openTagEditor('tag', null);
  $('#btnOpenSettings').onclick = openSettings;

  // 搜索
  const onSearch = debounce(() => {
    state.query = $('#searchInput').value.trim();
    state.mode = $('#searchMode').value;
    loadSnippets();
  }, 220);
  $('#searchInput').addEventListener('input', onSearch);
  $('#searchMode').addEventListener('change', () => {
    state.mode = $('#searchMode').value;
    state.query = $('#searchInput').value.trim();
    loadSnippets();
  });
  $('#btnClearSearch').onclick = () => {
    $('#searchInput').value = '';
    state.query = '';
    loadSnippets();
  };
  $('#sortSelect').onchange = () => { state.sort = $('#sortSelect').value; loadSnippets(); };
  $('#checkAllVisible').onchange = (e) => toggleSelectAll(e.target.checked);

  // 批量操作
  $('#btnBulkApplyCategory').onclick = async () => {
    const catId = $('#bulkCategory').value;
    if (!catId) { toast('请选择分类', 'warn'); return; }
    const res = await api('POST', '/api/snippets/bulk', { ids: Array.from(state.selection), action: 'assign', categoryId: catId });
    applyTaxonomy(res);
    toast(`已为 ${res.affected} 条灵感设置分类`, 'ok');
    await loadSnippets();
  };
  $('#btnBulkAddTag').onclick = async () => {
    const name = $('#bulkTagInput').value.trim();
    if (!name) { toast('请输入标签名', 'warn'); return; }
    const t = await addTagByName(name);
    if (!t) return;
    const res = await api('POST', '/api/snippets/bulk', { ids: Array.from(state.selection), action: 'assign', addTags: [t.name] });
    applyTaxonomy(res);
    $('#bulkTagInput').value = '';
    toast(`已为 ${res.affected} 条灵感添加标签「${t.name}」`, 'ok');
    await loadSnippets();
  };
  $('#btnBulkArchive').onclick = async () => {
    const res = await api('POST', '/api/snippets/bulk', { ids: Array.from(state.selection), action: 'archive' });
    applyTaxonomy(res); toast(`已归档 ${res.affected} 条`, 'ok'); await loadSnippets();
  };
  $('#btnBulkUnarchive').onclick = async () => {
    const res = await api('POST', '/api/snippets/bulk', { ids: Array.from(state.selection), action: 'unarchive' });
    applyTaxonomy(res); toast(`已取消归档 ${res.affected} 条`, 'ok'); await loadSnippets();
  };
  $('#btnBulkDelete').onclick = () => deleteSnippets(Array.from(state.selection));
  $('#btnClearSelection').onclick = () => toggleSelectAll(false);

  // AI 打标 / 流水线
  const selectedIds = () => Array.from(state.selection);
  $('#btnAiTag').onclick = () => runAiSuggest(selectedIds());
  $('#btnAiTagSel').onclick = () => runAiSuggest(selectedIds());
  $('#btnPipeline').onclick = () => openPipeline(selectedIds());
  $('#btnPipelineSel').onclick = () => openPipeline(selectedIds());

  // 编辑器
  $('#btnEdSave').onclick = saveEditor;
  $('#btnEdCancel').onclick = () => closeEditor(false);
  $('#btnEdDelete').onclick = async () => {
    if (!state.editingId) return;
    const id = state.editingId;
    if (state.settings.ui.confirmDelete) {
      const ok = await confirmDialog('确定删除这条灵感吗？', '删除灵感');
      if (!ok) return;
    }
    await api('DELETE', '/api/snippets/' + id);
    state.editorDirty = false;
    closeModal('editorModal');
    state.editingId = null;
    toast('已删除', 'ok');
    await loadSnippets(); renderSidebar();
  };
  $('#btnEdAiTitle').onclick = async () => {
    const content = state.editor ? state.editor.value() : '';
    if (!content.trim()) { toast('先写点内容再让 AI 起标题', 'warn'); return; }
    if (!state.settings.ai.enabled) { toast('请先在设置中启用 AI', 'err'); return; }
    $('#btnEdAiTitle').disabled = true;
    $('#btnEdAiTitle').textContent = '✨ 生成中…';
    try {
      const res = await api('POST', '/api/ai/title', { content });
      $('#edTitle').value = res.title;
      state.editorDirty = true;
      toast('AI 标题：' + res.title, 'ok');
    } catch (err) { toast(err.message, 'err', 5000); }
    finally { $('#btnEdAiTitle').disabled = false; $('#btnEdAiTitle').textContent = '✨ AI 生成标题'; }
  };
  $('#edTagInput').addEventListener('keydown', async (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const t = await addTagByName(e.target.value);
    if (t) { state.editorTags.add(t.id); state.editorDirty = true; e.target.value = ''; renderEditorTags(); updateEditorMeta(); }
  });
  $('#edCategory').addEventListener('change', updateEditorMeta);

  // 设置
  $('#btnSaveSettings').onclick = () => saveSettings().catch(err => toast(err.message, 'err'));
  $('#btnAiTest').onclick = async () => {
    const cfg = collectSettings().ai;
    $('#aiTestResult').textContent = '测试中…';
    $('#aiTestResult').className = 'test-result';
    try {
      const res = await api('POST', '/api/ai/test', cfg);
      $('#aiTestResult').textContent = res.message;
      $('#aiTestResult').className = 'test-result ' + (res.ok ? 'ok' : 'err');
    } catch (err) {
      $('#aiTestResult').textContent = err.message;
      $('#aiTestResult').className = 'test-result err';
    }
  };
  $('#aiPreset').onchange = () => {
    const v = $('#aiPreset').value;
    if (!v) return;
    const [url, model] = v.split('|');
    $('#aiBaseUrl').value = url;
    $('#aiModel').value = model;
    $('#aiEnabled').checked = true;
  };
  $('#btnDavTest').onclick = webdavTest;
  $('#btnDavBackup').onclick = webdavBackupNow;
  $('#btnDavRefresh').onclick = webdavListRemote;
  $('#btnDavLocalSnapshot').onclick = localSnapshot;
  $('#btnSnapshot2').onclick = localSnapshot;
  $('#btnExport').onclick = () => { window.location.href = '/api/backup/export'; };
  $('#btnImport').onclick = importBackup;
  $('#btnShortcutReset').onclick = () => {
    state.settings.shortcuts = Object.assign({}, DEFAULT_SHORTCUTS);
    renderShortcutEditor();
    toast('已恢复默认快捷键（记得点保存设置）', 'ok');
  };

  // 设置页签
  $$('.tabs .tab').forEach(tab => {
    tab.onclick = () => {
      $$('.tabs .tab').forEach(t => t.classList.toggle('active', t === tab));
      $$('.tab-panel').forEach(p => p.classList.toggle('active', p.dataset.panel === tab.dataset.tab));
    };
  });

  // 通用关闭
  $$('[data-close-modal]').forEach(btn => {
    btn.onclick = () => {
      const modal = btn.closest('.modal-backdrop');
      if (!modal) return;
      if (modal.id === 'editorModal') { closeEditor(false); return; }
      if (modal.id === 'pipelineModal') { finishPipeline(); return; }
      modal.hidden = true;
    };
  });
  $$('.modal-backdrop').forEach(backdrop => {
    backdrop.addEventListener('mousedown', (e) => {
      if (e.target !== backdrop) return;
      if (backdrop.id === 'editorModal') { closeEditor(false); return; }
      if (backdrop.id === 'pipelineModal') { finishPipeline(); return; }
      if (backdrop.id === 'confirmModal') return;
      backdrop.hidden = true;
    });
  });

  // AI 弹窗
  $('#btnAiApply').onclick = applyAiSuggestions;
  $('#btnAiCheckAll').onclick = () => {
    $$('#aiList .ai-item-on').forEach(c => { c.checked = true; });
    $$('#aiList .ai-tag, #aiList .ai-cat').forEach(c => { c.checked = true; });
  };
  $('#btnAiUncheckAll').onclick = () => {
    $$('#aiList .ai-item-on, #aiList .ai-tag, #aiList .ai-cat').forEach(c => { c.checked = false; });
  };

  // 流水线
  $('#btnPipeSaveNext').onclick = pipelineSaveNext;
  $('#btnPipeSkip').onclick = () => { state.pipeline.index++; renderPipeline(); };
  $('#btnPipePrev').onclick = () => { if (state.pipeline.index > 0) { state.pipeline.index--; renderPipeline(); } };
  $('#btnPipeStop').onclick = finishPipeline;
  $('#pipeCategory').onchange = () => {
    const item = state.pipeline.items[state.pipeline.index];
    if (!item) return;
    state.pipeline.drafts[item.id].categoryId = $('#pipeCategory').value;
  };
  $('#pipeTagInput').addEventListener('keydown', async (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const t = await addTagByName(e.target.value);
    if (!t) return;
    const item = state.pipeline.items[state.pipeline.index];
    state.pipeline.drafts[item.id].tags.add(t.id);
    e.target.value = '';
    renderPipeline();
  });

  // 标签编辑
  $('#btnTagSave').onclick = saveTagEditor;
  $('#btnTagDelete').onclick = deleteTagEditor;
  $('#tagColorInput').oninput = () => { state.tagEditor.color = $('#tagColorInput').value; renderPalette(); };
  $('#tagNameInput').addEventListener('keydown', (e) => { if (e.key === 'Enter') saveTagEditor(); });

  // 快捷键
  document.addEventListener('keydown', handleKeydown);

  window.addEventListener('beforeunload', (e) => {
    if (state.editorDirty && !$('#editorModal').hidden) { e.preventDefault(); e.returnValue = ''; }
  });
}

/* ------------------------------------------------------------------ 启动 */
async function init() {
  bindEvents();
  renderPalette();
  $('#sortSelect').value = state.sort;
  $('#searchMode').value = state.mode;
  try {
    await bootstrap();
  } catch (err) {
    toast('初始化失败：' + err.message, 'err', 8000);
    return;
  }
  // 批量分类下拉
  const bulkCat = $('#bulkCategory');
  bulkCat.innerHTML = '<option value="">— 设置分类 —</option><option value="__none__">（未分类）</option>' +
    state.categories.map(c => `<option value="${c.id}">${escapeHtml(c.name)}</option>`).join('');
  renderSidebar();
  await loadSnippets();
}

// 分类列表变化后同步批量下拉

init();
