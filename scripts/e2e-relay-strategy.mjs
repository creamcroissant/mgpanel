import { chromium } from "playwright-core";
import { readFileSync } from "fs";
const JWT = readFileSync("/tmp/admin.jwt", "utf8").trim();
const browser = await chromium.launch();
const ctx = await browser.newContext({ ignoreHTTPSErrors: true });
await ctx.addInitScript(([t]) => sessionStorage.setItem("mgpanel-token", t), [JWT]);
const page = await ctx.newPage();
page.on("pageerror", e => console.log("PAGEERROR:", e.message.slice(0,150)));
let apiLog = [];
page.on("response", async r => {
  const u = r.url();
  if (u.includes("/relay-paths") && r.request().method() !== "GET") apiLog.push(`${r.request().method()} → ${r.status()}`);
});

await page.goto("https://154.88.64.107/", { waitUntil: "domcontentloaded" }).catch(()=>{});
await page.waitForTimeout(300);
await page.goto("https://154.88.64.107/admin/routing", { waitUntil: "networkidle", timeout: 30000 }).catch(()=>{});
await page.waitForTimeout(1200);
await page.locator('button:has-text("拓扑")').first().click(); await page.waitForTimeout(1000);
await page.locator('button:has-text("Server Links")').first().click(); await page.waitForTimeout(800);

// ===== 链路A: HK(agent-3) → TW(agent-4) 连线弹窗创建 =====
console.log("== 链路A: HK → TW (连线弹窗) ==");
await page.locator('button[title^="Connect"], button[title^="连线"]').first().click(); await page.waitForTimeout(250);
const agents = page.locator('[data-id^="agent-"]');
for (let i = 0; i < await agents.count(); i++) {
  if ((await agents.nth(i).getAttribute("data-id")) === "agent-3") {
    const b = await agents.nth(i).boundingBox();
    await page.mouse.click(b.x + b.width/2, b.y + b.height/2); break;
  }
}
await page.waitForTimeout(350);
for (let i = 0; i < await agents.count(); i++) {
  if ((await agents.nth(i).getAttribute("data-id")) === "agent-4") {
    const b = await agents.nth(i).boundingBox();
    await page.mouse.click(b.x + b.width/2, b.y + b.height/2); break;
  }
}
await page.waitForTimeout(700);
const dlg = page.locator("[role=dialog]");
if (!(await dlg.count())) { console.log("❌ 弹窗未出现"); process.exit(1); }
await dlg.locator("input").first().fill("hk-to-tw-backbone");
await dlg.locator('button:has-text("创建链路")').click();
await page.waitForTimeout(2000);
console.log("   创建完成, 边数:", await page.locator(".react-flow__edge").count());

// ===== 链路B: HK → JPS → US 三跳（先建两跳，再经 Drawer 追加第三节点）=====
console.log("== 链路B: HK → JPS → US-dedirock (三跳) ==");
// 步骤1: 连线 HK→JPS 建基础链路
for (let i = 0; i < await agents.count(); i++) {
  if ((await agents.nth(i).getAttribute("data-id")) === "agent-3") {
    const b = await agents.nth(i).boundingBox();
    await page.mouse.click(b.x + b.width/2, b.y + b.height/2); break;
  }
}
await page.waitForTimeout(350);
for (let i = 0; i < await agents.count(); i++) {
  if ((await agents.nth(i).getAttribute("data-id")) === "agent-2") {
    const b = await agents.nth(i).boundingBox();
    await page.mouse.click(b.x + b.width/2, b.y + b.height/2); break;
  }
}
await page.waitForTimeout(700);
const dlg2 = page.locator("[role=dialog]").last();
if (!(await dlg2.count())) { console.log("❌ 弹窗未出现"); process.exit(1); }
await dlg2.locator("input").first().fill("hk-jp-us-transit");
await dlg2.locator('button:has-text("创建链路")').click();
await page.waitForTimeout(2000);

// 步骤2: 点击该链路的边打开 Drawer，追加 dedirock-us 节点
const relayEdges = page.locator('.react-flow__edge');
const edgeCount = await relayEdges.count();
console.log("   当前边数:", edgeCount);
// 找到 hk-jp-us-transit 的边（最新创建的 relay 边）——点击最后一条 relay 边
let clicked = false;
for (let i = edgeCount - 1; i >= 0 && !clicked; i--) {
  const id = await relayEdges.nth(i).getAttribute("data-id");
  if (id?.includes("relay-")) {
    const b = await relayEdges.nth(i).boundingBox();
    if (b) { await page.mouse.click(b.x + b.width/2, b.y + b.height/2); clicked = true; }
  }
}
await page.waitForTimeout(800);
// Drawer 中应有"添加节点"下拉 + 行列表；追加 agent-5
const drawerDialog = page.locator("[role=dialog], .fixed.inset-y-0").last();
const addRowBtn = page.locator('button:has-text("添加"), button:has-text("Add"), button[title*="add"], button:has-text("+ 添加")');
console.log("   添加按钮候选:", await addRowBtn.count());
// 直接找 RelayPathDrawer 内的加行控件：查看 schema MemberRow 模式的按钮
const plusRow = page.locator('div.fixed button:has-text("+")').last();
console.log("   + 按钮:", await plusRow.count());
if (await plusRow.count()) {
  await plusRow.click(); await page.waitForTimeout(400);
  // 新行的 agent 下拉选择 dedirock-us
  const selects = page.locator('[data-slot="select-trigger"], button[role="combobox"]');
  const sc = await selects.count();
  console.log("   抽屉内下拉:", sc);
  await selects.last().click(); await page.waitForTimeout(300);
  // 选含 us/dedirock 的选项
  const opt = page.locator('[role="option"]', { hasText: /us|dedirock|192\.236/i }).first();
  if (await opt.count()) { await opt.click(); await page.waitForTimeout(300); }
  else { await page.locator('[role="option"]').last().click(); await page.waitForTimeout(300); }
  // 保存
  const sv = page.locator('button:has-text("Save"), button:has-text("保存"), button:has-text("保存修改"), button:has-text("保存更改")').last();
  console.log("   保存按钮:", await sv.count());
  await sv.click(); await page.waitForTimeout(2000);
}

console.log("\n========= API 记录 =========");
apiLog.forEach(x => console.log(" ", x));
console.log("========= 最终边数 =========");
console.log(await page.locator(".react-flow__edge").count());
await page.screenshot({ path: "/tmp/relay-strategy.png", fullPage: false });
await browser.close();
