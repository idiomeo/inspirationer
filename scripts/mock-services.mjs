/**
 * 本地模拟服务：OpenAI 兼容接口 + WebDAV 服务端
 * 仅用于端到端验证灵感管理器的 AI 与 WebDAV 功能，不参与生产运行。
 *
 *   node scripts/mock-services.mjs
 *
 *   - OpenAI 兼容:  http://127.0.0.1:9998/v1/chat/completions
 *   - WebDAV:       http://127.0.0.1:9999/dav/   (Basic: mock-user / mock-pass)
 */
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(__dirname, '..', 'logs', 'davroot');
fs.rmSync(ROOT, { recursive: true, force: true });
fs.mkdirSync(ROOT, { recursive: true });

/* ------------------------------------------------------------ 模拟 AI */
const AI_PORT = 9998;
const aiServer = http.createServer((req, res) => {
  let body = '';
  req.on('data', (c) => { body += c; });
  req.on('end', () => {
    if (req.method !== 'POST' || !req.url.includes('/chat/completions')) {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: { message: 'not found' } }));
      return;
    }
    if (!req.headers.authorization) {
      res.writeHead(401, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: { message: 'missing api key' } }));
      return;
    }
    let payload = {};
    try { payload = JSON.parse(body); } catch { }
    const system = (payload.messages?.[0]?.content || '');
    const user = (payload.messages?.[1]?.content || '');
    let content;
    if (system.includes('"tags"')) {
      // 结构化建议：故意带 markdown 代码块与大小写差异，验证解析健壮性
      const hasMarkdown = user.includes('Markdown');
      content = '```json\n' + JSON.stringify({
        title: 'AI 自动标题',
        tags: hasMarkdown ? ['Markdown', '写作', 'markdown'] : ['产品设计', '效率工具'],
        category: hasMarkdown ? '写作素材' : '产品设计',
        summary: '这是模拟 AI 生成的一句话摘要',
      }) + '\n```';
    } else {
      content = 'AI 总结的标题';
    }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      id: 'mock', object: 'chat.completion',
      model: payload.model || 'mock-model',
      choices: [{ index: 0, message: { role: 'assistant', content }, finish_reason: 'stop' }],
      usage: { prompt_tokens: 10, completion_tokens: 5, total_tokens: 15 },
    }));
  });
});

/* ------------------------------------------------------------ 模拟 WebDAV */
const DAV_PORT = 9999;
// 仅用于本地模拟服务的一次性假凭据，不指向任何真实账号
const USER = 'mock-user', PASS = 'mock-pass';
const MOUNT = '/dav';

function safePath(urlPath) {
  let p = decodeURIComponent(urlPath);
  if (!p.startsWith(MOUNT)) return null;
  p = p.slice(MOUNT.length).replace(/^\/+/, '');
  const abs = path.join(ROOT, p);
  if (!abs.startsWith(ROOT)) return null;
  return abs;
}
function unauthorized(res) {
  res.writeHead(401, { 'WWW-Authenticate': 'Basic realm="mock"' });
  res.end('unauthorized');
}
function authorized(req) {
  const h = req.headers.authorization || '';
  if (!h.startsWith('Basic ')) return false;
  const [u, p] = Buffer.from(h.slice(6), 'base64').toString('utf8').split(':');
  return u === USER && p === PASS;
}
function hrefFor(abs, isDir) {
  let rel = path.relative(ROOT, abs).split(path.sep).join('/');
  const encoded = rel.split('/').map(encodeURIComponent).join('/');
  return MOUNT + '/' + encoded + (isDir ? '/' : '');
}

const davServer = http.createServer((req, res) => {
  if (!authorized(req)) return unauthorized(res);
  const abs = safePath(req.url);
  if (!abs) { res.writeHead(403); res.end('forbidden'); return; }
  const method = req.method.toUpperCase();

  if (method === 'MKCOL') {
    if (fs.existsSync(abs)) { res.writeHead(405); res.end('exists'); return; }
    try { fs.mkdirSync(abs, { recursive: false }); res.writeHead(201); res.end(); }
    catch (e) { res.writeHead(409); res.end(String(e.message)); }
    return;
  }
  if (method === 'PUT') {
    const chunks = [];
    req.on('data', (c) => chunks.push(c));
    req.on('end', () => {
      try {
        fs.mkdirSync(path.dirname(abs), { recursive: true });
        fs.writeFileSync(abs, Buffer.concat(chunks));
        res.writeHead(201); res.end();
      } catch (e) { res.writeHead(500); res.end(String(e.message)); }
    });
    return;
  }
  if (method === 'GET') {
    if (!fs.existsSync(abs) || fs.statSync(abs).isDirectory()) { res.writeHead(404); res.end('not found'); return; }
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(fs.readFileSync(abs));
    return;
  }
  if (method === 'DELETE') {
    if (!fs.existsSync(abs)) { res.writeHead(404); res.end('not found'); return; }
    fs.rmSync(abs, { recursive: true, force: true });
    res.writeHead(204); res.end();
    return;
  }
  if (method === 'PROPFIND') {
    if (!fs.existsSync(abs)) { res.writeHead(404); res.end('not found'); return; }
    const st = fs.statSync(abs);
    const entries = st.isDirectory() ? [abs, ...fs.readdirSync(abs).map(n => path.join(abs, n))] : [abs];
    const parts = entries.map((e) => {
      const s = fs.statSync(e);
      const dir = s.isDirectory();
      return `<d:response>
  <d:href>${hrefFor(e, dir)}</d:href>
  <d:propstat>
    <d:prop>
      <d:displayname>${path.basename(e)}</d:displayname>
      <d:getcontentlength>${dir ? 0 : s.size}</d:getcontentlength>
      <d:getlastmodified>${s.mtime.toUTCString()}</d:getlastmodified>
      <d:resourcetype>${dir ? '<d:collection/>' : ''}</d:resourcetype>
    </d:prop>
    <d:status>HTTP/1.1 200 OK</d:status>
  </d:propstat>
</d:response>`;
    });
    const xml = `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
${parts.join('\n')}
</d:multistatus>`;
    res.writeHead(207, { 'Content-Type': 'application/xml; charset=utf-8' });
    res.end(xml);
    return;
  }
  res.writeHead(405); res.end('method not allowed');
});

aiServer.listen(AI_PORT, '127.0.0.1', () => console.log(`mock OpenAI  -> http://127.0.0.1:${AI_PORT}/v1`));
davServer.listen(DAV_PORT, '127.0.0.1', () => console.log(`mock WebDAV  -> http://127.0.0.1:${DAV_PORT}/dav/  (mock-user/mock-pass)`));
console.log('模拟服务已启动，按 Ctrl+C 退出。');
