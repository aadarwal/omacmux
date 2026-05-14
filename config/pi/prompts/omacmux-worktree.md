---
description: Create or use a worktree-isolated implementation flow
argument-hint: "<task>"
---
Use omacmux worktree discipline for this task:

$ARGUMENTS

Start by checking:
- `git status --short`
- `git worktree list`
- current branch and base branch

If a new branch is appropriate, use the `omacmux` tool with `worktree_add`.

For multiple agents, prefer a worktree swarm:
- use `swarm_start` with topology `wt`
- use `pix` or the builder preset for implementation agents
- keep reviewer/scout roles read-only where possible

When done, summarize:
- worktree path
- branch name
- files changed
- commands/tests run
- merge/review recommendation
