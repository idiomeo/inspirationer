/**
 * 截取界面截图（无头 Edge/Chrome + CDP），输出到 docs/。
 * 用法：node scripts/screenshot.mjs [baseUrl]
 */
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const BASE = process.argv[2] || 'http://127.0.0.1:8420';
// 可选第 3 个参数：文件名前缀（例如 ja- 用于截日语界面）
const PREFIX = process.argv[3] || '';
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const DOCS = path.join(__dirname, '..', 'docs');
const PORT = 9334;
const sleep = (ms) => new Promise(r => setTimeout(r, ms));

function findBrowser() {
  for (const c of [
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  ]) if (fs.existsSync(c)) return c;
  return null;
}

class CDP {
  constructor(ws) {
    this.ws = ws; this.id = 0; this.pending = new Map();
    ws.addEventListener('message', (ev) => {
      const m = JSON.parse(ev.data);
      if (m.id && this.pending.has(m.id)) {
        const { res, rej } = this.pending.get(m.id);
        this.pending.delete(m.id);
        m.error ? rej(new Error(JSON.stringify(m.error))) : res(m.result);
      }
    });
  }
  send(method, params = {}) {
    const id = ++this.id;
    return new Promise((res, rej) => {
      this.pending.set(id, { res, rej });
      this.ws.send(JSON.stringify({ id, method, params }));
      setTimeout(() => { if (this.pending.has(id)) { this.pending.delete(id); rej(new Error('CDP 超时 ' + method)); } }, 20000);
    });
  }
  async eval(expression) {
    const r = await this.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
    if (r.exceptionDetails) throw new Error((r.exceptionDetails.exception || {}).description || r.exceptionDetails.text);
    return r.result.value;
  }
  async waitFor(expr, timeout = 15000) {
    const t0 = Date.now();
    while (Date.now() - t0 < timeout) {
      try { if (await this.eval(`!!(${expr})`)) return; } catch { }
      await sleep(150);
    }
    throw new Error('等待超时: ' + expr);
  }
  async shot(name) {
    await sleep(450);
    const r = await this.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
    const file = path.join(DOCS, PREFIX + name + '.png');
    fs.writeFileSync(file, Buffer.from(r.data, 'base64'));
    console.log('已保存', file);
  }
}

async function main() {
  const browser = findBrowser();
  if (!browser) { console.error('未找到 Edge/Chrome'); process.exit(1); }
  fs.mkdirSync(DOCS, { recursive: true });
  const userDataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'inspirationer-shot-'));

  const proc = spawn(browser, [
    '--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check',
    '--hide-scrollbars', '--force-device-scale-factor=1.5',
    `--remote-debugging-port=${PORT}`, `--user-data-dir=${userDataDir}`,
    '--window-size=1560,980', BASE + '/',
  ], { stdio: 'ignore' });

  let target = null;
  for (let i = 0; i < 60; i++) {
    await sleep(300);
    try {
      const list = await (await fetch(`http://127.0.0.1:${PORT}/json/list`)).json();
      target = list.find(t => t.type === 'page' && t.url.startsWith(BASE));
      if (target && target.webSocketDebuggerUrl) break;
    } catch { }
  }
  if (!target) { proc.kill(); throw new Error('无法连接调试端口'); }

  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((res, rej) => { ws.addEventListener('open', res); ws.addEventListener('error', rej); });
  const cdp = new CDP(ws);
  await cdp.send('Runtime.enable');
  await cdp.send('Page.enable');
  await cdp.send('Emulation.setDeviceMetricsOverride', { width: 1560, height: 980, deviceScaleFactor: 1.5, mobile: false });

  try {
    await cdp.waitFor("document.readyState === 'complete' && document.querySelectorAll('#grid .card').length > 0");
    await cdp.eval("if (state.selection.size) { state.selection.clear(); renderBulkBar(); } true");
    await cdp.shot('01-overview');

    // 编辑器
    await cdp.eval("openEditor(state.snippets[0].id)");
    await cdp.waitFor("!document.querySelector('#editorModal').hidden");
    await cdp.shot('02-editor');
    await cdp.eval("closeEditor(true)");
    await cdp.waitFor("document.querySelector('#editorModal').hidden");

    // 流水线
    await cdp.eval("state.selection.clear(); state.snippets.slice(0,3).forEach(s => state.selection.add(s.id)); renderBulkBar(); document.querySelector('#btnPipelineSel').click(); true");
    await cdp.waitFor("!document.querySelector('#pipelineModal').hidden");
    await cdp.shot('03-pipeline');
    await cdp.eval("finishPipeline()");

    // 设置
    await cdp.eval("openSettings()");
    await cdp.waitFor("!document.querySelector('#settingsModal').hidden");
    await cdp.eval("document.querySelector('.tabs .tab[data-tab=webdav]').click()");
    await cdp.shot('04-settings');
    await cdp.eval("document.querySelector('.tabs .tab[data-tab=ai]').click()");
    await cdp.shot('05-settings-ai');
    await cdp.eval("document.querySelector('.tabs .tab[data-tab=shortcuts]').click()");
    await cdp.shot('06-settings-shortcuts');
  } finally {
    try { ws.close(); } catch { }
    try { proc.kill(); } catch { }
    await sleep(300);
    spawnSync('taskkill', ['/F', '/T', '/PID', String(proc.pid)], { stdio: 'ignore' });
    try { fs.rmSync(userDataDir, { recursive: true, force: true }); } catch { }
  }
  console.log('截图完成 →', DOCS);
}

main().catch(err => { console.error('截图失败：', err.message); process.exit(1); });
