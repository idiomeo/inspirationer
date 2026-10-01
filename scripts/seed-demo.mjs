/**
 * 清空数据并写入一套演示数据（用于首次体验 / 截图）。
 * 用法：node scripts/seed-demo.mjs [baseUrl]
 */
const BASE = process.argv[2] || 'http://127.0.0.1:8420';

async function api(method, p, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) { opts.headers['Content-Type'] = 'application/json; charset=utf-8'; opts.body = JSON.stringify(body); }
  const res = await fetch(BASE + p, opts);
  const text = await res.text();
  if (!res.ok) throw new Error(`${method} ${p} -> ${res.status} ${text}`);
  return text ? JSON.parse(text) : null;
}

const DEFAULT_SHORTCUTS = {
  newSnippet: 'Alt+N', saveSnippet: 'Alt+S', focusSearch: 'Alt+K', selectAll: 'Alt+A',
  pipelineNext: 'Alt+J', togglePreview: 'Alt+P', openSettings: 'Alt+O', closeModal: 'Escape',
};

async function main() {
  console.log('清空现有数据…');
  const snips = await api('GET', '/api/snippets?archive=all');
  if (snips.items.length) await api('POST', '/api/snippets/bulk', { ids: snips.items.map(s => s.id), action: 'delete' });
  for (const t of (await api('GET', '/api/tags')).items) await api('DELETE', '/api/tags/' + t.id);
  for (const c of (await api('GET', '/api/categories')).items) await api('DELETE', '/api/categories/' + c.id);

  console.log('重置设置为默认值…');
  const boot = await api('GET', '/api/bootstrap');
  const s = boot.settings;
  s.ai = { enabled: false, baseUrl: 'https://api.openai.com/v1', apiKey: '', model: 'gpt-4o-mini', temperature: 0.3, maxTokens: 512, timeoutSeconds: 45 };
  s.webdav = { enabled: false, url: '', username: '', password: '', remoteDir: 'inspirationer', intervalMinutes: 60, keepRemote: 10, lastBackup: '0001-01-01T00:00:00Z', lastStatus: '' };
  s.shortcuts = Object.assign({}, DEFAULT_SHORTCUTS);
  s.ui = { theme: 'dark', cardPreview: true, titleMaxRunes: 10, confirmDelete: true, autoBackupHint: false };
  await api('PUT', '/api/settings', s);

  console.log('创建分类与标签…');
  const cat = {};
  for (const [name, color] of [['产品想法', '#6366f1'], ['技术方案', '#0ea5e9'], ['阅读笔记', '#f59e0b']]) {
    cat[name] = (await api('POST', '/api/categories', { name, color })).category.id;
  }
  const tag = {};
  for (const [name, color] of [['灵感碎片', '#8b5cf6'], ['Markdown', '#22c55e'], ['AI', '#ec4899'], ['效率工具', '#14b8a6'], ['副业', '#f97316'], ['前端', '#3b82f6']]) {
    tag[name] = (await api('POST', '/api/tags', { name, color })).tag.id;
  }

  console.log('写入演示灵感…');
  const demo = [
    {
      title: '用 Go 做本地灵感管理器的架构设想',
      categoryId: cat['技术方案'],
      tags: [tag['灵感碎片'], tag['效率工具']],
      pinned: true,
      content: `# 目标

做一个**完全本地运行**、打开就能写的灵感收集器，不要登录、不要云、不要广告。

## 核心取舍

| 维度 | 选择 | 理由 |
| --- | --- | --- |
| 后端 | Go + 标准库 | 单文件可执行，双击即用 |
| 存储 | JSON 文件 + 原子写 | 可以直接用网盘同步，坏了也能手改 |
| 前端 | 原生 JS + 内嵌资源 | 不需要构建步骤，离线可用 |

## 待办

- [x] 快捷键流水线：\`Alt+N\` 新建 → \`Alt+S\` 保存
- [x] WebDAV 定时备份
- [ ] 接入本地 embedding，做语义检索

> 关键判断：**数据要属于用户自己**，所以任何设计都不能把数据锁在服务端。`,
    },
    {
      title: '',
      categoryId: cat['产品想法'],
      tags: [tag['灵感碎片']],
      content: `灵感：给每一条灵感加一个「时间胶囊」概念——写的时候可以设定未来某个时间才提醒自己回看，避免过早否定一个还不成熟的想法。也许可以结合每周复盘一起做。`,
    },
    {
      title: '每周复盘模板（周五 30 分钟）',
      categoryId: cat['产品想法'],
      tags: [tag['效率工具']],
      pinned: false,
      content: `## 1. 清空收集箱
把所有灵感过一遍，打上标签与分类，删掉确实没价值的。

## 2. 三问

1. 这周最有价值的产出是什么？
2. 哪件事其实可以不做？
3. 下周最重要的一件事是什么？

## 3. 输出
- 一条周报
- 一个下周的关键结果

\`\`\`text
原则：宁可少做，也要做完。
\`\`\``,
    },
    {
      title: '《深度工作》读书笔记',
      categoryId: cat['阅读笔记'],
      tags: [tag['Markdown'], tag['效率工具']],
      content: `# 《深度工作》· Cal Newport

## 一句话

**深度工作 = 在无干扰状态下专注于高认知需求的任务，它能创造新价值、提升技能且难以复制。**

## 可执行的做法

- 固定时段：每天上午 9:00–11:00 是神圣不可侵犯的深度时段
- 仪式感：同一张桌子、同一杯咖啡、关掉所有通知
- 量化：记录每天的深度工作小时数
- 拥抱无聊：排队时不刷手机，让大脑习惯低刺激

> 注意力残留（attention residue）：切换任务后，前一个任务会持续占用认知资源。`,
    },
    {
      title: 'AI 自动打标的产品思路',
      categoryId: cat['产品想法'],
      tags: [tag['AI'], tag['灵感碎片']],
      content: `# 为什么是「打标」而不是「自动整理」

整理是**破坏性**操作（会移动/合并内容），打标是**增量**操作（只加元数据），所以打标失败的成本极低。

## 交互设计上的两个要点

1. **永远保留人类否决权**：AI 给建议，人点确认，不静默修改数据。
2. **复用优先**：先看已有标签库，能复用就不新建，否则标签会爆炸。

## 流水线模式

比起一次处理 20 条，逐条「看 → 打标 → 下一条」的心流体验更好，适合手动精修。`,
    },
    {
      title: '网页快捷键设计备忘（避开浏览器冲突）',
      categoryId: cat['技术方案'],
      tags: [tag['前端'], tag['效率工具']],
      content: `## 浏览器已占用的组合键（不要用）

| 平台 | 已被占用 |
| --- | --- |
| Chrome/Edge | Ctrl+N/T/W/Shift+N/D/F/P/S/J/H/L，F5，Alt+F/E/D/Home/←/→ |
| Firefox | 同上，另有 Alt+F 打开菜单栏 |

## 结论

用 \`Alt + 字母\`（避开 E/F/D/H）最稳妥：

- \`Alt+N\` 新建
- \`Alt+S\` 保存
- \`Alt+K\` 搜索
- \`Alt+J\` 下一条
- \`Alt+A\` 全选
- \`Escape\` 关闭

并且要允许用户自己录制修改，因为不同输入法/键盘布局下冲突情况不一样。`,
    },
  ];

  for (const item of demo) {
    const payload = { title: item.title, content: item.content, tags: item.tags, categoryId: item.categoryId, pinned: !!item.pinned };
    const res = await api('POST', '/api/snippets', payload);
    if (!item.title) console.log(`  无标题灵感 → 自动标题：「${res.snippet.title}」`);
  }

  const stats = await api('GET', '/api/stats');
  console.log('\n完成：', JSON.stringify(stats));
}

main().catch(err => { console.error('失败：', err.message); process.exit(1); });
