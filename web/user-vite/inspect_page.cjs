const { chromium } = require('playwright');

(async () => {
  const BASE_URL = 'http://127.0.0.1:8080';
  const ADMIN = {
    email: 'admin@example.com',
    username: 'admin',
    password: 'Admin12345!'
  };

  console.log('[1] 连接调试浏览器...');
  let browser;
  try {
    // 尝试连接调试端口
    browser = await chromium.connectOverCDP('http://127.0.0.1:9222', { timeout: 30000 });
  } catch (e) {
    console.error('CDP 连接失败，尝试直接使用 WebSocket:', e.message);
    const fetch = await import('node-fetch').then(m => m.default).catch(() => null);
    if (fetch) {
      const res = await fetch('http://127.0.0.1:9222/json/version').then(r => r.json());
      browser = await chromium.connectOverCDP(res.webSocketDebuggerUrl, { timeout: 30000 });
    } else {
      throw e;
    }
  }

  const context = browser.contexts()[0] || await browser.newContext();
  const page = await context.newPage();

  try {
    console.log('[2] 访问安装页面...');
    await page.goto(`${BASE_URL}/install`, { waitUntil: 'networkidle' });
    await page.screenshot({ path: 'test-results/manual/step-01-install.png' });

    console.log('[3] 填写初始化表单...');
    await page.fill('#install-email', ADMIN.email);
    await page.fill('#install-username', ADMIN.username);
    await page.fill('#install-password', ADMIN.password);
    await page.fill('#install-confirm-password', ADMIN.password);
    await page.screenshot({ path: 'test-results/manual/step-02-install-filled.png' });

    console.log('[4] 提交安装...');
    await page.click('button[type="submit"]');

    // 等待跳转到登录页
    await page.waitForURL('**/login', { timeout: 10000 });
    console.log('  [成功] 已跳转到登录页');
    await page.screenshot({ path: 'test-results/manual/step-03-install-done.png' });

    console.log('[5] 执行登录...');
    await page.fill('#login-email', ADMIN.email);
    await page.fill('#login-password', ADMIN.password);
    await page.click('button[type="submit"]');

    await page.waitForURL('**/dashboard', { timeout: 10000 });
    console.log('  [成功] 已进入 Dashboard');
    await page.screenshot({ path: 'test-results/manual/step-04-dashboard.png' });

  } catch (err) {
    console.error('发生错误:', err);
    await page.screenshot({ path: 'test-results/manual/step-error.png' });
  } finally {
    await browser.close();
  }
})();
