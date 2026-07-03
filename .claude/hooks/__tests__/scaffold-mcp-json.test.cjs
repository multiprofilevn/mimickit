/**
 * Tests for db-connect/scripts/scaffold-mcp-json.cjs — spawned via child_process.
 * Run: node .claude/hooks/__tests__/scaffold-mcp-json.test.cjs
 *
 * Scenarios (per phase-04 spec):
 * 1. postgres: DATABASE_URL in .env.dev → .mcp.json has env-ref "${DATABASE_URL}" + read-only, no raw DSN.
 * 2. redis: REDIS_PASSWORD → mapped to REDIS_PWD env-ref (not REDIS_PASSWORD).
 * 3. merge: pre-existing .mcp.json with server "other" → BOTH "other" and new server present after scaffold.
 * 4. gitignore-ensure: target dir without .gitignore → after run, .gitignore contains ".mcp.json".
 * 5. no raw secret leaks: even if .env.dev has realpass, it MUST NOT appear in .mcp.json.
 * 6. unknown datastore → exit 1.
 * 7. clickhouse: CLICKHOUSE_URL parsed into host/port/user/db env-refs.
 * 8. s3: S3_ENDPOINT/S3_ACCESS_KEY_ID/S3_SECRET_ACCESS_KEY env-refs, S3_REGION=auto.
 * 9. missing env file → graceful error (exit 1), no crash.
 * 10. idempotent: run twice with same server-name → only one entry in mcpServers.
 */

'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const SCRIPT = path.resolve(
  __dirname, '..', '..', 'skills', 'db-connect', 'scripts', 'scaffold-mcp-json.cjs',
);

let passed = 0;
let failed = 0;

function test(name, fn) {
  try {
    fn();
    console.log(`✓ ${name}`);
    passed++;
  } catch (e) {
    console.log(`✗ ${name}`);
    console.log(`  Error: ${e.message}`);
    failed++;
  }
}

function assertEquals(actual, expected, msg = '') {
  if (actual !== expected) {
    throw new Error(
      `${msg}\n  Expected: ${JSON.stringify(expected)}\n  Actual:   ${JSON.stringify(actual)}`,
    );
  }
}

function assertContains(actual, substr, msg = '') {
  if (typeof actual !== 'string' || !actual.includes(substr)) {
    throw new Error(
      `${msg}\n  Expected to contain: ${JSON.stringify(substr)}\n  Actual: ${JSON.stringify(actual)}`,
    );
  }
}

function assertNotContains(actual, substr, msg = '') {
  if (typeof actual === 'string' && actual.includes(substr)) {
    throw new Error(
      `${msg}\n  Expected NOT to contain: ${JSON.stringify(substr)}\n  Actual: ${JSON.stringify(actual)}`,
    );
  }
}

/** Run scaffold-mcp-json.cjs with args. */
function run(args, cwd) {
  return spawnSync(process.execPath, [SCRIPT, ...args], {
    encoding: 'utf8',
    cwd: cwd || process.cwd(),
  });
}

/** Create a temp dir + write .env file, return the dir path. */
function tmpDir(envContent) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'scaffold-mcp-test-'));
  if (envContent) {
    fs.writeFileSync(path.join(dir, '.env.dev'), envContent, 'utf8');
  }
  return dir;
}

function readMcp(dir, filename) {
  return JSON.parse(fs.readFileSync(path.join(dir, filename || '.mcp.json'), 'utf8'));
}

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

test('1. postgres: env-ref "${DATABASE_URL}" in .mcp.json, no raw DSN', () => {
  const dir = tmpDir('DATABASE_URL=postgresql://user:realpass@localhost/mydb\n');
  try {
    const r = run(['postgres', 'pg-dev', '--env', path.join(dir, '.env.dev'), '--out', path.join(dir, '.mcp.json')]);
    assertEquals(r.status, 0, `exit code (stderr: ${r.stderr})`);

    const mcp = readMcp(dir);
    const server = mcp.mcpServers['pg-dev'];
    assertEquals(typeof server, 'object', 'server entry exists');

    const mcpStr = JSON.stringify(mcp);
    assertContains(mcpStr, '${DATABASE_URL}', 'env-ref present');
    assertNotContains(mcpStr, 'realpass', 'no raw secret in .mcp.json');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('2. redis: REDIS_PASSWORD → mapped to REDIS_PWD env-ref', () => {
  const dir = tmpDir('REDIS_HOST=localhost\nREDIS_PORT=6379\nREDIS_PASSWORD=s3cret\n');
  try {
    const r = run(['redis', 'redis-dev', '--env', path.join(dir, '.env.dev'), '--out', path.join(dir, '.mcp.json')]);
    assertEquals(r.status, 0, `exit code (stderr: ${r.stderr})`);

    const mcp = readMcp(dir);
    const server = mcp.mcpServers['redis-dev'];
    assertEquals(typeof server, 'object', 'redis server entry exists');

    // MCP key is the renamed REDIS_PWD, but the ref must point at the var that
    // actually exists at runtime (REDIS_PASSWORD) — else the password is empty.
    assertEquals(server.env.REDIS_PWD, '${REDIS_PASSWORD}', 'REDIS_PWD key refs the real env var');
    assertNotContains(JSON.stringify(mcp), 's3cret', 'no raw secret');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('3. merge: pre-existing server "other" preserved + new server added', () => {
  const dir = tmpDir('DATABASE_URL=postgresql://localhost/dev\n');
  const mcpPath = path.join(dir, '.mcp.json');
  // pre-existing .mcp.json
  fs.writeFileSync(mcpPath, JSON.stringify({
    mcpServers: { other: { command: 'node', args: ['other.js'] } },
  }, null, 2), 'utf8');

  try {
    const r = run(['postgres', 'pg-dev', '--env', path.join(dir, '.env.dev'), '--out', mcpPath]);
    assertEquals(r.status, 0, `exit code (stderr: ${r.stderr})`);

    const mcp = readMcp(dir);
    assertEquals(typeof mcp.mcpServers['other'], 'object', '"other" server preserved');
    assertEquals(typeof mcp.mcpServers['pg-dev'], 'object', '"pg-dev" server added');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('4. gitignore-ensure: no .gitignore → created with ".mcp.json" entry', () => {
  const dir = tmpDir('DATABASE_URL=postgresql://localhost/dev\n');
  try {
    const r = run(['postgres', 'pg-dev', '--env', path.join(dir, '.env.dev'), '--out', path.join(dir, '.mcp.json')]);
    assertEquals(r.status, 0, `exit code (stderr: ${r.stderr})`);

    const gi = fs.readFileSync(path.join(dir, '.gitignore'), 'utf8');
    assertContains(gi, '.mcp.json', '.gitignore contains .mcp.json');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('5. no raw secret leaks: realpass MUST NOT appear in .mcp.json', () => {
  const dir = tmpDir('DATABASE_URL=postgresql://u:realpass@prod-like-host/db\n');
  try {
    run(['postgres', 'pg-dev', '--env', path.join(dir, '.env.dev'), '--out', path.join(dir, '.mcp.json')]);
    const raw = fs.readFileSync(path.join(dir, '.mcp.json'), 'utf8');
    assertNotContains(raw, 'realpass', 'raw secret must not appear in .mcp.json');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('6. unknown datastore → exit 1', () => {
  const dir = tmpDir('FOO=bar\n');
  try {
    const r = run(['oracle', 'ora-dev', '--env', path.join(dir, '.env.dev'), '--out', path.join(dir, '.mcp.json')]);
    assertEquals(r.status, 1, 'exit code should be 1 for unknown datastore');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('7. clickhouse: CLICKHOUSE_URL parsed → env-refs in server entry', () => {
  const dir = tmpDir('CLICKHOUSE_URL=clickhouse://admin:pw@localhost:9000/analytics\n');
  try {
    const r = run(['clickhouse', 'ch-dev', '--env', path.join(dir, '.env.dev'), '--out', path.join(dir, '.mcp.json')]);
    assertEquals(r.status, 0, `exit code (stderr: ${r.stderr})`);

    const mcp = readMcp(dir);
    const server = mcp.mcpServers['ch-dev'];
    assertEquals(typeof server, 'object', 'clickhouse server exists');

    const mcpStr = JSON.stringify(mcp);
    assertContains(mcpStr, 'CLICKHOUSE', 'clickhouse env refs present');
    assertNotContains(mcpStr, 'pw', 'no raw password in clickhouse entry');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('8. s3: S3_ENDPOINT/S3_ACCESS_KEY_ID/S3_SECRET_ACCESS_KEY env-refs, region=auto', () => {
  const dir = tmpDir(
    'S3_ENDPOINT=https://accountid.r2.cloudflarestorage.com\n' +
    'S3_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE\n' +
    'S3_SECRET_ACCESS_KEY=wJalrXUtnFEMI/SECRET\n',
  );
  try {
    const r = run(['s3', 's3-dev', '--env', path.join(dir, '.env.dev'), '--out', path.join(dir, '.mcp.json')]);
    assertEquals(r.status, 0, `exit code (stderr: ${r.stderr})`);

    const mcp = readMcp(dir);
    const server = mcp.mcpServers['s3-dev'];
    assertEquals(typeof server, 'object', 's3 server exists');

    const mcpStr = JSON.stringify(mcp);
    assertContains(mcpStr, '${S3_ENDPOINT}', 'S3_ENDPOINT env-ref');
    assertContains(mcpStr, '${S3_ACCESS_KEY_ID}', 'S3_ACCESS_KEY_ID env-ref');
    assertContains(mcpStr, '${S3_SECRET_ACCESS_KEY}', 'S3_SECRET_ACCESS_KEY env-ref');
    assertContains(mcpStr, 'auto', 'S3_REGION auto value');
    assertNotContains(mcpStr, 'AKIAIOSFODNN7EXAMPLE', 'no raw access key');
    assertNotContains(mcpStr, 'wJalrXUtnFEMI', 'no raw secret key');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('9. missing env file → graceful error (exit 1), no crash', () => {
  const dir = tmpDir();
  try {
    const r = run(['postgres', 'pg-dev', '--env', path.join(dir, 'nonexistent.env'), '--out', path.join(dir, '.mcp.json')]);
    assertEquals(r.status, 1, 'exit code should be 1 for missing env file');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('10. idempotent: run twice same server-name → only one entry in mcpServers', () => {
  const dir = tmpDir('DATABASE_URL=postgresql://localhost/dev\n');
  const mcpPath = path.join(dir, '.mcp.json');
  const envPath = path.join(dir, '.env.dev');
  try {
    run(['postgres', 'pg-dev', '--env', envPath, '--out', mcpPath]);
    run(['postgres', 'pg-dev', '--env', envPath, '--out', mcpPath]);

    const mcp = readMcp(dir);
    const keys = Object.keys(mcp.mcpServers).filter(k => k === 'pg-dev');
    assertEquals(keys.length, 1, 'only one "pg-dev" entry after two runs');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
