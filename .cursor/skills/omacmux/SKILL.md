# omacmux

Use this skill when modifying omacmux shell, tmux, swarm, worktree, or agent
integration code.

## Project Shape

- omacmux is a Bash-first, tmux-based agent IDE for macOS.
- Shell functions live in `config/bash/fns/` and are sourced into interactive
  shells from `shell/bashrc`.
- Agent command aliases live in `config/bash/aliases`; the quick-launch picker
  registry lives in `config/bash/fns/agentlaunch`.
- tmux layouts are mostly in `config/bash/fns/tmux`, worktree layouts in
  `config/bash/fns/worktree`, and swarm orchestration in
  `config/bash/fns/swarm`.
- Install/link behavior is driven by `bin/omacmux`, `links.sh`, and `Brewfile`.

## Editing Guidance

- Preserve the shell-first style. Prefer small functions and explicit command
  strings over new frameworks.
- Keep commands usable both when typed into an existing interactive pane and
  when launched as a direct tmux command.
- When adding a new executable, make sure it is either on `$PATH` through
  `shell/bashrc` or linked from `links.sh`.
- Avoid destructive git operations in helpers unless the command name and docs
  make the destructive behavior explicit.
- For SDK-backed tools, keep credentials in environment variables and avoid
  writing API keys into repo files or generated state.

## Verification

- Run `bash -n` on edited Bash files.
- Run `node --check` on edited Node executables.
- Run `go test ./...` inside any changed Go module.
- For docs-only changes, inspect the rendered Markdown headings and examples for
  command accuracy.
