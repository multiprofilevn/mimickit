'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const HOOK = path.resolve(__dirname, '..', 'post-write-typecheck.cjs');

function runHook(payload, env = {}) {
  return spawnSync(process.execPath, [HOOK], {
    input: JSON.stringify(payload),
    encoding: 'utf8',
    env: { ...process.env, ...env }
  });
}

function makeWriteEvent(filePath) {
  return {
    tool_name: 'Write',
    tool_input: { file_path: filePath, content: 'x' }
  };
}

function makeEditEvent(filePath) {
  return {
    tool_name: 'Edit',
    tool_input: { file_path: filePath, old_string: 'a', new_string: 'b' }
  };
}

function makeMultiEditEvent(filePath) {
  return {
    tool_name: 'MultiEdit',
    tool_input: {
      edits: [{ file_path: filePath, old_string: 'a', new_string: 'b' }]
    }
  };
}

// ---- bypass / skip tests ----

test('exits 0 silently when CK_TYPECHECK_DISABLED=1', () => {
  const r = runHook(makeWriteEvent('/fake/file.ts'), { CK_TYPECHECK_DISABLED: '1' });
  assert.equal(r.status, 0);
  assert.equal(r.stderr.trim(), '');
});

test('exits 0 silently for unsupported extension (.md)', () => {
  const r = runHook(makeWriteEvent('/fake/README.md'));
  assert.equal(r.status, 0);
  assert.equal(r.stderr.trim(), '');
});

test('exits 0 silently for unsupported extension (.json)', () => {
  const r = runHook(makeWriteEvent('/fake/config.json'));
  assert.equal(r.status, 0);
  assert.equal(r.stderr.trim(), '');
});

test('exits 0 when payload has no tool_input', () => {
  const r = runHook({ tool_name: 'Write' });
  assert.equal(r.status, 0);
});

test('exits 0 when payload is empty', () => {
  const r = runHook({});
  assert.equal(r.status, 0);
});

test('exits 0 when stdin is empty/malformed JSON', () => {
  const r = spawnSync(process.execPath, [HOOK], { input: '', encoding: 'utf8' });
  assert.equal(r.status, 0);
});

// ---- latency guard: no config nearby ----

test('exits 0 silently for .ts file with no tsconfig.json anywhere nearby', () => {
  // Use a temp dir with no tsconfig — walk will reach filesystem root and find nothing
  // on systems that don't have a tsconfig at root. We place file inside a deep temp path.
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ck-tc-noconfig-'));
  const fakeFile = path.join(tmp, 'src', 'util.ts');
  const r = runHook(makeWriteEvent(fakeFile));
  assert.equal(r.status, 0);
  // No warning expected because no tsconfig found
  // (If tsc is installed and happens to find a global tsconfig this assertion may vary —
  //  the hook still exits 0 either way; we only assert non-blocking.)
  fs.rmSync(tmp, { recursive: true, force: true });
});

test('exits 0 silently for .py file with no mypy/pyright config nearby', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ck-tc-noconfig-'));
  const fakeFile = path.join(tmp, 'app.py');
  const r = runHook(makeWriteEvent(fakeFile));
  assert.equal(r.status, 0);
  fs.rmSync(tmp, { recursive: true, force: true });
});

// ---- fail-soft: warn but never block ----

test('CORE: always exits 0 (non-blocking) for .ts file with tsconfig present, even if tsc errors', () => {
  // Create a temp dir with a tsconfig.json and a .ts file that has a type error.
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ck-tc-tserr-'));
  const tsconfig = {
    compilerOptions: { strict: true, noEmit: true, target: 'ES2020' }
  };
  fs.writeFileSync(path.join(tmp, 'tsconfig.json'), JSON.stringify(tsconfig));

  // Write a .ts file with a deliberate type error
  const tsFile = path.join(tmp, 'bad.ts');
  fs.writeFileSync(tsFile, 'const x: number = "not a number";\n');

  const r = runHook(makeWriteEvent(tsFile));

  // MUST exit 0 regardless — fail-soft
  assert.equal(r.status, 0, `hook must not block: stderr=${r.stderr}`);

  // If tsc is installed, we expect a warning in stderr; if not installed → silent skip.
  // Either way: no block.
  if (r.stderr.includes('[post-write-typecheck] WARNING')) {
    assert.match(r.stderr, /TypeScript errors/);
    assert.match(r.stderr, /bad\.ts/);
  }

  fs.rmSync(tmp, { recursive: true, force: true });
});

test('CORE: exits 0 for Edit event on .ts file with tsconfig (non-blocking)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ck-tc-edit-'));
  fs.writeFileSync(path.join(tmp, 'tsconfig.json'), JSON.stringify({
    compilerOptions: { strict: true, noEmit: true }
  }));
  const tsFile = path.join(tmp, 'foo.ts');
  fs.writeFileSync(tsFile, 'const n: string = 42;\n');

  const r = runHook(makeEditEvent(tsFile));
  assert.equal(r.status, 0);

  fs.rmSync(tmp, { recursive: true, force: true });
});

test('CORE: exits 0 for MultiEdit event on .ts file (non-blocking)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ck-tc-medit-'));
  fs.writeFileSync(path.join(tmp, 'tsconfig.json'), JSON.stringify({
    compilerOptions: { strict: true, noEmit: true }
  }));
  const tsFile = path.join(tmp, 'bar.ts');
  fs.writeFileSync(tsFile, 'export const x: number = "oops";\n');

  const r = runHook(makeMultiEditEvent(tsFile));
  assert.equal(r.status, 0);

  fs.rmSync(tmp, { recursive: true, force: true });
});

// ---- hook disable via .ck.json ----

test('exits 0 silently when hook disabled via .ck.json (hooks.post-write-typecheck=false)', () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'ck-tc-disabled-'));

  // Write tsconfig so latency guard would normally allow the check
  fs.writeFileSync(path.join(tmp, 'tsconfig.json'), JSON.stringify({
    compilerOptions: { strict: true, noEmit: true }
  }));
  const tsFile = path.join(tmp, 'file.ts');
  fs.writeFileSync(tsFile, 'const x: number = "bad";\n');

  // Write .ck.json disabling the hook — must be in cwd where loadConfig reads it.
  // loadConfig reads from LOCAL_CONFIG_PATH = 'claude/.ck.json' relative to cwd.
  // Hook runs with default cwd (process.cwd()). We override cwd to our tmp dir.
  fs.mkdirSync(path.join(tmp, 'claude'), { recursive: true });
  fs.writeFileSync(path.join(tmp, 'claude', '.ck.json'), JSON.stringify({
    hooks: { 'post-write-typecheck': false }
  }));

  const r = spawnSync(process.execPath, [HOOK], {
    input: JSON.stringify(makeWriteEvent(tsFile)),
    encoding: 'utf8',
    cwd: tmp,
    env: process.env
  });

  assert.equal(r.status, 0);
  assert.equal(r.stderr.trim(), '', 'should be silent when disabled via .ck.json');

  fs.rmSync(tmp, { recursive: true, force: true });
});
