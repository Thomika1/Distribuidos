import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';

const lockWaitMs = new Trend('demo_lock_wait_ms');
const lockElapsedMs = new Trend('demo_elapsed_ms');
const createdCount = new Counter('demo_created');
const conflictCount = new Counter('demo_conflict');

export const options = {
  scenarios: {
    concurrent_burst: {
      executor: 'per-vu-iterations',
      vus: 5,
      iterations: 1,
      maxDuration: '15s',
    },
  },
};

function makePayload() {
  return JSON.stringify({
    transaction_date: new Date().toISOString(),
    transaction_value: '100.00',
    card_brand: 'visa',
    reason_code: '10.4',
  });
}

export default function () {
  const vuId = __VU;
  const start = Date.now();

  const res = http.post(`${BASE_URL}/api/v1/chargebacks`, makePayload(), {
    headers: { 'Content-Type': 'application/json' },
    tags: { vu: `vu${vuId}` },
  });

  const elapsed = Date.now() - start;
  const waitMs = res.headers['X-Lock-Wait-Ms'] || '0';
  const acquired = res.headers['X-Lock-Acquired'] || 'N/A';
  const status = res.status;

  lockElapsedMs.add(elapsed, { vu: `vu${vuId}`, status: status.toString() });
  if (waitMs) lockWaitMs.add(parseFloat(waitMs), { vu: `vu${vuId}` });

  if (status === 201) createdCount.add(1, { vu: `vu${vuId}`, wait: waitMs });
  else if (status === 409) conflictCount.add(1, { vu: `vu${vuId}`, wait: waitMs });

  console.log(`VU${vuId} | status=${status} | lock=${acquired} | wait=${waitMs}ms | total=${elapsed}ms`);

  check(res, {
    'status is 201 or 409': (r) => r.status === 201 || r.status === 409,
  });

  sleep(0.1);
}

export function handleSummary(data) {
  const dash = '═'.repeat(60);
  const dash2 = '─'.repeat(60);

  const created = data.metrics.demo_created?.values?.count || 0;
  const conflict = data.metrics.demo_conflict?.values?.count || 0;
  const waitAvg = data.metrics.demo_lock_wait_ms?.values?.avg?.toFixed(2) || '0';
  const waitMax = data.metrics.demo_lock_wait_ms?.values?.max?.toFixed(2) || '0';
  const waitMin = data.metrics.demo_lock_wait_ms?.values?.min?.toFixed(2) || '0';
  const elapsedAvg = data.metrics.demo_elapsed_ms?.values?.avg?.toFixed(2) || '0';
  const elapsedMax = data.metrics.demo_elapsed_ms?.values?.max?.toFixed(2) || '0';

  const output = `
${dash}
   DEMO: 5 VUs SIMULTÂNEOS COM LOCK 2PL
${dash}

  Cada VU envia 1 request POST ao mesmo tempo.
  O lock exclusivo serializa as requests (apenas 1 processa por vez).
  As demais esperam até o lock ser liberado ou atingirem timeout (2s).

${dash2}

  RESULTADO DAS 5 REQUESTS:
${dash2}
    ✅ Lock adquirido (201):   ${created} requests processaram
    ❌ Lock timeout (409):      ${conflict} requests bloqueadas

${dash2}

  TEMPOS DE ESPERA PELO LOCK:
${dash2}
    Wait mínimo:   ${waitMin} ms   (primeira request não espera)
    Wait médio:    ${waitAvg} ms
    Wait máximo:   ${waitMax} ms   (última request espera mais)
${dash2}

  TEMPO TOTAL (wait + processamento):
${dash2}
    Total médio:   ${elapsedAvg} ms
    Total máximo:  ${elapsedMax} ms
${dash2}

  INTERPRETAÇÃO:
${dash2}
    1. A request com wait=0ms chegou primeiro e pegou o lock imediatamente
    2. As outras ficaram esperando (wait > 0) = FILA do lock 2PL
    3. Se wait máximo ≈ 2000ms = request atingiu timeout (409)
    4. Se todas 201 com wait crescente = serialização funcionando perfeitamente
${dash}
`;

  return { stdout: output };
}