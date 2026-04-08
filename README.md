# omacmux

An agent-first IDE built entirely in the terminal.

> *"It never gets old that your operating system is this malleable."*
> — DHH

---

tmux is the window manager. Neovim is the editor. AI agents are first-class panes — not plugins bolted on, but peers sitting next to your code with their own space to think. You compose layouts that give you and your agents exactly the arrangement you need, then tear them down and reshape them when the work changes.

This is a research environment. The kind of setup where you spawn four Claude instances across four git worktrees and let them all run in parallel. Where you open a swarm of agents tiled across your screen and watch them collaborate. Where your dev layout is nvim on the left, an AI agent on the right, and a terminal at the bottom — and that's just the default.

Everything is a shell function. Everything is composable. Everything is yours to change.

---

## Quick Start

### Option A: Homebrew (recommended)

```bash
brew install aadarwal/omacmux/omacmux
omacmux init
```

### Option B: Git clone

```bash
git clone https://github.com/aadarwal/omacmux.git ~/omacmux
~/omacmux/bin/omacmux init
```

`omacmux init` installs dependencies, then walks you through each config file — merge with your existing setup, replace (with backup), or skip. For a fresh machine, use `omacmux init --replace-all`.

Open a **new Ghostty window**, then:

```bash
t              # start tmux
tdl cx         # dev layout: nvim + claude + terminal
```

Or go straight to voice-driven agent work:

```bash
voice on       # enable audio feedback
vibe           # launch 4 agents + auto-commit in one word
check          # "4 agents active, 12 files changed"
recap          # AI summary of what changed
vibe stop      # tear it all down
```

---

## The Stack

| Layer | Tool |
|-------|------|
| Terminal | [Ghostty](https://ghostty.org) |
| Multiplexer | tmux |
| Editor | [Neovim](https://neovim.io) + [LazyVim](https://www.lazyvim.org) |
| Prompt | [Starship](https://starship.rs) |
| Shell | Bash 5 + [eza](https://eza.rocks) + [fzf](https://junegunn.github.io/fzf/) + [zoxide](https://github.com/ajeetdsouza/zoxide) + [bat](https://github.com/sharkdp/bat) + [mise](https://mise.jdx.dev) |
| AI | [Claude Code](https://claude.ai/code), [opencode](https://github.com/sst/opencode), [Codex](https://openai.com/index/codex/), or anything that runs in a terminal |

---

## Layouts

All layout commands run inside tmux. This is the core of omacmux — arranging agents and editors in space.

### `tdl` — Dev Layout

The daily driver. Editor left, agent right, terminal bottom.

```
┌──────────────────────┬─────────────┐
│                      │   AI (30%)  │
│     nvim (70%)       │   e.g. cx   │
│                      │             │
├──────────────────────┴─────────────┤
│           terminal (15%)           │
└────────────────────────────────────┘
```

```bash
tdl cx         # nvim + claude + terminal
tdl cx c       # nvim + claude + opencode (stacked)
```

### `tsl` — Swarm Layout

N tiled panes all running the same command. Parallel AI at scale.

```bash
tsl 4 cx       # 4 claude instances, tiled
tsl 4 cxx      # same, full permissions skip
```

### `twdl` — Worktree Dev Layout

One agent per git worktree, stacked on the right. Editor on the left browses all of them.

```
┌──────────────────────┬─────────────────────┐
│                      │ cx @ master          │
│     nvim (70%)       ├─────────────────────┤
│   (browses all wts)  │ cx @ feat/auth       │
├──────────────────────┴─────────────────────┤
│              terminal (15%)                │
└────────────────────────────────────────────┘
```

```bash
twdl cx              # auto-detect all worktrees
twdl cx master feat  # specific branches only
```

### `twsl` — Worktree Swarm

Full-width vertical stack, one AI per worktree.

```bash
twsl cxx             # full-width swarm across all worktrees
```

### More Layouts

| Command | What it does |
|---------|--------------|
| `tdlm <ai>` | One `tdl` per subdirectory — built for monorepos |
| `tpl` | Two editors side by side + terminal |
| `tml <cmds...>` | Main pane left, stacked command panes right |
| `twl <branch> <ai>` | New tab with `tdl` in a specific worktree |
| `twlm <ai>` | One tab per worktree, each with full `tdl` |

### Dynamic Expansion

Add or remove agents mid-session without destroying your layout.

| Command | Description |
|---------|-------------|
| `taa [cmd]` | Add an agent pane (default: claude). Re-tiles layout. |
| `tra [pane]` | Remove an agent pane (fzf picker if no arg) |
| `tscale <n> [cmd]` | Scale to exactly N agent panes |
| `tap` | List tracked agent panes with status |

```bash
# Start with 2 agents, scale up when you need more
tdl cx
taa cx         # add another claude
taa cdx        # add a codex
tscale 6 cx    # scale to 6 agents
```

---

## Agent Swarms

The swarm system lets you orchestrate multiple AI agents across different topologies. Each topology defines how agents communicate.

### Topologies

| Command | Topology | Description |
|---------|----------|-------------|
| `swarm start <n> <cmd>` | Flat | N equal workers, you orchestrate |
| `swarm star <n> <cmd>` | Star | 1 conductor + N-1 workers. Conductor delegates. |
| `swarm pipe <n> <cmd>` | Pipeline | Sequential stages. Each reads predecessor output. |
| `swarm pair <cmd>` | Pair | Coder + reviewer feedback loop |
| `swarm wt <n> <cmd>` | Worktree | One agent per auto-created git worktree |
| `swarm mesh <n> <cmd>` | Distributed | Across Tailscale devices |

### Communication

| Command | Description |
|---------|-------------|
| `swarm send <agent> "<msg>"` | Message a specific agent |
| `swarm broadcast "<msg>"` | Message all agents |
| `swarm capture <agent>` | Capture agent's pane output |
| `swarm collect` | Aggregate all agent outputs |

### Management

| Command | Description |
|---------|-------------|
| `swarm status` | Agent status table |
| `swarm dashboard` | Live-refreshing popup (5s refresh) |
| `swarm ls` | List all active swarms |
| `swarm kill` | Tear down current swarm |
| `swarm merge [--all]` | Merge worktree agent branches back |

### Enhanced Swarm (`swarmx`)

| Command | Description |
|---------|-------------|
| `swarmx plan <topo> <n> <cmd>` | Dry-run: preview what a swarm would create |
| `swarmx status` | Enhanced status with git diff stats per agent |
| `swarmx merge` | Interactive fzf merge with conflict detection |

### Quick Launch

| Command | Description |
|---------|-------------|
| `al [cmd]` | Launch agent in a new split pane (fzf picker) |
| `alw [cmd]` | Launch agent in a new window |

Available agent aliases: `cx` (Claude), `cxx` (Claude full-auto), `c` (OpenCode), `cdx` (Codex), `cdxx` (Codex full-auto).

---

## Change Review

When a swarm finishes, you don't want to read 200 raw diffs. The review system sends diffs to a model and shows you AI-comprehended summaries — cached per commit SHA so it's instant on re-review.

| Command | Description |
|---------|-------------|
| `review` | Overview of all divergent branches with AI summaries |
| `review <swarm_id>` | Scope review to a specific swarm |
| `review <agent>` | Deep dive: one agent's work with per-file annotations |
| `review <agent> <file>` | AI-explained diff for a single file |
| `review conflicts` | Cross-agent conflict analysis |
| `reviewd` | Dashboard mode: persistent pane, auto-refreshing |

---

## Voice Mode

omacmux is designed to be driven by voice via [Wispr Flow](https://wispr.ai) or any voice-to-text tool. Voice mode adds TTS feedback and sound effects so you know what's happening without staring at the terminal.

### Toggle

```bash
voice on       # hear "Voice mode active"
voice off      # hear "Voice mode off", then silence
voice status   # check state
```

### Voice Commands

These are the commands you'd speak. Short, unambiguous, composable.

| Command | Description |
|---------|-------------|
| `vibe [n]` | Launch N agents (default 4) + auto-commit. One word to start working. |
| `vibe stop` | Kill swarm + stop auto-commit |
| `check` | Spoken status: "3 agents active, 47 files changed" |
| `ship [branch]` | Push branch, report PR URL |
| `focus <name>` | Switch to an agent's pane |
| `recap` | AI summary of what changed on this branch |

### Agent Names (NATO Phonetic)

Agents get NATO names automatically — no setup. Agent-1 is Alpha, agent-2 is Bravo, etc.

| Command | Description |
|---------|-------------|
| `who` | List agents with names and activity |
| `tell <name> "<msg>"` | Send message to named agent |
| `name <ref> <nickname>` | Give an agent a custom name |

```bash
who                           # "Alpha: active, Bravo: active, Charlie: idle"
tell alpha "focus on auth"    # sends to agent-1
tell bravo "write tests"      # sends to agent-2
name 3 scout                  # agent-3 is now "scout"
focus scout                   # switch to scout's pane
```

### Audio Feedback

When voice mode is on:
- **Notifications** from Claude Code are spoken aloud
- **Sound effects** play for swarm events (start, done, error)
- All commands announce what they're doing

---

## Recipes

Pre-built swarm configurations for common workflows. Speak a recipe name and agents spin up.

| Command | Description |
|---------|-------------|
| `recipe research <topic>` | 4 agents, star topology — deep-dive a topic |
| `recipe build <feature>` | 3 agents, worktree topology — build in parallel branches |
| `recipe fix <issue>` | 2 agents, pair topology — debug together |
| `recipe review` | Launch the review dashboard |

### Custom Recipes

| Command | Description |
|---------|-------------|
| `recipe list` | Show built-in and user recipes |
| `recipe save <name>` | Save current swarm config as a recipe |
| `recipe edit <name>` | Edit a user recipe |
| `recipe delete <name>` | Delete a user recipe |

---

## Auto-Commit

Toggle periodic background commits for "vibe mode" — when you want agents working fast without manual git management.

| Command | Description |
|---------|-------------|
| `acm on [seconds]` | Start auto-committing (default: every 5 min) |
| `acm off` | Stop |
| `acm status` | Show state, PID, last commit time |
| `acm log` | Show recent auto-commits (tagged `[auto]`) |

---

## Project Dashboard

See your entire project state at a glance.

| Command | Description |
|---------|-------------|
| `pd` | Full-screen fzf popup: branches, worktrees, active swarms |
| `pds` | Persistent sidebar (toggle on/off) |

Actions inside `pd`: checkout branch (enter), create worktree (ctrl-w), spawn agent (ctrl-a).

---

## Sessions

| Command | Description |
|---------|-------------|
| `t` | Attach last session / create "Work", or fzf picker inside tmux |
| `t <name>` | Attach or create named session |
| `t <name> <dir>` | Named session rooted at directory |
| `t .` | Session named after current git repo |
| `tj [query]` | Jump to project dir (zoxide + fzf), create/attach session |
| `tp` | fzf session picker with preview and inline kill (`ctrl-x`) |
| `tk [name]` | Kill session |
| `tl` | List sessions |

### Workspace Configs

Drop a `.tmux-workspace` file in any project directory. When `t` creates a session there, it sources the file automatically.

```bash
# .tmux-workspace
tdl cx
tw server "rails server"
tw logs "tail -f log/development.log"
tmux select-window -t :1
```

### Persistence

```bash
tss              # save tmux state
tsr              # restore it
```

---

## Windows

| Command | Description |
|---------|-------------|
| `tw <name> [cmd]` | Create named window, optionally run a command |
| `twp` | fzf window picker |
| `to [cmd]` | Popup overlay / scratchpad |
| `tws` | Stash current window |
| `twg` | Grab window back from stash |

---

## Worktrees

| Command | Description |
|---------|-------------|
| `gwa <branch> [base]` | Create worktree as sibling directory |
| `gwr [branch]` | Remove worktree (fzf picker if no arg) |
| `gwl` | List worktrees |
| `gws [branch]` | cd into worktree (fzf picker if no arg) |
| `twf [path]` | Refocus all AI panes to a specific worktree |

In nvim, `<leader>gw` opens a telescope worktree picker that switches cwd and auto-focuses the corresponding agent pane.

---

## Git

### Quick Commands

| Alias | Description |
|-------|-------------|
| `g` | `git` |
| `gcm <msg>` | Commit with message |
| `gcam <msg>` | Stage all + commit |
| `gwip` | Quick WIP commit (stage all) |
| `gunwip` | Undo last WIP commit |
| `gp` | Push + print PR URL |
| `gpf` | Force push with lease |
| `gsync` | Fetch + rebase on default branch |
| `gpr [-d]` | Push + create PR (add -d for draft) |
| `gclean` | Delete merged branches (fzf multi-select) |

### Interactive (fzf)

| Command | Description |
|---------|-------------|
| `gb [filter]` | Branch checkout with preview, delete (ctrl-d) |
| `gl` | Git log browser, copy SHA (enter), show diff (ctrl-d) |
| `ga` | Interactive staging: multi-select, stage/unstage |
| `gd [ref]` | Diff viewer, open in editor |
| `gst` | Stash manager: apply, drop, pop |

---

## Config Management

| Command | Description |
|---------|-------------|
| `cfgmap` | fzf browser of all editable configs (categorized by level) |
| `cfgedit <name>` | Quick-edit config by short name |

```bash
cfgedit tmux       # opens tmux.conf
cfgedit aliases    # opens aliases file
cfgedit ghostty    # opens terminal config
```

---

## Utilities

| Command | Description |
|---------|-------------|
| `rgi [query]` | Interactive ripgrep — search, preview, open match in editor |
| `fdi [query]` | Interactive fd — find files, open in editor |
| `fp` | Process picker: SIGTERM (enter), SIGKILL (ctrl-k) |
| `fenv` | Environment variable browser |
| `mkd <name>` | mkdir + cd |
| `json [file] [query]` | Pretty-print JSON or run jq query |
| `serve [port]` | Quick HTTP server (Python) |
| `extract <file>` | Universal archive extraction |

---

## Mesh (Tailscale)

Connect to remote devices for distributed agent swarms.

| Command | Description |
|---------|-------------|
| `mesh` | fzf device picker (online + SSH-capable) |
| `mesh ssh` | Filter for SSH-capable devices |
| `mesh all` | Show all devices |

---

## Keybindings

### No prefix (always available)

| Key | Action |
|-----|--------|
| `Ctrl+Option+Arrows` | Navigate panes |
| `Ctrl+Option+Shift+Arrows` | Resize panes |
| `Option+1-9` | Switch to window |
| `Option+Left/Right` | Prev/next window |
| `Option+Up/Down` | Prev/next session |
| `F12` | Toggle nested tmux pass-through |

### With prefix (`Ctrl+B`)

| Key | Action |
|-----|--------|
| `h` / `v` | Split below / right |
| `x` | Kill pane |
| `c` / `k` / `r` | New / kill / rename window |
| `C` / `K` / `R` | New / kill / rename session |
| `q` | Reload config |
| `s` / `w` / `j` | Session / window / project picker (fzf) |
| `` ` `` | Scratchpad popup |
| `a` | AI popup (claude) |
| `A` | Agent launcher (fzf picker) |
| `+` / `-` | Add / remove agent pane |
| `f` | Review popup |
| `d` | Project dashboard popup |
| `g` / `G` / `b` | Git log / worktree / branch picker |
| `m` | Mesh (Tailscale) popup |
| `S` | Swarm hub popup |
| `U` | Grab window from stash |

---

## Aliases

| Alias | What it runs |
|-------|-------------|
| `cx` | Claude Code (permissions skip) |
| `cxx` | Claude Code (full permissions skip) |
| `c` | OpenCode |
| `cdx` | Codex |
| `cdxx` | Codex (full auto) |
| `n` | Neovim |
| `g` | git |
| `d` | docker |
| `r` | rails |
| `ls` | eza with icons |
| `lt` | eza tree view |
| `ff` | fzf with bat preview |
| `eff` | fzf + open in editor |
| `cd` | zoxide (smart directory jumping) |

---

## Install / Upgrade / Uninstall

### The `omacmux` CLI

| Command | Description |
|---------|-------------|
| `omacmux init` | Interactive setup — deps, config linking, git identity |
| `omacmux init --replace-all` | Non-interactive: replace all configs (fresh machine) |
| `omacmux init --merge-all` | Non-interactive: merge shell files, replace the rest |
| `omacmux init --skip-existing` | Non-interactive: skip any existing config file |
| `omacmux unlink` | Remove all omacmux configs, restore backups |
| `omacmux status` | Show current state of all config links |
| `omacmux upgrade` | Pull latest + sync deps + update links |
| `omacmux doctor` | Check installation health |

### Config conflict resolution

When `omacmux init` finds an existing config file, it offers three strategies:

- **Merge** (shell files like `~/.bashrc`): appends a `source omacmux` line to your existing file. Your config stays intact.
- **Replace**: backs up your file (e.g., `~/.bashrc.omacmux-bak.20260408_143022`), then symlinks ours. `omacmux unlink` restores the backup.
- **Skip**: leaves the file untouched.

### Legacy scripts

`./install.sh`, `./upgrade.sh`, and `./uninstall.sh` still work — they delegate to `omacmux init/upgrade/unlink`.

### What gets installed

**Tools** (via Homebrew): tmux, bash 5, neovim, starship, eza, fzf, zoxide, bat, ripgrep, fd, mise, gh, jq, tree

**Font** (install separately): `brew install --cask font-jetbrains-mono-nerd-font`

### What gets linked

All config files are linked from the repo to their standard locations (`~/.config/tmux/`, `~/.config/nvim/`, `~/.config/ghostty/`, `~/.bashrc`, etc.). Run `omacmux status` to see the full list and current state.

---

## State & Data

omacmux stores runtime state in `~/.local/share/omacmux/`:

| Directory | Contents |
|-----------|----------|
| `swarms/` | Swarm metadata, agent state, mailboxes |
| `reviews/` | Cached AI summaries per commit SHA |
| `autocommit/` | PID files, auto-commit state |
| `voice/` | Voice mode toggle state |
| `recipes/` | User-saved swarm recipes |
| `agentnames/` | Custom agent nicknames |
| `mesh/` | Tailscale device cache, host definitions |

---

## Notes

- **Bash 5+** required for `tsl`. The installer offers to set Homebrew bash as default.
- **macOS only** — uses Ghostty, `say` (TTS), `afplay` (sounds), `osascript` (notifications).
- **Mission Control** — `Ctrl+Option+Shift+Arrows` may conflict. Disable in System Settings > Keyboard > Keyboard Shortcuts > Mission Control.
- **First nvim launch** auto-installs 46 plugins via lazy.nvim (~30-60s).
- **Voice mode** requires [Wispr Flow](https://wispr.ai) or any voice-to-text tool for input. TTS output uses macOS built-in `say`.

---

## Credits

The DHH quote is from his work on [omakub](https://omakub.org) / [omarchy](https://github.com/basecamp/omarchy), which inspired the original direction. omacmux has since diverged into something different — an agent-first research environment rather than a general-purpose dev setup.
