# Real-Profile ATTACH Mode — CDP-Only Guide

> Referenced from `SKILL.md` § "ATTACH real-profile (opt-in)". Read this before implementing any real-profile flow.

## When to use

Only when the task **explicitly requires** the user's real Chrome profile — their cookies, account sessions, or workspace state. Trigger: user passes `--real-profile=<email-or-name>` flag, OR you ask via `AskUserQuestion` with a privacy warning and the user opts in.

**Do NOT infer from keywords like "login", "my account", "sign in", "real".** Throwaway mode handles 99 % of tasks.

## Prerequisites

Chrome must already be running with CDP enabled:

```bash
# macOS
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --remote-debugging-port=9222 --remote-allow-origins=*

# Linux
google-chrome --remote-debugging-port=9222 --remote-allow-origins=*

# Windows (PowerShell)
& "${env:ProgramFiles}\Google\Chrome\Application\chrome.exe" `
  --remote-debugging-port=9222 --remote-allow-origins=*
```

Verify CDP is alive:
```bash
curl -s http://localhost:9222/json/version
```
Non-empty JSON with `"Browser": "Chrome/..."` → ready.

## Profile resolution (from `Local State`)

Chrome stores profile metadata in `<user-data-dir>/Local State` as JSON. The skill reads **only this file** to resolve a profile directory from an email or display-name substring.

```
OS paths for <user-data-dir>:
  macOS:   ~/Library/Application Support/Google/Chrome
  Linux:   ~/.config/google-chrome
  Windows: %LOCALAPPDATA%\Google\Chrome\User Data
```

Resolution algorithm:
1. Read `<user-data-dir>/Local State` (JSON).
2. Iterate `profile.info_cache` — each key is a profile dir (`Default`, `Profile 1`, …).
3. Match `gaia_given_name`, `name`, or `user_name` (email) against the requested key (case-insensitive substring).
4. Return the first match's directory name.
5. **Redact the email before logging or displaying** — show `u***@domain` or omit entirely.

**WHITELIST — permitted reads:**
- `<user-data-dir>/Local State` ← the ONLY file read

**BANNED reads (hard block — never read these):**
- `Cookies`
- `Login Data`
- `<profile-dir>/Cookies`
- `<profile-dir>/Network/Cookies`
- Any `*Cookies*` path
- Any `*Login Data*` path
- Any `*Password*` path

## Tab selection via CDP

After profile is resolved, open the target URL in that profile. Because Chrome's process singleton handles cross-profile IPC, the simplest path is to launch Chrome with the profile flag and a URL — Chrome routes it to the running process:

```bash
# macOS example — Chrome routes to existing process via SingletonLock IPC
"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  --profile-directory="Profile 7" "https://example.com#cdp-open=<token>"
```

Then bind to the tab via CDP:
```javascript
// Pure CDP — no Playwright/Puppeteer library needed
const ws = await fetch('http://localhost:9222/json').then(r => r.json());
const tab = ws.find(t => t.url.includes('cdp-open=<token>'));
// Connect WebSocket to tab.webSocketDebuggerUrl and send CDP commands
```

Or with Playwright attach (no stealth plugin — CDP attach only):
```javascript
import { chromium } from 'playwright';
const browser = await chromium.connectOverCDP('http://localhost:9222');
const ctx = browser.contexts()[0];
const page = ctx.pages().find(p => p.url().includes('cdp-open=<token>'));
// operate on page
await browser.disconnect(); // never close() — user's Chrome
```

## Privacy guards (mandatory)

| Guard | Rule |
|---|---|
| Email redaction | Never log/display/include in response full email. Show `u***@domain` or omit. |
| Path whitelist | Only `Local State` is permitted. Block any read of Cookies / Login Data paths. |
| Opt-in only | Mode activates ONLY when user passes `--real-profile=<key>` or approves `AskUserQuestion` warning |
| No cookie access | Do not read, copy, or log cookies from profile. Use the live session via CDP only. |
| Disconnect, never close | `browser.disconnect()` — the Chrome process belongs to the user. |

## AskUserQuestion — opt-in warning template

Use this when the intent is ambiguous but might need real profile:

```json
{
  "question": "This task may benefit from your real Chrome profile (cookies, logged-in sessions). Use real profile?",
  "header": "Browser Profile Mode",
  "options": [
    {
      "label": "Use throwaway profile (default, safe)",
      "description": "Fresh Chromium launched by skill — no access to your real data, cookies, or accounts"
    },
    {
      "label": "ATTACH real Chrome profile (opt-in)",
      "description": "Connects to your running Chrome via CDP port 9222. Reads only Local State for profile lookup. Never reads Cookies or Login Data."
    }
  ],
  "multiSelect": false
}
```
