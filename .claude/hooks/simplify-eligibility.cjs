#!/usr/bin/env node
/**
 * simplify-eligibility.cjs — Decide whether to offer /simplify in the chain.
 *
 * Reads .claude/.ck.json -> workflow.simplifyThreshold:
 *   { "maxLoc": 30, "maxFiles": 2 }
 * Counts ONLY code files in git diff HEAD (--numstat), excluding docs/plans/
 * reports/markdown/lockfiles — simplify never touches those, so counting them
 * inflated the gate. If code changes <= both thresholds, simplify is skipped.
 *
 * Output (stdout, always exit 0):
 *   {"eligible": true|false, "reason": "...", "stats": {"files": N, "loc": M}}
 *
 * Fail-open: any error (no git, no repo, parse fail) -> eligible=true so the
 * chain behaves normally.
 *
 * Contract: claude/rules/workflow-chaining.md (Conditional Skills -> simplify gate)
 */

'use strict';

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

const DEFAULT_MAX_LOC = 30;
const DEFAULT_MAX_FILES = 2;

function readConfig() {
  const root = process.env.CLAUDE_PROJECT_DIR || process.cwd();
  const cfgPath = path.join(root, '.claude', '.ck.json');
  try {
    const raw = fs.readFileSync(cfgPath, 'utf8');
    const cfg = JSON.parse(raw);
    const t = (cfg.workflow && cfg.workflow.simplifyThreshold) || {};
    return {
      maxLoc: Number.isFinite(t.maxLoc) ? t.maxLoc : DEFAULT_MAX_LOC,
      maxFiles: Number.isFinite(t.maxFiles) ? t.maxFiles : DEFAULT_MAX_FILES,
    };
  } catch {
    return { maxLoc: DEFAULT_MAX_LOC, maxFiles: DEFAULT_MAX_FILES };
  }
}

// Paths simplify never edits → excluded from the gate so they don't inflate it.
const NON_CODE_RE = /(^|\/)(plans|docs|reports)\//i;
const NON_CODE_EXT_RE = /\.(md|mdx|markdown|txt|lock|log)$/i;
const LOCKFILE_RE = /(^|\/)(package-lock\.json|pnpm-lock\.yaml|yarn\.lock|composer\.lock|go\.sum|Cargo\.lock)$/i;

function isCodeFile(p) {
  if (NON_CODE_RE.test(p) || NON_CODE_EXT_RE.test(p) || LOCKFILE_RE.test(p)) return false;
  return true;
}

function getDiffStats() {
  // --numstat: per-file "<added>\t<deleted>\t<path>" → filter to code files only.
  const out = execSync('git diff HEAD --numstat', {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  }).trim();
  if (!out) return { files: 0, loc: 0 };
  let files = 0;
  let loc = 0;
  for (const line of out.split('\n')) {
    const parts = line.split('\t');
    if (parts.length < 3) continue;
    const [added, deleted, ...rest] = parts;
    const filePath = rest.join('\t'); // path may contain tabs in rename notation
    if (!isCodeFile(filePath)) continue;
    files += 1;
    // Binary files show "-" for added/deleted → treat as 0 LOC.
    loc += (parseInt(added, 10) || 0) + (parseInt(deleted, 10) || 0);
  }
  return { files, loc };
}

function main() {
  const { maxLoc, maxFiles } = readConfig();
  let stats;
  try {
    stats = getDiffStats();
  } catch {
    process.stdout.write(JSON.stringify({
      eligible: true,
      reason: 'git diff unavailable (fail-open)',
      stats: { files: 0, loc: 0 },
    }) + '\n');
    process.exit(0);
  }

  if (stats.files === 0 && stats.loc === 0) {
    process.stdout.write(JSON.stringify({
      eligible: false,
      reason: 'no uncommitted changes detected',
      stats,
    }) + '\n');
    process.exit(0);
  }

  const tooSmall = stats.files <= maxFiles && stats.loc <= maxLoc;
  if (tooSmall) {
    process.stdout.write(JSON.stringify({
      eligible: false,
      reason: `changes too small (${stats.files} files / ${stats.loc} LOC <= ${maxFiles}/${maxLoc})`,
      stats,
    }) + '\n');
    process.exit(0);
  }

  process.stdout.write(JSON.stringify({
    eligible: true,
    reason: `${stats.files} files / ${stats.loc} LOC exceeds threshold`,
    stats,
  }) + '\n');
  process.exit(0);
}

main();
