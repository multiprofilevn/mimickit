'use strict';

/**
 * parse-memory.cjs — Frontmatter parser + dedup helper for consolidate-memory skill.
 *
 * CLI usage:
 *   node parse-memory.cjs parse <file-path>
 *     → prints JSON: { id, type, created, updated, tags, content } or { error }
 *
 *   node parse-memory.cjs dedup <json-string-of-fact-array>
 *     → prints JSON: { facts, exactRemoved, removedIds, possibleDupes }
 *
 * No external dependencies. Pure Node.js CJS.
 */

const fs = require('fs');
const path = require('path');
const { parseFrontmatter, toSlug, normalize } = require('./yaml-frontmatter.cjs');

// ---------------------------------------------------------------------------
// Parse command — read one memory file, extract typed fact
// ---------------------------------------------------------------------------

function cmdParse(filePath) {
  const resolved = path.resolve(filePath);
  let raw;
  try {
    raw = fs.readFileSync(resolved, 'utf8');
  } catch (e) {
    return { error: `Cannot read file: ${e.message}` };
  }

  let frontmatter, body;
  try {
    ({ frontmatter, body } = parseFrontmatter(raw));
  } catch (e) {
    return { error: `Frontmatter parse failed: ${e.message}` };
  }

  if (!frontmatter) return { error: 'No frontmatter found' };

  const meta = frontmatter.metadata || {};
  const type = meta.type || frontmatter.type || null;
  if (!type) return { error: 'Missing metadata.type' };

  return {
    id: meta.id || path.basename(resolved, '.md'),
    type,
    created: meta.created || frontmatter.created || null,
    updated: meta.updated || frontmatter.updated || null,
    tags: Array.isArray(meta.tags) ? meta.tags : [],
    content: body,
  };
}

// ---------------------------------------------------------------------------
// Dedup command — exact + heuristic dedup on a fact array
// ---------------------------------------------------------------------------

function cmdDedup(jsonStr) {
  let facts;
  try {
    facts = JSON.parse(jsonStr);
  } catch (e) {
    return { error: `Invalid JSON input: ${e.message}` };
  }

  if (!Array.isArray(facts)) return { error: 'Input must be a JSON array' };

  // --- Exact dedup: normalize content, keep most-recently updated copy ---
  const exactSeen = new Map(); // normalizedContent -> index of best
  const exactRemoved = [];

  // Facts with no content can't be deduplicated by content — keep them as-is
  // (never silently drop a real fact; that would violate the fail-open contract).
  const keptNoContent = new Set();

  for (let i = 0; i < facts.length; i++) {
    const f = facts[i];
    const key = normalize(f.content || '');
    if (!key) { keptNoContent.add(i); continue; }

    if (exactSeen.has(key)) {
      const prevIdx = exactSeen.get(key);
      const prev = facts[prevIdx];
      if ((f.updated || '') > (prev.updated || '')) {
        exactRemoved.push(prev.id || prevIdx);
        exactSeen.set(key, i);
      } else {
        exactRemoved.push(f.id || i);
      }
    } else {
      exactSeen.set(key, i);
    }
  }

  const keptIndices = new Set([...exactSeen.values(), ...keptNoContent]);
  const deduped = facts.filter((_, i) => keptIndices.has(i));

  // --- Heuristic dedup: same id/title slug → flag as possibleDupe, do NOT remove ---
  const slugMap = new Map();
  for (const f of deduped) {
    const firstLine = (f.content || '').split('\n')[0];
    const slug = toSlug(f.id || firstLine);
    if (!slugMap.has(slug)) slugMap.set(slug, []);
    slugMap.get(slug).push(f);
  }

  const possibleDupes = [];
  for (const [slug, group] of slugMap.entries()) {
    if (group.length < 2) continue;
    for (let a = 0; a < group.length - 1; a++) {
      for (let b = a + 1; b < group.length; b++) {
        possibleDupes.push({
          slug,
          a: group[a].id || '?',
          b: group[b].id || '?',
          reason: 'same title slug',
        });
      }
    }
  }

  return { facts: deduped, exactRemoved: exactRemoved.length, removedIds: exactRemoved, possibleDupes };
}

// ---------------------------------------------------------------------------
// CLI entry
// ---------------------------------------------------------------------------

function main() {
  const [, , cmd, ...rest] = process.argv;

  if (cmd === 'parse') {
    if (!rest[0]) { process.stderr.write('Usage: parse-memory.cjs parse <file-path>\n'); process.exit(1); }
    const result = cmdParse(rest[0]);
    process.stdout.write(JSON.stringify(result, null, 2) + '\n');
    if (result.error) process.exit(1);
    return;
  }

  if (cmd === 'dedup') {
    if (!rest[0]) { process.stderr.write('Usage: parse-memory.cjs dedup <json-facts-array>\n'); process.exit(1); }
    const result = cmdDedup(rest[0]);
    process.stdout.write(JSON.stringify(result, null, 2) + '\n');
    if (result.error) process.exit(1);
    return;
  }

  process.stderr.write('Usage: parse-memory.cjs <parse|dedup> <args>\n');
  process.exit(1);
}

module.exports = { cmdParse, cmdDedup };
if (require.main === module) main();
