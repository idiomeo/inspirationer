/**
 * 校验界面文案字典的完整性（可放进 CI）：
 *   1. en / zh-CN / ja 三份字典的 key 集合必须完全一致；
 *   2. index.html 与 app.js 里引用的 key 必须都存在于字典中；
 *   3. 报告字典里已无人引用的 key（只提示，不算失败）。
 *
 * 用法：node scripts/check-i18n.mjs
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(__dirname, '..');
const LANGS = ['en', 'zh-CN', 'ja'];

// ---- 载入字典（i18n.js 是给浏览器用的，这里补一个 window 再 require）
global.window = {};
const i18nPath = path.join(ROOT, 'web', 'i18n.js');
const dict = (() => {
  const src = fs.readFileSync(i18nPath, 'utf8');
  // eslint-disable-next-line no-new-func
  new Function('window', src)(global.window);
  return global.window.I18N_DICT;
})();

let fail = 0;
const say = (ok, msg) => {
  console.log(`${ok ? '✅' : '❌'} ${msg}`);
  if (!ok) fail++;
};

if (!dict) {
  console.error('❌ 无法从 web/i18n.js 载入 I18N_DICT');
  process.exit(1);
}

// ---- 1. 三份字典的 key 必须一致
const base = Object.keys(dict.en);
console.log(`字典基准（en）：${base.length} 个 key\n`);
for (const lang of LANGS.slice(1)) {
  const keys = Object.keys(dict[lang] || {});
  const missing = base.filter(k => !(k in (dict[lang] || {})));
  const extra = keys.filter(k => !base.includes(k));
  const empty = base.filter(k => !String(dict[lang][k] || '').trim());
  say(missing.length === 0, `${lang}：key 数量 ${keys.length}，缺失 ${missing.length}${missing.length ? ' → ' + missing.slice(0, 5).join(', ') : ''}`);
  say(extra.length === 0, `${lang}：多余 key ${extra.length}${extra.length ? ' → ' + extra.slice(0, 5).join(', ') : ''}`);
  say(empty.length === 0, `${lang}：空文案 ${empty.length}${empty.length ? ' → ' + empty.slice(0, 5).join(', ') : ''}`);
}

// ---- 2. 前端引用的 key 必须存在
const used = new Map(); // key -> Set(文件)
for (const file of ['web/index.html', 'web/app.js']) {
  const src = fs.readFileSync(path.join(ROOT, file), 'utf8');
  const add = (key) => {
    if (!used.has(key)) used.set(key, new Set());
    used.get(key).add(file);
  };
  for (const m of src.matchAll(/data-i18n(?:-html|-placeholder|-title)?="([A-Za-z.]+)"/g)) add(m[1]);
  for (const m of src.matchAll(/\bt\('([A-Za-z]+\.[A-Za-z]+)'/g)) add(m[1]);
  for (const m of src.matchAll(/(?:labelKey|descKey):\s*'([A-Za-z.]+)'/g)) add(m[1]);
}

const unknown = [...used.keys()].filter(k => !base.includes(k));
say(unknown.length === 0, `前端引用了 ${used.size} 个 key，其中字典里不存在 ${unknown.length}${unknown.length ? ' → ' + unknown.slice(0, 8).join(', ') : ''}`);

// 宽松判定“是否还有人用”：key 只要作为字符串字面量出现过就算（有些 key 是通过辅助函数传入的）
const rawAll = ['web/index.html', 'web/app.js']
  .map(f => fs.readFileSync(path.join(ROOT, f), 'utf8'))
  .join('\n');
const unused = base.filter(k => !rawAll.includes(`'${k}'`) && !rawAll.includes(`"${k}"`));
console.log(`\n提示：字典中已无人引用的 key：${unused.length} 个`);
if (unused.length) console.log('  ' + unused.join(', '));

console.log(fail === 0 ? '\n全部检查通过 ✅' : `\n有 ${fail} 项检查未通过 ❌`);
process.exit(fail === 0 ? 0 : 1);
