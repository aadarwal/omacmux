---
description: Review omacmux, swarm, or branch changes
argument-hint: "[branch|swarm|agent]"
---
Review this omacmux work:

$ARGUMENTS

Adopt the reviewer preset mentally:
- prioritize bugs, regressions, unsafe shell behavior, workflow breakage, and missing tests
- inspect actual diffs and command outputs before concluding
- avoid style-only comments unless they hide a real maintainability risk

Useful commands and tools:
- `git status --short`
- `git diff --stat`
- `git diff`
- `omacmux` action `review` for existing omacmux review flows
- `omacmux` action `collect` or `capture` for active swarm output

Return findings first, ordered by severity, with file references where possible.
