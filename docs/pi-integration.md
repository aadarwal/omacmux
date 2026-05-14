# Pi Integration

This repo should integrate Pi primarily as another terminal-native agent, not as a separate Node application embedded into omacmux.

## Why This Shape

omacmux is intentionally Bash and tmux first. Its abstractions are panes, worktrees, shell functions, and lightweight state files. Pi already has a terminal UI, print/JSON modes, RPC mode, SDK support, project-local resources, extensions, skills, and prompt templates. The best fit is to let Pi run inside the same tmux layouts as Claude, OpenCode, and Codex, then add a thin managed Pi bundle for omacmux-specific context and helper actions.

Pi's own design points in the same direction: its docs emphasize extension/package customization, context files, project `.pi` resources, and external orchestration through tools such as tmux. It intentionally does not ship built-in MCP, sub-agents, plan mode, permission popups, or background bash. Those are already omacmux strengths, so duplicating them through the Pi SDK would add a second orchestration layer without solving the main integration problem.

## Options Considered

### 1. CLI-as-Agent

Add Pi to the existing command registry and aliases:

- `pi`: default Pi session.
- `pix`: Pi with `read,bash,edit,write,grep,find,ls`.
- `pir`: Pi read-only review mode.

This makes `tdl pi`, `tsl 4 pix`, `twdl pir`, `swarm wt 3 pix`, and the fzf agent launcher work exactly like the existing Claude/OpenCode/Codex flows.

This is the default path.

### 2. omacmux-pi Package

Use `config/pi` as a local Pi package named `omacmux-pi`. It bundles the extension, skill, prompt templates, presets, and safety gates while still allowing `omacmux init` to link the resources into Pi's normal global discovery locations.

This is included in the implementation.

### 3. Published Pi Package

Packaging the `config/pi` bundle for npm or git would make sense once this integration stabilizes and needs to be shared independently of the repo. For now, a package would mostly add release overhead.

Keep this as a future extraction path.

### 4. Pi SDK or RPC

SDK/RPC integration is useful if omacmux grows a persistent UI, daemon, or non-tmux controller that needs structured Pi events. Today, omacmux already has terminal panes and tmux observability, so SDK/RPC would add runtime complexity and a Node dependency to a Bash-first project.

Do not make this the default unless a concrete UI/daemon feature needs it.

## Current Implementation

- `config/bash/aliases` adds `pix` and `pir`.
- `config/bash/fns/agentlaunch` registers `pi`, `pix`, and `pir` for `al` and `alw`.
- `config/tmux/tmux.conf` enables `extended-keys` with `csi-u`, matching Pi's recommended tmux setup for modified Enter keys.
- `config/pi/package.json` declares a local Pi package named `omacmux-pi`.
- `config/pi/extensions/omacmux-pi/index.ts` registers the `omacmux` tool, `/omacmux` command, `/omacmux-preset` command, preset flag, and safety gates.
- `config/pi/skills/omacmux/SKILL.md` gives Pi omacmux-specific workflows and command names.
- `config/pi/prompts/` provides swarm, worktree, review, and handoff prompt templates.
- `links.sh` links those resources into `~/.pi/agent/` during `omacmux init` when the target paths are free.
- `README.md` documents Pi as a first-class agent option.

## Package Surface

The package gives Pi these omacmux-aware commands and resources:

- `/omacmux`: inspect/control swarms, worktrees, recipes, reviews, and agent messages.
- `/omacmux-preset`: switch between `scout`, `planner`, `reviewer`, `builder`, and `conductor`.
- `--omacmux-preset <name>`: start Pi directly in a role.
- `/omacmux-swarm`, `/omacmux-worktree`, `/omacmux-review`, `/omacmux-handoff`: prompt templates for common workflows.
- Safety gates: protected path blocking and confirmation for dangerous bash commands.

## Future Work

- Publish `config/pi` as an npm or git package if it should be installable independently of omacmux.
- Add a small smoke command to `omacmux doctor` that checks `pi --version` and warns if tmux extended keys are unavailable.
- Consider an RPC bridge only if a future desktop/widget UI needs structured Pi session events.

## References

- https://pi.dev/docs/latest/usage
- https://pi.dev/docs/latest/extensions
- https://pi.dev/docs/latest/packages
- https://pi.dev/docs/latest/sdk
- https://pi.dev/docs/latest/rpc
- https://pi.dev/docs/latest/tmux
