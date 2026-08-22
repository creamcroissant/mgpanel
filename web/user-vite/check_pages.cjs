/**
 * Check Knowledge and Admin System pages
 */
const WebSocket = require('ws');
const http = require('http');
const fs = require('fs');

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

async function checkPage(ws, path, name) {
  console.log(`\n=== Checking ${name} (${path}) ===`);
  await sendCommand(ws, 'Page.navigate', { url: `${BASE_URL}${path}` });
  await new Promise(r => setTimeout(r, 3000));

  const result = await sendCommand(ws, 'Runtime.evaluate', {
    expression: `
      (function() {
        return {
          url: window.location.href,
          bodyText: document.body.innerText.substring(0, 1500),
          // Check for loading spinner
          hasSpinner: document.querySelector('[role="status"]') !== null ||
                      document.body.innerHTML.includes('animate-spin') ||
                      document.body.innerHTML.includes('loading'),
          // Check for error messages
          hasError: document.body.innerText.includes('失败') ||
                    document.body.innerText.includes('Error') ||
                    document.body.innerText.includes('error'),
          // Console errors
          consoleErrors: window.__consoleErrors || []
        };
      })()
    `,
    returnByValue: true
  });

  const data = result.result.value;
  console.log('URL:', data.url);
  console.log('Has Spinner:', data.hasSpinner);
  console.log('Has Error:', data.hasError);
  console.log('Body Text:', data.bodyText);

  // Take screenshot
  const screenshot = await sendCommand(ws, 'Page.captureScreenshot', { format: 'png' });
  const filename = `test-results/manual/${name.toLowerCase().replace(/\s+/g, '_')}.png`;
  fs.writeFileSync(filename, Buffer.from(screenshot.data, 'base64'));
  console.log('Screenshot saved:', filename);
}

(async () => {
  console.log('=== Page Check ===\n');

  const debuggerUrl = await getDebuggerUrl();
  const ws = new WebSocket(debuggerUrl);

  await new Promise((resolve, reject) => {
    ws.on('open', resolve);
    ws.on('error', reject);
  });

  await sendCommand(ws, 'Page.enable');
  await sendCommand(ws, 'Runtime.enable');

  // Add console error capture
  await sendCommand(ws, 'Runtime.evaluate', {
    expression: `
      window.__consoleErrors = [];
      const originalError = console.error;
      console.error = (...args) => {
        window.__consoleErrors.push(args.join(' '));
        originalError.apply(console, args);
      };
    `
  });

  // Login first
  console.log('Logging in...');
  await sendCommand(ws, 'Page.navigate', { url: `${BASE_URL}/login` });
  await new Promise(r => setTimeout(r, 2000));

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
          const btn = document.querySelector('button[type="submit"]');
          if (btn) btn.click();
        }
      })()
    `
  });
  await new Promise(r => setTimeout(r, 3000));

  // Check pages
  await checkPage(ws, '/knowledge', 'Knowledge');
  await checkPage(ws, '/admin/system', 'Admin System');

  ws.close();
  console.log('\n=== Done ===');
})();
