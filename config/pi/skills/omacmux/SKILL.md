---
name: omacmux
description: Work effectively inside the omacmux terminal agent IDE. Use when operating tmux layouts, swarms, worktree-isolated agents, recipes, review flows, or the managed Pi omacmux tool.
---

# omacmux

omacmux is a Bash/tmux agent environment. Prefer existing shell functions and project conventions over inventing a new orchestration layer.

## Core Commands

- `tdl <agent>`: editor left, agent right, terminal bottom.
- `tsl <n> <agent>`: tiled panes running the same agent command.
- `twdl <agent> [branches...]`: one agent pane per git worktree, editor on the left.
- `twsl <agent> [branches...]`: full-width worktree swarm.
- `swarm status`: show the current swarm.
- `swarm collect`: aggregate current swarm outputs.
- `swarm send agent-1 "message"`: send a targeted message.
- `swarm broadcast "message"`: message every agent in the active swarm.
- `swarm wt <n> <agent>`: create worktree-isolated agents.
- `recipe list`: list built-in and user recipes.
- `review`: inspect divergent branch work with AI summaries.

## Agent Commands

- `cx`: Claude Code with permissions skipped.
- `cxx`: Claude Code full-auto mode.
- `c`: OpenCode.
- `cdx`: Codex.
- `cdxx`: Codex full-auto mode.
- `pi`: Pi Coding Agent default mode.
- `pix`: Pi with all built-in tools enabled.
- `pir`: Pi read-only mode.

## Pi-Specific Guidance

Use the `omacmux` tool when available. It exposes safe helper actions:

- `status`: inspect the active swarm or list swarms.
- `collect`: aggregate outputs.
- `capture`: read one agent pane.
- `send`: message one agent.
- `broadcast`: message all agents.
- `worktrees`: list git worktrees.
- `recipes`: list recipes without opening the fzf picker.
- `recipe_run`: run a named omacmux recipe.
- `review`: run the omacmux review flow.
- `merge`: merge swarm worktree results.
- `worktree_add`: create a sibling git worktree.
- `swarm_start`: start a flat, star, pipe, pair, or worktree swarm.

Before coordinating agents, call `omacmux` with `status`. Before changing files from a Pi agent, check git state and prefer a dedicated worktree for substantial work.

## Presets

Use `/omacmux-preset <name>` when a session should take on a role:

- `scout`: fast codebase reconnaissance, no edits.
- `planner`: deep implementation planning, no edits.
- `reviewer`: bug/regression review, no edits.
- `builder`: focused implementation with edit/write access.
- `conductor`: tmux swarm/worktree coordination.

The package can also start with a preset:

```bash
pi --omacmux-preset scout
pi --omacmux-preset builder
```

## Prompt Templates

Use these package prompt templates when appropriate:

- `/omacmux-swarm <task>`: choose or start a swarm topology.
- `/omacmux-worktree <task>`: create/use worktree-isolated implementation.
- `/omacmux-review [target]`: review branch, swarm, or agent output.
- `/omacmux-handoff <goal>`: create a focused handoff for another agent/session.

## Safety Gates

The package blocks writes/edits to protected paths such as `.env`, `.git`, `node_modules`, SSH/AWS/GPG config, and common key/credential files. It also asks for confirmation before dangerous bash commands such as recursive removal, `sudo`, destructive git resets/cleans, disk/system operations, and downloaded shell execution.
