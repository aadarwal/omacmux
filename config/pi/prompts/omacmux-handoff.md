---
description: Produce a focused handoff prompt for a new agent/session
argument-hint: "<next goal>"
---
Create a self-contained handoff for this next goal:

$ARGUMENTS

Include:
- current objective
- relevant decisions and constraints
- files changed or likely involved
- commands/tests already run
- current git/worktree state
- exact next action for the receiving agent

If a swarm is active, inspect it with the `omacmux` tool using `status`, then use `collect` or `capture` as needed.

The output should be directly pasteable into a new Pi, Claude, Codex, or OpenCode agent session.
