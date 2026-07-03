/**
 * Tests for consolidate-memory/scripts/parse-memory.cjs
 * Run: node .claude/hooks/__tests__/parse-memory.test.cjs
 *
 * Scenarios:
 * 1. parseFrontmatter — valid frontmatter returns correct fields
 * 2. parseFrontmatter — no frontmatter returns null fm + full body
 * 3. cmdParse — file with valid metadata.type returns fact object
 * 4. cmdParse — file missing metadata.type returns { error }
 * 5. cmdParse — nonexistent file returns { error }
 * 6. cmdDedup — exact duplicates: keeps most-recent, reports count
 * 7. cmdDedup — heuristic: same slug flagged as possibleDupe, NOT removed
 * 8. cmdDedup — distinct entries: zero removed, zero possibleDupes
 * 9. cmdDedup — invalid JSON input returns { error }
 * 10. CLI parse command — spawned, exit 0, stdout is valid JSON
 * 11. CLI dedup command — spawned, exit 0, stdout is valid JSON
 */

'use strict';

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

const SCRIPT = path.resolve(
  __dirname,
  '../../skills/consolidate-memory/scripts/parse-memory.cjs',
);
const YAML_MOD = path.resolve(
  __dirname,
  '../../skills/consolidate-memory/scripts/yaml-frontmatter.cjs',
);

// Require modules directly for unit tests
const { parseFrontmatter } = require(YAML_MOD);
const { cmdParse, cmdDedup } = require(SCRIPT);

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

function run(args) {
  return spawnSync(process.execPath, [SCRIPT, ...args], { encoding: 'utf8' });
}

function writeTmp(content) {
  const f = path.join(os.tmpdir(), `pm-test-${Date.now()}-${Math.random().toString(36).slice(2)}.md`);
  fs.writeFileSync(f, content, 'utf8');
  return f;
}

// ---------------------------------------------------------------------------
// Unit tests (module-level)
// ---------------------------------------------------------------------------

test('1. parseFrontmatter — valid frontmatter returns correct fields', () => {
  const raw = `---
metadata:
  type: project
  id: my-fact
  created: 2024-01-01
  updated: 2024-06-01
  tags: [foo, bar]
---
Body content here.`;

  const { frontmatter, body } = parseFrontmatter(raw);
  assertEquals(frontmatter.metadata.type, 'project', 'type');
  assertEquals(frontmatter.metadata.id, 'my-fact', 'id');
  assertEquals(body, 'Body content here.', 'body');
  assertEquals(Array.isArray(frontmatter.metadata.tags), true, 'tags is array');
  assertEquals(frontmatter.metadata.tags.length, 2, 'tags length');
});

test('2. parseFrontmatter — no frontmatter returns null fm + full body', () => {
  const raw = 'Just plain markdown\nno frontmatter here.';
  const { frontmatter, body } = parseFrontmatter(raw);
  assertEquals(frontmatter, null, 'frontmatter is null');
  assertEquals(body, raw, 'body is full raw content');
});

test('3. cmdParse — file with valid metadata.type returns fact object', () => {
  const f = writeTmp(`---
metadata:
  type: user
  id: pref-dark-mode
  updated: 2024-05-01
  tags: [ui]
---
User prefers dark mode.`);
  try {
    const result = cmdParse(f);
    assertEquals(result.type, 'user', 'type');
    assertEquals(result.id, 'pref-dark-mode', 'id');
    assertContains(result.content, 'dark mode', 'content');
    assertEquals(result.error, undefined, 'no error');
  } finally {
    fs.unlinkSync(f);
  }
});

test('4. cmdParse — file missing metadata.type returns { error }', () => {
  const f = writeTmp(`---
metadata:
  id: no-type-here
---
Content without type.`);
  try {
    const result = cmdParse(f);
    assertEquals(typeof result.error, 'string', 'error is string');
    assertContains(result.error, 'metadata.type', 'error mentions type');
  } finally {
    fs.unlinkSync(f);
  }
});

test('5. cmdParse — nonexistent file returns { error }', () => {
  const result = cmdParse('/tmp/does-not-exist-xyz.md');
  assertEquals(typeof result.error, 'string', 'error is string');
  assertContains(result.error, 'Cannot read file', 'error mentions read failure');
});

test('6. cmdDedup — exact duplicates: keeps most-recent, reports count', () => {
  const facts = [
    { id: 'a', type: 'project', content: 'same content here', updated: '2024-01-01', tags: [] },
    { id: 'b', type: 'project', content: 'same content here', updated: '2024-06-01', tags: [] },
  ];
  const result = cmdDedup(JSON.stringify(facts));
  assertEquals(result.error, undefined, 'no error');
  assertEquals(result.facts.length, 1, 'one fact kept');
  assertEquals(result.exactRemoved, 1, 'one exact dupe removed');
  // The more recent one (b) is kept
  assertEquals(result.facts[0].id, 'b', 'kept the most recent');
});

test('7. cmdDedup — heuristic: same slug flagged as possibleDupe, NOT removed', () => {
  const facts = [
    { id: 'fact-about-auth', type: 'project', content: 'Auth strategy: JWT', updated: '2024-01-01', tags: [] },
    { id: 'fact-about-auth', type: 'project', content: 'Auth strategy: OAuth2', updated: '2024-02-01', tags: [] },
  ];
  const result = cmdDedup(JSON.stringify(facts));
  assertEquals(result.error, undefined, 'no error');
  // Different content → not exact deduped
  assertEquals(result.facts.length, 2, 'both facts kept (heuristic does not remove)');
  assertEquals(result.possibleDupes.length >= 1, true, 'possibleDupe flagged');
  assertEquals(result.possibleDupes[0].reason, 'same title slug', 'reason is same title slug');
});

test('8. cmdDedup — distinct entries: zero removed, zero possibleDupes', () => {
  const facts = [
    { id: 'fact-alpha', type: 'user', content: 'User prefers dark mode', updated: '2024-01-01', tags: [] },
    { id: 'fact-beta', type: 'feedback', content: 'Keep responses concise', updated: '2024-02-01', tags: [] },
  ];
  const result = cmdDedup(JSON.stringify(facts));
  assertEquals(result.error, undefined, 'no error');
  assertEquals(result.facts.length, 2, 'both facts kept');
  assertEquals(result.exactRemoved, 0, 'zero exact removed');
  assertEquals(result.possibleDupes.length, 0, 'zero possible dupes');
});

test('9. cmdDedup — invalid JSON input returns { error }', () => {
  const result = cmdDedup('not valid json {{');
  assertEquals(typeof result.error, 'string', 'error is string');
  assertContains(result.error, 'Invalid JSON', 'error mentions JSON');
});

test('9b. cmdDedup — empty-content fact is KEPT, not silently dropped (M1)', () => {
  const facts = [
    { id: 'empty', type: 'project', content: '', updated: '2024-01-01', tags: [] },
    { id: 'real', type: 'user', content: 'Has content', updated: '2024-01-01', tags: [] },
  ];
  const result = cmdDedup(JSON.stringify(facts));
  assertEquals(result.facts.length, 2, 'empty-content fact survives dedup');
  assertEquals(result.facts.some(f => f.id === 'empty'), true, 'empty fact present');
});

test('9c. parseYamlValue — mismatched quotes are NOT stripped (M2)', () => {
  const { frontmatter } = parseFrontmatter('---\ntitle: \'mismatched"\n---\nbody');
  assertEquals(frontmatter.title, '\'mismatched"', 'mismatched quotes preserved verbatim');
});

// ---------------------------------------------------------------------------
// CLI spawn tests
// ---------------------------------------------------------------------------

test('10. CLI parse command — exit 0, stdout is valid JSON', () => {
  const f = writeTmp(`---
metadata:
  type: reference
  id: cli-test-ref
  updated: 2024-03-01
  tags: []
---
Some reference content.`);
  try {
    const r = run(['parse', f]);
    assertEquals(r.status, 0, 'exit code');
    const parsed = JSON.parse(r.stdout);
    assertEquals(parsed.type, 'reference', 'type');
    assertEquals(parsed.id, 'cli-test-ref', 'id');
  } finally {
    fs.unlinkSync(f);
  }
});

test('11. CLI dedup command — exit 0, stdout is valid JSON', () => {
  const facts = [
    { id: 'x', type: 'project', content: 'unique fact', updated: '2024-01-01', tags: [] },
  ];
  const r = run(['dedup', JSON.stringify(facts)]);
  assertEquals(r.status, 0, 'exit code');
  const out = JSON.parse(r.stdout);
  assertEquals(Array.isArray(out.facts), true, 'facts is array');
  assertEquals(out.exactRemoved, 0, 'no exact removed');
});

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
