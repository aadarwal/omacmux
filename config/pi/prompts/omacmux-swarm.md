---
description: Plan or start an omacmux swarm for a task
argument-hint: "<task>"
---
Use omacmux swarm coordination for this task:

$ARGUMENTS

First inspect current state with the `omacmux` tool using `status`.

Then propose the right topology:
- `star` for one conductor coordinating workers.
- `wt` for implementation work that should happen in isolated git worktrees.
- `pair` for coder/reviewer loops.
- `pipe` for staged research, implementation, and verification.

If the user asked you to start the swarm, use the `omacmux` tool with `swarm_start`; otherwise return a concise plan with:
- topology
- agent count
- agent command or preset
- initial message to each role
- expected collection/review step
