export const meta = {
  name: 'full-audit-final',
  description: '全量并行审计 — 8路Agent 覆盖所有模块',
  phases: [
    { title: 'Audit', detail: '8 路并行全量审计' },
    { title: 'Report', detail: '汇总报告' },
  ],
};

const S = {
  type: 'object',
  properties: {
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          severity: { type: 'string', enum: ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'] },
          category: { type: 'string' },
          file: { type: 'string' },
          line: { type: 'integer' },
          summary: { type: 'string' },
          detail: { type: 'string' },
          recommendation: { type: 'string' },
        },
        required: ['severity', 'category', 'file', 'summary', 'recommendation'],
      },
    },
    module_summary: { type: 'string' },
  },
  required: ['findings', 'module_summary'],
};

phase('Audit');

log('启动 8 路全量并行审计...');

const results = await parallel([
  () => agent('审计 cmd/mgpanel/ 和 cmd/agent/ 和 internal/config/。检查所有 .go 文件。\n\n检查项：\n1. 错误处理：_ = func() 模式、log.Fatal/panic(非main)、err.Error()泄露\n2. Context传播：所有 context.Background()/TODO()是否合理\n3. Shutdown：信号处理、graceful shutdown顺序、goroutine等待\n4. 资源泄露：ticker、文件描述符、终端恢复\n5. 并发安全：全局变量无锁访问\n\n命令参考：\ngrep -rn \'_ = \' cmd/ internal/config/ --include="*.go" | grep -v \'_test.go\'\ngrep -rn \'context.Background()\' cmd/ internal/config/ --include="*.go"\ngrep -rn \'context.TODO()\' cmd/ internal/config/ --include="*.go"\n\n只输出 Read 确认后的真实问题。', { label: 'audit:cmd+config', schema: S }),

  () => agent('全量审计 internal/service/（112 个 .go 文件）。\n\n检查：\n1. _ = func() 错误吞没（排除 .Store/.Load/.Delete/.Set/.Close 等惯用模式）\n2. context.Background()/TODO()是否合理\n3. map 无锁保护（struct map 字段缺 sync.RWMutex）\n4. time.Sleep 在生产代码中\n5. defer 在 for 循环中\n6. for+select+ticker 的 defer Stop()\n\n命令：\ngrep -rn \'_ = \' internal/service/ --include="*.go" | grep -v \'_test.go\' | grep -v \'\\.Store\|\\.Load\|\\.Delete\|\\.Set\|\\.Close\|\\.Remove\|\\.Done\|\\.Rollback\|\\.Err()\'\ngrep -rn \'context.Background()\' internal/service/ --include="*.go" | grep -v \'_test.go\'\n\n对每个命中 Read 确认后输出。', { label: 'audit:service', schema: S }),

  () => agent('全量审计 internal/repository/sqlite/（55 个 .go 文件）。\n\n检查：\n1. SQL注入：fmt.Sprintf拼接字段名、WHERE/ORDER BY\n2. ErrNoRows未处理：scan函数是否都转换了sql.ErrNoRows\n3. Missing RowsAffected：单行DELETE/UPDATE是否都有检查\n4. Missing rows.Close/rows.Err\n5. LIMIT/OFFSET参数化\n6. 列不对齐：迁移添加的列在CRUD中完整\n\n命令：\ngrep -rn \'fmt\\.Sprintf.*WHERE\|fmt\\.Sprintf.*ORDER\|+ " WHERE\|+ " ORDER\' --include="*.go" | grep -v \'_test.go\'\ngrep -rn \', _ := \' --include="*.go" | grep -v \'_test.go\' | grep -i \'ExecContext\|Exec(\'\ngrep -rn \'rows.Close\|rows.Err\' --include="*.go" | grep -v \'_test.go\'\n\n对每个候选 Read 确认。', { label: 'audit:repository', schema: S }),

  () => agent('全量审计 internal/api/。\n\n检查：\n1. err.Error()泄露：任何返回给客户端的err.Error()\n2. 鉴权缺失：handler无AdminFromContext/requireAdmin\n3. 路径穿越：路径参数未验证\n4. 响应一致性：非JSON响应\n5. 请求体大小限制\n6. 安全头完整性\n\n命令：\ngrep -rn \'err\\.Error()\' internal/api/ --include="*.go" | grep -v \'_test.go\' | grep -v \'//.*TODO\|slog\\.\|log\\.\'\ngrep -rn \'http\\.Error\' internal/api/handler/ --include="*.go" | grep -v \'_test.go\'\ngrep -rn \'os\\.Open\|ioutil\\.ReadFile\|os\\.ReadFile\' internal/api/ --include="*.go" | grep -v \'_test.go\'\n\n对每个候选 Read 确认。', { label: 'audit:api', schema: S }),

  () => agent('全量审计 internal/agent/（排除 service/ 子目录）。子包：access/api/capability/cdn/command/config/configcenter/core/forwarding/grpc/initsys/mesh/monitor/protocol/proxy/server/stream/syncer/traffic/updater/\n\n检查：\n1. exec.Command无context：应使用exec.CommandContext\n2. goroutine泄漏：go func是否有退出机制\n3. time.Sleep在生产代码\n4. FD泄漏：os.Open/os.Create有Close？\n5. sync.Once误用\n6. channel close未保护\n7. 进程管理：cmd.Start后cmd.Wait\n\n命令：\ngrep -rn \'exec\\.Command(\' internal/agent/ --include="*.go" | grep -v \'_test.go\' | grep -v \'CommandContext\|ctx\|context\'\ngrep -rn \'time\\.Sleep\' internal/agent/ --include="*.go" | grep -v \'_test.go\'\ngrep -rn \'go func\' internal/agent/ --include="*.go" | grep -v \'_test.go\' | grep -v \'\\.Wait\|\\.Done\|defer\|WaitGroup\|sync\\.\'\n\n对每个候选 Read 确认。', { label: 'audit:agent-subpackages', schema: S }),

  () => agent('全量审计 internal/agent/service/。\n\n重点：service.go, mesh_probe.go, mesh_route.go, cdn_probe.go, cdn_operation.go\n\n检查：\n1. service.go主循环：sync/report goroutine生命周期、ticker Stop、shutdown顺序\n2. goroutine泄漏：OnStateChange、keepalive、CDN probe\n3. context传播：Background是否合理使用\n4. 并发：map访问保护、atomic使用\n5. mesh_probe：keepalive循环退出路径\n\n命令：\ngrep -rn \'context.Background()\' internal/agent/service/ --include="*.go"\ngrep -rn \'go func\|go \' internal/agent/service/ --include="*.go"\ngrep -rn \'time\\.Sleep\' internal/agent/service/ --include="*.go"\n\n对每个候选 Read 确认后输出。', { label: 'audit:agent-service', schema: S }),

  () => agent('全量审计 internal/grpc/ internal/job/ internal/probe/ internal/cdn/。\n\n检查：\n1. gRPC：消息大小限制、拦截器顺序、GracefulStop超时、流处理ctx\n2. Job：panic保护、任务cleanup、超时\n3. Probe：stopCh竞争、ctx传播、定时器\n4. CDN：凭据安全、重试逻辑、Provider设计\n\n命令：\ngrep -rn \'context.Background()\' internal/grpc/ internal/job/ internal/probe/ internal/cdn/ --include="*.go" | grep -v \'_test.go\'\ngrep -rn \'_ = \' internal/grpc/ internal/job/ internal/probe/ internal/cdn/ --include="*.go" | grep -v \'_test.go\' | grep -v \'\\.Store\|\\.Load\|\\.Delete\|\\.Close\|\\.Rollback\'\n\n对每个候选 Read 确认后输出。', { label: 'audit:infra', schema: S }),

  () => agent('全量审计 web/user-vite/src/。\n\n检查：\n1. localStorage：是否所有token都已迁移到sessionStorage\n2. as any类型擦除：尤其是config-center\n3. dangerouslySetInnerHTML：是否有DOMPurify\n4. useEffect依赖数组：空依赖、缺失依赖\n5. 非空断言!：特别是在config-center中\n6. SSE/WebSocket cleanup：useEffect返回cleanup\n7. 硬编码字符串：非i18n\n8. 路由安全：未登录可访问管理页面？\n\n命令：\ngrep -rn \'localStorage\' web/user-vite/src/ --include="*.ts" --include="*.tsx" | grep -v node_modules | grep -v \'\\.d\\.ts\'\ngrep -rn \'as any\' web/user-vite/src/pages/admin/config-center/ --include="*.tsx" --include="*.ts" | grep -v \'eslint\|node_modules\'\ngrep -rn \'dangerouslySetInnerHTML\|\\.innerHTML\' web/user-vite/src/ --include="*.tsx" --include="*.ts" | grep -v \'node_modules\|\\.d\\.ts\'\ngrep -rn \'useEffect(\' web/user-vite/src/ --include="*.tsx" --include="*.ts" | head -30\n\n对每个候选 Read 确认后输出。', { label: 'audit:frontend', schema: S }),
]);

phase('Report');

log('8 路审计全部完成，汇总中...');

const allFindings = [];
let verifiedCount = 0;

for (const r of results) {
  if (r && r.findings) {
    const seen = new Set();
    for (const f of r.findings) {
      const key = f.file + ':' + (f.line || 0) + ':' + f.summary;
      if (!seen.has(key)) { seen.add(key); allFindings.push(f); }
    }
  }
}

const order = { CRITICAL: 0, HIGH: 1, MEDIUM: 2, LOW: 3 };
allFindings.sort((a, b) => (order[a.severity] ?? 9) - (order[b.severity] ?? 9));

const counts = { CRITICAL: 0, HIGH: 0, MEDIUM: 0, LOW: 0 };
for (const f of allFindings) counts[f.severity] = (counts[f.severity] || 0) + 1;

const byCat = {};
for (const f of allFindings) {
  const c = f.category || 'other';
  if (!byCat[c]) byCat[c] = [];
  byCat[c].push(f);
}

const report = [
  '=== 全量并行审计报告 ===',
  '',
  '总计: ' + allFindings.length + ' 条发现',
  '  CRITICAL: ' + (counts.CRITICAL || 0),
  '  HIGH:     ' + (counts.HIGH || 0),
  '  MEDIUM:   ' + (counts.MEDIUM || 0),
  '  LOW:      ' + (counts.LOW || 0),
  '',
  '--- 按类别分布 ---',
];
for (const [cat, items] of Object.entries(byCat).sort()) {
  const crit = items.filter(f => f.severity === 'CRITICAL').length;
  const high = items.filter(f => f.severity === 'HIGH').length;
  report.push('  ' + cat + ': ' + items.length + ' (CRITICAL=' + crit + ', HIGH=' + high + ')');
}

report.push('', '--- 发现详情 ---');
for (const f of allFindings) {
  report.push('[' + f.severity + '] ' + f.file + ':' + (f.line || '?') + ' — ' + f.summary);
  if (f.recommendation) report.push('  => ' + f.recommendation);
}

const summaries = results.filter(Boolean).map(r => r.module_summary).filter(Boolean);
if (summaries.length) {
  report.push('', '--- 模块摘要 ---');
  for (const s of summaries) report.push(s);
}

const output = report.join('\n');
log(output);

await agent('将审计报告写入 /opt/work/mgpanel/docs/superpowers/audit/2026-07-22-audit-final.md\n\n' + output, { label: 'write-report' });

return { total: allFindings.length, counts, findings: allFindings.slice(0, 50) };
