# Skills Catalog

Auto-generated catalog of all available skills in ClaudeKit Engineer.

**Last Updated**: 2026-06-02

**Total Skills**: 66

## Categories

- [AI & Machine Learning](#ai-ml)
- [Frontend & Design](#frontend)
- [Backend Development](#backend)
- [Infrastructure & DevOps](#infrastructure)
- [Database & Storage](#database)
- [Development Tools](#dev-tools)
- [Multimedia & Processing](#multimedia)
- [Frameworks & Platforms](#frameworks)
- [Utilities & Helpers](#utilities)
- [Other](#other)

## Legend

- 📦 Has executable scripts
- 📚 Has reference documentation

## AI & Machine Learning

### 📦 📚 `ai-multimodal`

Analyze images/audio/video with Gemini API (better vision than Claude). Generate images (Imagen 4, Nano Banana 2, MiniMax), videos (Veo 3, Hailuo), speech (MiniMax TTS), music (MiniMax). Use for vision analysis, transcription, OCR, design extraction, multimodal AI.

**Location**: `.claude/skills/ai-multimodal\SKILL.md`

## Frontend & Design

### `db-design`

Design or extend database schemas following per-library conventions (Prisma, Mongoose, Sequelize, TypeORM, Drizzle, raw SQL). Enforces migration safety — always run the ORM's migrate command, never hand-write migration files. Use when designing new tables/collections, adding columns, changing relations, or writing DDL.

**Location**: `.claude/skills/db-design\SKILL.md`

### 📚 `frontend-design`

Create polished frontend interfaces from designs/screenshots/videos. Use for web components, 3D experiences, replicating UI designs, quick prototypes, immersive interfaces, avoiding AI slop.

**Location**: `.claude/skills/frontend-design\SKILL.md`

### `frontend-development`

Build React/TypeScript frontends with modern patterns. Use for components, Suspense, lazy loading, useSuspenseQuery, MUI v7 styling, TanStack Router, performance optimization.

**Location**: `.claude/skills/frontend-development\SKILL.md`

### `react-best-practices`

Apply React and Next.js performance optimization patterns from Vercel Engineering. Use for component optimization, rendering performance, bundle analysis.

**Location**: `.claude/skills/react-best-practices\SKILL.md`

### 📦 📚 `ui-styling`

Style UIs with shadcn/ui components (Radix UI + Tailwind CSS). Use for accessible components, themes, dark mode, responsive layouts, design systems, color customization.

**Location**: `.claude/skills/ui-styling\SKILL.md`

### 📦 `ui-ux-pro-max`

UI/UX design intelligence for web and mobile: style selection, color systems, typography, layout, accessibility, interaction states, responsive behavior, forms, charts, design systems, and code review across React, Next.js, Vue, Svelte, SwiftUI, React Native, Flutter, Tailwind, shadcn/ui, and HTML/CSS.

**Location**: `.claude/skills/ui-ux-pro-max\SKILL.md`

### `web-design-guidelines`

Review UI code for Web Interface Guidelines compliance. Use when asked to "review my UI", "check accessibility", "audit design", "review UX", or "check my site against best practices".

**Location**: `.claude/skills/web-design-guidelines\SKILL.md`

## Backend Development

### 📚 `backend-development`

Build backends with Node.js, Python, Go (NestJS, FastAPI, Django). Use for REST/GraphQL/gRPC APIs, auth (OAuth, JWT), databases, microservices, security (OWASP), Docker/K8s.

**Location**: `.claude/skills/backend-development\SKILL.md`

### 📚 `go-backend`

Build Go backends with Echo + GORM + Redis. Use for REST APIs, handlers, middleware, database queries, caching, authentication, project structure.

**Location**: `.claude/skills/go-backend\SKILL.md`

### 📚 `node-backend`

Build Node.js backends with NestJS + Prisma + Mongoose + Redis. Use for REST APIs, modules, services, DTOs, guards, interceptors, database queries, caching, authentication.

**Location**: `.claude/skills/node-backend\SKILL.md`

## Infrastructure & DevOps

### 📚 `deploy`

Deploy projects to any platform with auto-detection. Use when user says "deploy", "publish", "ship", "go live", "push to production", "host this app", or mentions any hosting platform (Vercel, Netlify, Cloudflare, Railway, Fly.io, Render, Heroku, TOSE, Github Pages, AWS, GCP, Digital Ocean, Vultr, Coolify, Dokploy). Auto-detects deployment target from config files and docs/deployment.md.

**Location**: `.claude/skills/deploy\SKILL.md`

### 📦 📚 `devops`

Deploy to Cloudflare (Workers, R2, D1), Docker, GCP (Cloud Run, GKE), Kubernetes (kubectl, Helm). Use for serverless, containers, CI/CD, GitOps, security audit.

**Location**: `.claude/skills/devops\SKILL.md`

## Database & Storage

### 📦 📚 `databases`

Design schemas, write queries for MongoDB and PostgreSQL. Use for database design, SQL/NoSQL queries, aggregation pipelines, indexes, migrations, replication, performance optimization, psql CLI.

**Location**: `.claude/skills/databases\SKILL.md`

## Development Tools

### 📚 `agentize`

Convert a codebase, feature, or module into an AI-agent-friendly CLI and/or MCP server. Covers npm packaging, stdio/SSE/Streamable HTTP surfaces, credential resolution, docs, tests, CI, and a companion Claude skill for users who need an existing capability exposed as a reusable agent tool.

**Location**: `.claude/skills/agentize\SKILL.md`

### 📦 📚 `docs-seeker`

Search library/framework documentation via llms.txt (context7.com). Use for API docs, GitHub repository analysis, technical documentation lookup, latest library features.

**Location**: `.claude/skills/docs-seeker\SKILL.md`

### 📚 `git`

Git operations with conventional commits. Use for staging, committing, pushing, PRs, merges. Auto-splits commits by type/scope. Security scans for secrets.

**Location**: `.claude/skills/git\SKILL.md`

### `graphify`

Build queryable knowledge graphs from code, docs, papers, and images. Use for codebase understanding, architecture analysis, cross-file relationship discovery, token-efficient navigation.

**Location**: `.claude/skills/graphify\SKILL.md`

### 📦 `plans-kanban`

Open the ClaudeKit plans dashboard in the CLI config UI. Use for plan kanban views, progress tracking, timeline checks, and quick navigation into plan files.

**Location**: `.claude/skills/plans-kanban\SKILL.md`

### 📦 📚 `repomix`

Pack repositories into AI-friendly files with Repomix (XML, Markdown, plain text). Use for new-project onboarding, codebase snapshots, LLM context preparation, security audits, third-party library analysis.

**Location**: `.claude/skills/repomix\SKILL.md`

### 📚 `scout`

Fast codebase scouting using parallel agents. Use for file discovery, task context gathering, quick searches across directories. Supports internal (Explore) and external (Gemini/OpenCode) agents.

**Location**: `.claude/skills/scout\SKILL.md`

### 📚 `ship`

Ship pipeline: merge main, test, review, commit, push, PR. Single command from feature branch to PR URL. Use for shipping official releases to main/master or beta releases to dev/beta branches.

**Location**: `.claude/skills/ship\SKILL.md`

### 📦 📚 `skill-creator`

Create or update Claude skills with eval-driven iteration. Use for new skills, skill scripts, references, benchmark optimization, description optimization, eval testing, extending Claude's capabilities.

**Location**: `.claude/skills/skill-creator\SKILL.md`

### 📦 📚 `use-mcp`

Discover and execute MCP server tools. Two execution paths: Gemini CLI (LLM-driven, all tasks) or direct scripts (deterministic, specific tool/server). Use for MCP integrations, tool execution, capability discovery, persistent tool catalog.

**Location**: `.claude/skills/use-mcp\SKILL.md`

### 📦 📚 `web-testing`

Web testing with Playwright, Vitest, k6. E2E/unit/integration/load/security/visual/a11y testing. Use for test automation, flakiness, Core Web Vitals, mobile gestures, cross-browser.

**Location**: `.claude/skills/web-testing\SKILL.md`

### 📦 `worktree`

Create, inspect, and clean isolated git worktrees. Use for feature isolation, worktree health audits, stale cleanup, and monorepo or submodule workflows.

**Location**: `.claude/skills/worktree\SKILL.md`

### 📚 `xia`

Extract, compare, port, or adapt a feature from a GitHub repository or local repo path into the current project. Use when the user wants to copy behavior from another repo, study how another codebase implements something, compare implementations, or rewrite a feature in the local stack. Triggers on: 'port from', 'copy from repo', 'like how X does it', 'clone feature from', 'adapt from', 'bring feature from', 'borrow from', 'take from repo', 'xia', 'xi a', 'xia feature'.

**Location**: `.claude/skills/xia\SKILL.md`

## Multimedia & Processing

### `browser`

Drive a browser via Playwright or Puppeteer with stealth plugins enabled. Picks the library based on what's installed (or user preference), auto-installs missing deps, hides automation fingerprints by default. Use when the user says: browser, scrape, crawl, automate, stealth, bypass bot detection, headless, login, screenshot, capture api.

**Location**: `.claude/skills/browser\SKILL.md`

### 📦 `docx`

Create, edit, analyze .docx Word documents. Use for document creation, tracked changes, comments, formatting preservation, text extraction, template modification.

**Location**: `.claude/skills/document-skills\docx\SKILL.md`

### 📦 📚 `media-processing`

Process media with FFmpeg (video/audio), ImageMagick (images), RMBG (AI background removal). Use for encoding, format conversion, filters, thumbnails, batch processing, HLS/DASH streaming.

**Location**: `.claude/skills/media-processing\SKILL.md`

### 📦 `pdf`

Extract text/tables, create, merge, split PDFs. Fill PDF forms programmatically. Use for PDF processing, generation, form filling, document analysis, batch operations.

**Location**: `.claude/skills/document-skills\pdf\SKILL.md`

### 📦 `pptx`

Create, edit, analyze .pptx PowerPoint files. Use for presentations, slides, layouts, speaker notes, template modification, content extraction, slide generation.

**Location**: `.claude/skills/document-skills\pptx\SKILL.md`

### `xlsx`

Create, edit, analyze spreadsheets (.xlsx, .csv, .tsv). Use for Excel formulas, data analysis, visualization, formatting, pivot tables, charts, formula recalculation.

**Location**: `.claude/skills/document-skills\xlsx\SKILL.md`

## Frameworks & Platforms

### 📚 `mobile-development`

Build mobile apps with React Native, Flutter, Swift/SwiftUI, Kotlin/Jetpack Compose. Use for iOS/Android, mobile UX, performance optimization, offline-first, app store deployment.

**Location**: `.claude/skills/mobile-development\SKILL.md`

### 📦 📚 `shopify`

Build Shopify apps, extensions, themes with Shopify CLI. Use for GraphQL/REST APIs, Polaris UI, Liquid templates, checkout customization, webhooks, billing integration.

**Location**: `.claude/skills/shopify\SKILL.md`

### 📚 `tanstack`

Build with TanStack Start (full-stack React framework), TanStack Form (headless form management), and TanStack AI (AI streaming/chat). Use when creating TanStack projects, routes, server functions, forms, validation, or AI chat features.

**Location**: `.claude/skills/tanstack\SKILL.md`

### 📦 📚 `web-frameworks`

Build with Next.js (App Router, RSC, SSR, ISR), Turborepo monorepos. Use for React apps, server rendering, build optimization, caching strategies, shared dependencies.

**Location**: `.claude/skills/web-frameworks\SKILL.md`

## Utilities & Helpers

### 📚 `bootstrap`

Bootstrap new projects with research, tech stack, design, planning, and implementation. Modes: full (default interactive), auto (explicit autonomous), fast (skip research), parallel (multi-agent).

**Location**: `.claude/skills/bootstrap\SKILL.md`

### 📚 `brainstorm`

Brainstorm solutions with trade-off analysis and brutal honesty. Use for ideation, architecture decisions, technical debates, feature exploration, feasibility assessment, design discussions.

**Location**: `.claude/skills/brainstorm\SKILL.md`

### 📚 `code-review`

Review code quality with evidence-based rigor. Supports input modes: pending changes, PR number, commit hash, and codebase scan. Focuses on bugs, regressions, maintainability, reliability, and verification gaps.

**Location**: `.claude/skills/code-review\SKILL.md`

### `coding-level`

Set coding experience level for tailored output. Use for adjusting explanation depth, code complexity, and response format to user expertise.

**Location**: `.claude/skills/coding-level\SKILL.md`

### 📚 `cook`

Implement features, plans, and fixes with structured workflow. Use for feature development, plan execution, code implementation pipelines.

**Location**: `.claude/skills/cook\SKILL.md`

### 📦 📚 `debug`

Debug systematically with root cause analysis before fixes. Use for bugs, test failures, unexpected behavior, performance issues, call stack tracing, multi-layer validation, log analysis, CI/CD failures, database diagnostics, system investigation.

**Location**: `.claude/skills/debug\SKILL.md`

### 📚 `docs`

Analyze codebase and manage project documentation. Use for doc initialization, updates, summaries, codebase analysis.

**Location**: `.claude/skills/docs\SKILL.md`

### 📚 `fix`

Fix bugs, errors, test failures, and CI/CD issues with intelligent routing. Use for type errors, lint issues, log errors, UI bugs, code problems.

**Location**: `.claude/skills/fix\SKILL.md`

### `journal`

Write technical journal entries analyzing recent changes. Use for session reflections, change analysis, decision documentation.

**Location**: `.claude/skills/journal\SKILL.md`

### 📚 `loop`

Autonomous iterative optimization loop — run N iterations against a mechanical metric, learn from git history, auto-keep/discard changes. Use for improving measurable metrics (coverage, performance, bundle size, etc.) through repeated experimentation.

**Location**: `.claude/skills/loop\SKILL.md`

### 📚 `mermaidjs-v11`

Create diagrams with Mermaid.js v11 syntax. Use for flowcharts, sequence diagrams, class diagrams, ER diagrams, Gantt charts, state diagrams, architecture diagrams, timelines, user journeys.

**Location**: `.claude/skills/mermaidjs-v11\SKILL.md`

### 📚 `plan`

Plan implementations, design architectures, create technical roadmaps with detailed phases. Use for feature planning, system design, solution architecture, implementation strategy, phase documentation.

**Location**: `.claude/skills/plan\SKILL.md`

### 📚 `predict`

5 expert personas debate proposed changes before implementation. Catches architectural, security, performance, and UX issues early. Use before major features or risky changes.

**Location**: `.claude/skills/predict\SKILL.md`

### 📚 `preview`

View files or generate visual explanations, slides, and diagrams. Use for code walkthroughs, architecture visualization, HTML/Markdown presentations.

**Location**: `.claude/skills/preview\SKILL.md`

### 📚 `problem-solving`

Apply systematic problem-solving techniques when stuck. Use for complexity spirals, innovation blocks, recurring patterns, assumption constraints, simplification cascades, scale uncertainty.

**Location**: `.claude/skills/problem-solving\SKILL.md`

### 📚 `project-management`

Track progress, update plan statuses, generate reports, coordinate docs updates. Use for project oversight, status checks, plan completion, cross-session continuity.

**Location**: `.claude/skills/project-management\SKILL.md`

### 📚 `project-organization`

Organize files, directories, and content structure in any project. Use when creating files, determining output paths, organizing existing assets, or standardizing project layout.

**Location**: `.claude/skills/project-organization\SKILL.md`

### `research`

Research technical solutions, analyze architectures, gather requirements thoroughly. Use for technology evaluation, best practices research, solution design, scalability/security/maintainability analysis.

**Location**: `.claude/skills/research\SKILL.md`

### 📚 `retro`

Generate data-driven sprint retrospectives from any git history. Use for sprint reviews, commit analysis, code-health indicators, team-velocity reporting, and quarterly engineering reviews. Works on solo or team repos.

**Location**: `.claude/skills/retro\SKILL.md`

### 📚 `scenario`

Generate comprehensive edge cases and test scenarios by decomposing features across 12 dimensions. Use for pre-implementation risk discovery, QA planning, regression design, and iterative saturation when coverage must be exhaustive.

**Location**: `.claude/skills/scenario\SKILL.md`

### 📚 `security`

STRIDE + OWASP-based security audit with optional red-team persona discovery loop and auto-fix. Scans code for vulnerabilities from multiple attacker perspectives (auth attacker, supply chain, insider, infrastructure), categorizes by severity, and can iteratively fix findings. Use --quick for fast CI/pre-commit scans (replaces deprecated /security-scan).

**Location**: `.claude/skills/security\SKILL.md`

### 📦 📚 `sequential-thinking`

Apply step-by-step analysis for complex problems with revision capability. Use for multi-step reasoning, hypothesis verification, adaptive planning, problem decomposition, course correction.

**Location**: `.claude/skills/sequential-thinking\SKILL.md`

### 📚 `test`

Run unit, integration, e2e, and UI tests. Use for test execution, coverage analysis, build verification, visual regression, and QA reports.

**Location**: `.claude/skills/test\SKILL.md`

### 📦 `watzup`

Generate short handoff reports from Git branches, remote refs, worktrees, and unfinished plans. Use when the user asks what's in flight, wants progress/next steps, is in a fresh worktree or detached checkout, or needs end-of-session status.

**Location**: `.claude/skills/watzup\SKILL.md`

## Other

### `db-analyze`

Analyze database structure and data models for any stack. Auto-detects ORM/DB type (Prisma, TypeORM, Sequelize, Mongoose, GORM, SQLAlchemy, Drizzle, etc.), reads the relevant schema files, and produces a structured summary. Use when brainstorming, reviewing, or planning anything that touches the data layer.

**Location**: `.claude/skills/db-analyze\SKILL.md`

### `simplify`

Edit code the way a copyeditor edits prose — shorter sentences, clearer nouns, same meaning. Targets recently modified files, preserves every observable behavior, and refuses to trade clarity for cleverness. Run after cook or fix, before git.

**Location**: `.claude/skills/simplify\SKILL.md`

### 📚 `theme-implement`

Implement external UI code (Claude.ai artifacts, templates, Figma exports) into project. Hybrid Tailwind + CSS Module override for conflict-free integration.

**Location**: `.claude/skills/theme-implement\SKILL.md`

### 📚 `wails`

Build desktop apps with Wails (Go + React). Use for Go bindings, frontend-backend communication, window management, native dialogs, system tray, events, build/packaging.

**Location**: `.claude/skills/wails\SKILL.md`
