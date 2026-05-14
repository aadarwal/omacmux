# Cursor SDK Integration

## Sources Reviewed

- Cursor SDK launch post: https://cursor.com/blog/typescript-sdk
- Cursor SDK TypeScript docs: https://cursor.com/docs/sdk/typescript
- Local sample repo: `/Users/aadarwal/cursor-cookbook`

The cookbook examples that mattered most were:

- `sdk/quickstart`: minimal local `Agent.create()` plus `run.stream()`.
- `sdk/coding-agent-cli`: plain prompt mode, local/cloud switching, model
  discovery, cloud repo detection, and defensive tool-call summaries.
- `sdk/dag-task-runner`: fan-out through multiple local SDK agents.

## Recommendation

The best first integration is a small executable agent backend, not a new UI.
omacmux already has the UI surface: tmux panes, layouts, worktree swarms,
mailboxes, capture, and review. The Cursor SDK should therefore enter the
system as another terminal agent command that can be launched anywhere `cx`,
`cdx`, or `c` can be launched.

That is what `bin/omacmux-cursor-agent` provides:

- `cs`: Cursor SDK local interactive agent.
- `csf`: same local agent with `local.force` for stuck-run recovery.
- `csc`: Cursor SDK cloud agent, using the current GitHub remote and branch.
- `omacmux-cursor-agent "prompt"`: one-shot local run.
- `echo "prompt" | omacmux-cursor-agent`: headless prompt mode.
- `omacmux-cursor-agent --cloud --auto-pr "prompt"`: cloud run with PR creation
  requested by the SDK.

This preserves the existing omacmux architecture:

- `tdl cs` opens editor plus a Cursor SDK agent pane.
- `tsl 4 cs` starts four SDK local agents in tiled panes.
- `swarm wt 3 cs` gives each SDK agent its own worktree and branch.
- `al cs` works through the existing quick-launch registry.

## Why Not Start With Other Options

### Cursor CLI Alias Only

Adding `cursor-agent` aliases would be useful, but it would not exercise the
new SDK. The SDK gives omacmux programmatic access to local/cloud runtimes,
run IDs, streaming events, model listing, resume, and cloud PR metadata.

### Bun/OpenTUI App

The cookbook `coding-agent-cli` uses Bun because OpenTUI needs `bun:ffi`.
omacmux does not need another full-screen renderer. A plain Node process is
easier to install, easier to run in tmux panes, and works with shell-based swarm
message injection.

### Root Node Package

Adding a root `package.json` would make omacmux look like a Node project and
would require repo-local `node_modules`. Instead, `omacmux-cursor-agent setup`
installs `@cursor/sdk` into a user state directory:

```bash
${XDG_DATA_HOME:-$HOME/.local/share}/omacmux-cursor-sdk
```

This keeps the dotfiles repo clean while still pinning the install path and
making diagnostics straightforward through `csdoctor`.

### Cloud-Only Swarm Replacement

Cloud agents are valuable for durable runs and high parallelism, but they do
not replace tmux-local workflows. The first bridge should let users choose:
local agents when the working tree and tmux observability matter, cloud agents
when durability, isolation, or PR creation matter.

### MCP Server First

An omacmux MCP server is a strong next step, especially for exposing swarm
state, mailbox reads, capture results, atlas context, and worktree metadata to
Cursor agents. It should come after the agent backend because the SDK wrapper
creates the immediate place to consume project MCP config through
`local.settingSources`.

## Cursor SDK Constraints Captured In The Integration

- SDK APIs are public beta and may change.
- `CURSOR_API_KEY` is required for SDK runs.
- Local agents need a model selection; default is `composer-2`, overridden by
  `OMACMUX_CURSOR_MODEL` or `CURSOR_MODEL`.
- Local file-based Cursor config is opt-in through `local.settingSources`.
  omacmux defaults this to `project`, so `.cursor/skills/` and
  `.cursor/agents/` in the current repo can load without pulling in user-level
  settings by default.
- Cloud agents ignore `local.settingSources`; they load project/team/plugin
  config through Cursor cloud behavior.
- Inline MCP definitions are intentionally not handled by shell aliases yet.
  They should be added through `.cursor/mcp.json` first, or later through a
  small JSON config flag.
- Hooks are file-based only, so this integration does not try to create
  programmatic hook callbacks.
- Local artifact download is not currently available in the SDK.
- `npm audit` currently flags transitive vulnerabilities under
  `@cursor/sdk@1.0.12` (`sqlite3`/`node-gyp`/`tar` and `undici`). The audit
  reports no direct fix for the SDK package at this version, so the wrapper
  installs it outside the repo and keeps the package spec overrideable.

## Files Added Or Changed

- `bin/omacmux-cursor-agent`: plain Node CLI/REPL around `@cursor/sdk`.
- `config/bash/aliases`: `cs`, `csf`, and `csc` aliases.
- `config/bash/fns/cursor_sdk`: `cssetup`, `csdoctor`, `csmodels`, and `csrepo`.
- `config/bash/fns/agentlaunch`: quick-launch registry entries.
- `shell/bashrc`: adds `$OMACMUX_PATH/bin` to `PATH`.
- `links.sh`: links `omacmux-cursor-agent` into `~/.local/bin`.
- `Brewfile`: adds Node as the SDK runtime dependency.
- `.cursor/skills/omacmux/SKILL.md`: project skill for Cursor agents.
- `.cursor/agents/omacmux-integrator.md`: optional file-based Cursor subagent.

## Setup

```bash
cssetup
export CURSOR_API_KEY="crsr_..."
csdoctor
```

`cssetup` installs the SDK dependency outside the repo. If you want a custom
install location:

```bash
export OMACMUX_CURSOR_SDK_HOME="$HOME/.local/share/omacmux-cursor-sdk"
cssetup
```

By default it installs the validated package spec `@cursor/sdk@1.0.12`. Override
`OMACMUX_CURSOR_SDK_PACKAGE` if you intentionally want a newer SDK build.

## Usage

Local interactive pane:

```bash
tdl cs
```

One-shot local prompt:

```bash
cs "Explain this repository in one paragraph"
```

Worktree-isolated swarm:

```bash
swarm wt 3 cs
swarm broadcast "Explore different approaches to improving the review workflow"
```

Cloud run against the current GitHub remote and branch:

```bash
csc "Investigate this repo and suggest a safe cleanup plan"
```

Cloud run with PR creation requested:

```bash
omacmux-cursor-agent --cloud --auto-pr "Fix the failing tests and open a PR"
```

List available models:

```bash
csmodels
```

Detect the cloud repo target:

```bash
csrepo
```

## Next Best Extensions

1. Add `.cursor/mcp.json` plus an `omacmux-mcp` server that exposes swarm
   status, captures, worktree metadata, and atlas summaries.
2. Add a `swarm cloud` topology that records Cursor cloud agent IDs in the same
   swarm state directory without pretending they are tmux panes.
3. Add `review cursor <agent-id|run-id>` helpers around `Agent.listRuns()` and
   `Agent.getRun()` once real SDK runs are available with an API key.
4. Add a small JSON config loader for inline SDK `mcpServers` and named
   subagents when shell flags become too cramped.
