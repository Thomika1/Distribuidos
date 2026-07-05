import http from 'k6/http';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';

const status201 = new Counter('status_201_created');
const status409 = new Counter('status_409_conflict');
const lockWaitMs = new Trend('lock_wait_ms');
const lockAcquired = new Counter('lock_acquired_true');

export const options = {
  scenarios: {
    with_lock: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '3s', target: 30 },
        { duration: '10s', target: 50 },
        { duration: '3s', target: 0 },
      ],
      exec: 'withLock',
    },
    without_lock: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '3s', target: 30 },
        { duration: '10s', target: 50 },
        { duration: '3s', target: 0 },
      ],
      exec: 'withoutLock',
      startTime: '20s',
    },
  },
};

function makePayload() {
  const brands = ['visa', 'mastercard'];
  const visaCodes = ['10.1', '10.2', '10.4', '13.1', '12.5'];
  const mcCodes = ['4837', '4801', '4853', '4808', '4504'];
  const brand = brands[Math.floor(Math.random() * brands.length)];
  const codes = brand === 'visa' ? visaCodes : mcCodes;
  return JSON.stringify({
    transaction_date: new Date().toISOString(),
    transaction_value: (Math.random() * 1000 + 10).toFixed(2),
    card_brand: brand,
    reason_code: codes[Math.floor(Math.random() * codes.length)],
  });
}

export function withLock() {
  const res = http.post(`${BASE_URL}/api/v1/chargebacks`, makePayload(), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status === 201) status201.add(1);
  else if (res.status === 409) status409.add(1);

  const waitMs = res.headers['X-Lock-Wait-Ms'];
  if (waitMs) lockWaitMs.add(parseFloat(waitMs));
  if (res.headers['X-Lock-Acquired'] === 'true') lockAcquired.add(1);

  check(res, {
    'status is 201 or 409': (r) => r.status === 201 || r.status === 409,
    'has lock header': (r) => r.headers['X-Lock-Acquired'] !== undefined,
  });
}

export function withoutLock() {
  const res = http.post(`${BASE_URL}/api/v1/chargebacks/nolock`, makePayload(), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (res.status === 201) status201.add(1);

  check(res, {
    'status is 201': (r) => r.status === 201,
    'no lock header': (r) => r.headers['X-Lock-Acquired'] === undefined,
  });
}

export function handleSummary(data) {
  const dash = '═'.repeat(60);
  const dash2 = '─'.repeat(60);

  const total201 = data.metrics.status_201_created?.values?.count || 0;
  const total409 = data.metrics.status_409_conflict?.values?.count || 0;
  const lockWaitAvg = data.metrics.lock_wait_ms?.values?.avg?.toFixed(2) || '0';
  const lockWaitMax = data.metrics.lock_wait_ms?.values?.max?.toFixed(2) || '0';
  const lockWaitP95 = data.metrics.lock_wait_ms?.values?.['p(95)']?.toFixed(2) || '0';
  const totalReqs = data.metrics.http_reqs?.values?.count || 0;
  const reqRate = data.metrics.http_reqs?.values?.rate?.toFixed(2) || '0';
  const durAvg = data.metrics.http_req_duration?.values?.avg?.toFixed(2) || '0';
  const durP95 = data.metrics.http_req_duration?.values?.['p(95)']?.toFixed(2) || '0';

  const output = `
${dash}
   RESULTADOS - TESTE DE CARGA COM CONTROLE DE CONCORRÊNCIA 2PL
${dash}

  COM LOCK 2PL (POST /api/v1/chargebacks)
${dash2}
    Requests totais:        ${totalReqs}
    Taxa:                   ${reqRate} req/s
    201 Created:            ${total201}  (lock adquirido, processou)
    409 Conflict:           ${total409}  (lock timeout = BLOQUEIO FUNCIONANDO)

    Lock wait médio:        ${lockWaitAvg} ms
    Lock wait máximo:       ${lockWaitMax} ms
    Lock wait p95:          ${lockWaitP95} ms

    Duração média:          ${durAvg} ms
    Duração p95:            ${durP95} ms
${dash2}

  PROVAS DO 2PL FUNCIONANDO:
${dash2}
    1. ${total409 > 0 ? '✅' : '❌'} ${total409} requests receberam 409 (não conseguiram lock)
    2. ${parseFloat(lockWaitAvg) > 0 ? '✅' : '❌'} Lock wait médio = ${lockWaitAvg}ms (requests esperaram)
    3. ${parseFloat(lockWaitMax) > 1000 ? '✅' : '❌'} Lock wait max = ${lockWaitMax}ms (fila de espera)
    4. ${total409 === 0 && parseFloat(lockWaitAvg) === 0 ? '⚠️  Pouca contenção - aumente VUs ou delay' : '✅ Contenção detectada'}
${dash}
`;

  return { stdout: output };
}