#!/usr/bin/env node
/**
 * Tests for secrets-scanner-pre-commit.cjs
 * Run: node --test .claude/hooks/__tests__/secrets-scanner-pre-commit.test.cjs
 */

'use strict';

const test = require('node:test');
const assert = require('node:assert');
const { spawnSync, execFileSync } = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const HOOK = path.resolve(__dirname, '..', 'secrets-scanner-pre-commit.cjs');

// ─── Helpers ─────────────────────────────────────────────────────────────────

/** Create a minimal git repo in a temp dir. Returns the dir path. */
function makeRepo(ckJson = {}) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'secrets-scanner-'));
  const git = (...args) => execFileSync('git', args, { cwd: dir, stdio: 'ignore' });

  git('init', '--initial-branch=main');
  git('config', 'user.email', 'test@example.com');
  git('config', 'user.name', 'Test');

  // Initial commit so HEAD exists
  fs.writeFileSync(path.join(dir, 'README.md'), '# test\n');

  // Write .claude/.ck.json if needed (ck-config-utils uses LOCAL_CONFIG_PATH = '.claude/.ck.json')
  if (Object.keys(ckJson).length > 0) {
    fs.mkdirSync(path.join(dir, '.claude'), { recursive: true });
    fs.writeFileSync(
      path.join(dir, '.claude', '.ck.json'),
      JSON.stringify(ckJson)
    );
    git('add', '.');
  } else {
    git('add', 'README.md');
  }
  git('commit', '-m', 'init');

  return dir;
}

/** Run the hook with a given Bash command payload, from the given cwd. */
function runHook(command, cwd) {
  const payload = {
    tool_name: 'Bash',
    tool_input: { command },
  };
  return spawnSync(process.execPath, [HOOK], {
    input: JSON.stringify(payload),
    encoding: 'utf8',
    cwd: cwd || process.cwd(),
  });
}

/** Stage a file with given content in the repo. */
function stageFile(dir, filename, content) {
  fs.writeFileSync(path.join(dir, filename), content);
  execFileSync('git', ['add', filename], { cwd: dir, stdio: 'ignore' });
}

// ─── Tests: command detection ────────────────────────────────────────────────

test('passes through: git status (not commit/add)', () => {
  const dir = makeRepo();
  const r = runHook('git status', dir);
  assert.strictEqual(r.status, 0, 'git status should not be blocked');
  assert.strictEqual(r.stderr, '', 'no stderr for allowed command');
});

test('passes through: git log', () => {
  const dir = makeRepo();
  const r = runHook('git log --oneline', dir);
  assert.strictEqual(r.status, 0);
});

test('passes through: git push', () => {
  const dir = makeRepo();
  const r = runHook('git push origin main', dir);
  assert.strictEqual(r.status, 0);
});

test('passes through: git checkout', () => {
  const dir = makeRepo();
  const r = runHook('git checkout -b feature', dir);
  assert.strictEqual(r.status, 0);
});

// ─── Tests: git commit triggers scan ─────────────────────────────────────────

test('blocks git commit when staged file contains fake AWS key', () => {
  const dir = makeRepo();
  stageFile(dir, 'config.js', "const key = 'AKIAIOSFODNN7EXAMPLE';\n");
  const r = runHook('git commit -m "add config"', dir);
  assert.strictEqual(r.status, 2, 'hook must exit 2 when secret detected');
  assert.ok(r.stderr.includes('[secrets-scanner] BLOCKED'), 'stderr should contain BLOCKED');
  assert.ok(r.stderr.includes('AWS Access Key'), 'stderr should name the pattern type');
});

test('blocks git commit when staged file contains JWT', () => {
  const dir = makeRepo();
  const fakeJwt = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c';
  stageFile(dir, 'auth.js', `const token = '${fakeJwt}';\n`);
  const r = runHook('git commit -m "add auth"', dir);
  assert.strictEqual(r.status, 2);
  assert.ok(r.stderr.includes('JWT'));
});

test('blocks git commit when staged file contains PASSWORD= assignment', () => {
  const dir = makeRepo();
  stageFile(dir, 'config.env', 'DATABASE_PASSWORD=supersecretpass123\n');
  const r = runHook('git commit -m "add config"', dir);
  assert.strictEqual(r.status, 2);
  assert.ok(r.stderr.includes('PASSWORD assignment') || r.stderr.includes('BLOCKED'));
});

test('blocks git commit when .env file is staged', () => {
  const dir = makeRepo();
  stageFile(dir, '.env', 'API_KEY=test\nDB_PASS=localonly\n');
  const r = runHook('git commit -m "oops"', dir);
  assert.strictEqual(r.status, 2);
  assert.ok(r.stderr.includes('BLOCKED'));
  assert.ok(r.stderr.includes('.env'));
});

// ─── Tests: git add triggers scan ────────────────────────────────────────────

test('blocks git add when staged diff contains private key block', () => {
  const dir = makeRepo();
  stageFile(dir, 'deploy.pem', '-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----\n');
  const r = runHook('git add deploy.pem', dir);
  assert.strictEqual(r.status, 2);
  assert.ok(r.stderr.includes('Private Key'));
});

// ─── Tests: clean content passes ─────────────────────────────────────────────

test('allows git commit when staged file has no secrets', () => {
  const dir = makeRepo();
  stageFile(dir, 'hello.js', 'console.log("hello world");\n');
  const r = runHook('git commit -m "hello"', dir);
  assert.strictEqual(r.status, 0, 'clean file should not be blocked');
});

test('allows .env.example (template file, not a secret)', () => {
  const dir = makeRepo();
  stageFile(dir, '.env.example', 'API_KEY=your-key-here\nDB_URL=your-db-url\n');
  const r = runHook('git commit -m "add example"', dir);
  // .env.example should not trigger .env block (pattern only matches .env, .env.* not .env.example)
  // also no real secrets in the content — PASSWORD= value is too short ("your-key-here" has no match)
  assert.strictEqual(r.status, 0);
});

// ─── Tests: test-fixture path exemption ──────────────────────────────────────

test('allows fake secret inside a *.test.cjs fixture (path-exempt)', () => {
  const dir = makeRepo();
  fs.mkdirSync(path.join(dir, '__tests__'), { recursive: true });
  stageFile(dir, '__tests__/secret.test.cjs', "const key = 'AKIAIOSFODNN7EXAMPLE';\n");
  const r = runHook('git commit -m "add test"', dir);
  assert.strictEqual(r.status, 0, 'fake secret in a test fixture must not block');
});

test('still blocks a real secret in a non-test file alongside test fixtures', () => {
  const dir = makeRepo();
  stageFile(dir, 'prod-config.js', "const key = 'AKIAIOSFODNN7EXAMPLE';\n");
  const r = runHook('git commit -m "add config"', dir);
  assert.strictEqual(r.status, 2, 'exemption must not leak to production files');
});

// ─── Tests: allowlist bypass ──────────────────────────────────────────────────

test('allows git commit when finding matches allowlist pattern', () => {
  const ckJson = {
    hooks: {
      'secrets-scanner-pre-commit': {
        allow: ['AKIAIOSFODNN7EXAMPLE'],
      },
    },
  };
  const dir = makeRepo(ckJson);
  stageFile(dir, 'test-fixture.js', "const key = 'AKIAIOSFODNN7EXAMPLE';\n");
  const r = runHook('git commit -m "add test fixture"', dir);
  assert.strictEqual(r.status, 0, 'allowlisted pattern should not block');
});

// ─── Tests: hook disable ──────────────────────────────────────────────────────

test('passes through when hook disabled in .ck.json', () => {
  const ckJson = {
    hooks: {
      'secrets-scanner-pre-commit': false,
    },
  };
  const dir = makeRepo(ckJson);
  stageFile(dir, 'config.js', "const key = 'AKIAIOSFODNN7EXAMPLE';\n");
  const r = runHook('git commit -m "add config"', dir);
  assert.strictEqual(r.status, 0, 'disabled hook must not block');
});

// ─── Tests: chained commands ──────────────────────────────────────────────────

test('blocks when git commit is part of a chained command with a secret staged', () => {
  const dir = makeRepo();
  stageFile(dir, 'secrets.js', "const token = 'AKIAIOSFODNN7EXAMPLE';\n");
  const r = runHook('echo "staging" && git commit -m "deploy"', dir);
  assert.strictEqual(r.status, 2);
});

test('passes when chain contains non-commit git commands only', () => {
  const dir = makeRepo();
  const r = runHook('git fetch origin && git status', dir);
  assert.strictEqual(r.status, 0);
});
