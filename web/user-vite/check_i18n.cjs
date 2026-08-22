/**
 * Check i18n issue on AgentList page
 */
const WebSocket = require('ws');
const http = require('http');

const CDP_URL = 'http://127.0.0.1:9222';
const BASE_URL = 'http://localhost:8080';

async function getDebuggerUrl() {
  return new Promise((resolve, reject) => {
    http.get(`${CDP_URL}/json`, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try {
          const pages = JSON.parse(data);
          if (pages.length > 0) {
            resolve(pages[0].webSocketDebuggerUrl);
          } else {
            reject(new Error('No pages found'));
          }
        } catch (e) {
          reject(e);
        }
      });
    }).on('error', reject);
  });
}

async function sendCommand(ws, method, params = {}) {
  const id = Math.floor(Math.random() * 1000000);
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error(`Timeout: ${method}`)), 30000);
    const handler = (msg) => {
      const data = JSON.parse(msg);
      if (data.id === id) {
        clearTimeout(timeout);
        ws.off('message', handler);
        if (data.error) reject(new Error(data.error.message));
        else resolve(data.result);
      }
    };
    ws.on('message', handler);
    ws.send(JSON.stringify({ id, method, params }));
  });
}

(async () => {
  console.log('=== i18n Check ===\n');

  const debuggerUrl = await getDebuggerUrl();
  const ws = new WebSocket(debuggerUrl);

  await new Promise((resolve, reject) => {
    ws.on('open', resolve);
    ws.on('error', reject);
  });

  await sendCommand(ws, 'Page.enable');
  await sendCommand(ws, 'Runtime.enable');

  // First login
  console.log('1. Logging in...');
  await sendCommand(ws, 'Page.navigate', { url: `${BASE_URL}/login` });
  await new Promise(r => setTimeout(r, 3000));

  // Fill login form
  await sendCommand(ws, 'Runtime.evaluate', {
    expression: `
      (function() {
        const emailInput = document.querySelector('input[name="email"]') || document.querySelector('input[type="email"]');
        const passwordInput = document.querySelector('input[name="password"]') || document.querySelector('input[type="password"]');
        if (emailInput && passwordInput) {
          emailInput.value = 'admin@example.com';
          emailInput.dispatchEvent(new Event('input', { bubbles: true }));
          passwordInput.value = 'admin123';
          passwordInput.dispatchEvent(new Event('input', { bubbles: true }));
          return 'Login form filled';
        }
        return 'Form not found';
      })()
    `,
    returnByValue: true
  });

  // Click login button
  await sendCommand(ws, 'Runtime.evaluate', {
    expression: `
      (function() {
        const btn = document.querySelector('button[type="submit"]');
        if (btn) { btn.click(); return 'Clicked'; }
        return 'Button not found';
      })()
    `,
    returnByValue: true
  });

  await new Promise(r => setTimeout(r, 3000));

  // Navigate to admin/agents
  console.log('2. Navigating to /admin/agents...');
  await sendCommand(ws, 'Page.navigate', { url: `${BASE_URL}/admin/agents` });
  await new Promise(r => setTimeout(r, 3000));

  // Check page content
  const result = await sendCommand(ws, 'Runtime.evaluate', {
    expression: `
      (function() {
        const bodyText = document.body.innerText;
        const html = document.body.innerHTML;

        // Check for specific i18n issues
        const issues = [];

        // Check for untranslated keys (pattern: xxx.yyy.zzz)
        const keyPattern = /[a-z]+\\.[a-z]+\\.[a-zA-Z]+/g;
        const matches = bodyText.match(keyPattern) || [];
        const translationKeys = matches.filter(m =>
          m.includes('admin.') || m.includes('common.') || m.includes('user.')
        );

        if (translationKeys.length > 0) {
          issues.push('Untranslated keys: ' + translationKeys.join(', '));
        }

        return {
          url: window.location.href,
          bodyText: bodyText.substring(0, 1000),
          hasEmptyDescription: bodyText.includes('emptyDescription'),
          issues: issues
        };
      })()
    `,
    returnByValue: true
  });

  console.log('\n=== Page Analysis ===');
  console.log('URL:', result.result.value.url);
  console.log('Has emptyDescription key:', result.result.value.hasEmptyDescription);
  console.log('Issues:', result.result.value.issues.join('\n'));
  console.log('\n=== Body Text ===');
  console.log(result.result.value.bodyText);

  // Take screenshot
  const screenshot = await sendCommand(ws, 'Page.captureScreenshot', { format: 'png' });
  require('fs').writeFileSync('test-results/manual/i18n_check.png', Buffer.from(screenshot.data, 'base64'));
  console.log('\nScreenshot saved to test-results/manual/i18n_check.png');

  ws.close();
  console.log('\nDone!');
})();
