/**
 * Analyze specific pages with issues
 */
const WebSocket = require('ws');
const http = require('http');

const CDP_URL = 'http://127.0.0.1:9222';
const BASE_URL = 'http://localhost:8080';

async function getDebuggerUrl() {
  return new Promise((resolve, reject) => {
    http.get(`${CDP_URL}/json/list`, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        const pages = JSON.parse(data);
        const page = pages.find(p => p.type === 'page');
        if (page) resolve(page.webSocketDebuggerUrl);
        else reject(new Error('No page'));
      });
    }).on('error', reject);
  });
}

function createCDP(ws) {
  let id = 0;
  const pending = new Map();
  ws.on('message', (data) => {
    const msg = JSON.parse(data.toString());
    if (msg.id !== undefined && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      if (msg.error) reject(new Error(msg.error.message));
      else resolve(msg.result);
    }
  });
  return {
    send: (method, params = {}) => {
      const reqId = ++id;
      return new Promise((resolve, reject) => {
        pending.set(reqId, { resolve, reject });
        ws.send(JSON.stringify({ id: reqId, method, params }));
        setTimeout(() => {
          if (pending.has(reqId)) {
            pending.delete(reqId);
            reject(new Error(`Timeout: ${method}`));
          }
        }, 15000);
      });
    }
  };
}

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

async function evaluate(cdp, expr) {
  const result = await cdp.send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true });
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.text);
  return result.result.value;
}

async function analyzePage(cdp, path, name) {
  console.log(`\n${'='.repeat(60)}`);
  console.log(`  Analyzing: ${name} (${path})`);
  console.log('='.repeat(60));

  await cdp.send('Page.navigate', { url: `${BASE_URL}${path}` });
  await sleep(3000);

  const url = await evaluate(cdp, 'window.location.href');
  console.log(`URL: ${url}`);

  // Get detailed page structure
  const analysis = await evaluate(cdp, `
    (function() {
      const main = document.querySelector('main');
      const mainHtml = main ? main.innerHTML.substring(0, 1500) : 'NO MAIN ELEMENT';
      const mainText = main ? main.innerText : 'NO MAIN ELEMENT';

      // Check for specific patterns
      const hasEmptyState = !!document.querySelector('[class*="empty"], [class*="Empty"]');
      const hasTable = !!document.querySelector('table');
      const hasCards = document.querySelectorAll('[class*="card" i]').length;
      const hasLoading = !!document.querySelector('[class*="loading"], [class*="spinner"], [class*="animate-spin"]');
      const hasError = !!document.querySelector('[class*="error" i], [role="alert"]');

      // Get all visible text content
      const bodyText = document.body.innerText;

      // Check for error messages
      const errors = Array.from(document.querySelectorAll('[class*="error" i], [role="alert"]'))
        .filter(el => el.offsetParent !== null)
        .map(el => el.textContent.trim());

      // Check page title/header
      const pageHeader = document.querySelector('h1, [class*="title"], [class*="header"]')?.textContent.trim();

      return {
        mainHtml,
        mainText,
        hasEmptyState,
        hasTable,
        hasCards,
        hasLoading,
        hasError,
        errors,
        pageHeader,
        bodyTextLength: bodyText.length
      };
    })()
  `);

  console.log(`\nPage header: ${analysis.pageHeader || 'N/A'}`);
  console.log(`Main element text length: ${analysis.mainText.length} chars`);
  console.log(`Body text length: ${analysis.bodyTextLength} chars`);
  console.log(`Has empty state: ${analysis.hasEmptyState}`);
  console.log(`Has table: ${analysis.hasTable}`);
  console.log(`Has cards: ${analysis.hasCards}`);
  console.log(`Has loading: ${analysis.hasLoading}`);
  console.log(`Has error: ${analysis.hasError}`);

  if (analysis.errors.length > 0) {
    console.log(`\nErrors found: ${analysis.errors.join('; ')}`);
  }

  console.log(`\n--- Main Content (first 500 chars) ---`);
  console.log(analysis.mainText.substring(0, 500));

  console.log(`\n--- Main HTML (first 800 chars) ---`);
  console.log(analysis.mainHtml.substring(0, 800));

  return analysis;
}

async function main() {
  console.log('Page Content Analysis');
  console.log('=====================\n');

  let ws;
  try {
    const debuggerUrl = await getDebuggerUrl();
    ws = new WebSocket(debuggerUrl);
    await new Promise((resolve, reject) => {
      ws.on('open', resolve);
      ws.on('error', reject);
      setTimeout(() => reject(new Error('WS timeout')), 10000);
    });

    const cdp = createCDP(ws);
    await cdp.send('Page.enable');
    await cdp.send('Runtime.enable');

    // Check if logged in
    await cdp.send('Page.navigate', { url: `${BASE_URL}/dashboard` });
    await sleep(2000);
    const url = await evaluate(cdp, 'window.location.href');

    if (url.includes('/login')) {
      console.log('Need to login first...');
      await evaluate(cdp, `
        (function() {
          const email = document.querySelector('#login-email');
          const pwd = document.querySelector('input[type="password"]');
          if (email && pwd) {
            const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
            setter.call(email, 'admin@test.com');
            setter.call(pwd, 'TestPass123!');
            email.dispatchEvent(new Event('input', { bubbles: true }));
            pwd.dispatchEvent(new Event('input', { bubbles: true }));
          }
        })()
      `);
      await sleep(500);
      await evaluate(cdp, `
        document.querySelector('button[type="submit"]')?.click()
      `);
      await sleep(3000);
    }

    // Analyze problem pages
    const pagesToAnalyze = [
      ['/servers', 'Servers'],
      ['/traffic', 'Traffic'],
      ['/knowledge', 'Knowledge'],
      ['/admin/agents', 'Admin Agents'],
      ['/admin/plans', 'Admin Plans'],
      ['/admin/notices', 'Admin Notices'],
      ['/admin/system', 'Admin System']
    ];

    const results = {};
    for (const [path, name] of pagesToAnalyze) {
      results[name] = await analyzePage(cdp, path, name);
    }

    console.log('\n\n' + '='.repeat(60));
    console.log('  SUMMARY');
    console.log('='.repeat(60));

    for (const [name, result] of Object.entries(results)) {
      const status = result.mainText.length < 50 ? '❌' :
                     result.mainText.length < 100 ? '⚠️' : '✅';
      console.log(`${status} ${name}: ${result.mainText.length} chars, empty=${result.hasEmptyState}, table=${result.hasTable}`);
    }

  } catch (e) {
    console.error('Error:', e.message);
  } finally {
    if (ws) ws.close();
  }
}

main();
