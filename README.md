# omacmux

**The [omarchy](https://github.com/basecamp/omarchy) developer experience, rebuilt for macOS.**

One command to go from a fresh Mac to a fully configured terminal dev environment — tmux layouts, LazyVim, Ghostty, starship prompt, git config, and shell ergonomics. Everything symlinked, everything version-controlled, everything yours.

> *"It never gets old that your operating system is this malleable."*
> — DHH

---

## Quick Start

```bash
git clone https://github.com/aadarwal/omacmux.git ~/omacmux
cd ~/omacmux && ./install.sh
```

Open a **new Ghostty window**, then:

```bash
t              # start tmux
tdl cx         # dev layout: nvim + claude + terminal
```

That's it. You're in.

---

## What You Get

| Layer | What's configured |
|-------|-------------------|
| **Terminal** | [Ghostty](https://ghostty.org) with `macos-option-as-alt`, JetBrainsMono Nerd Font, hidden titlebar |
| **Multiplexer** | tmux with vi-copy, mouse support, git-branch-per-tab status bar, popup pickers, nested tmux toggle |
| **Editor** | [LazyVim](https://www.lazyvim.org) with 46 plugins, tokyonight-night, transparent backgrounds, telescope worktree picker |
| **Prompt** | [Starship](https://starship.rs) with git branch/status, worktree detection |
| **Shell** | Bash 5 with [eza](https://eza.rocks), [fzf](https://junegunn.github.io/fzf/), [zoxide](https://github.com/ajeetdsouza/zoxide), [bat](https://github.com/sharkdp/bat), [mise](https://mise.jdx.dev), history search, tab-completion cycling |
| **Git** | Rebase-on-pull, histogram diffs, rerere, GitHub CLI credential helper, worktree aliases |
| **Theme** | Matte-black palette across all surfaces with bundled wallpapers |

---

## Session Management

| Command | Description |
|---------|-------------|
| `t` | Attach last session / create "Work" (outside tmux) or fzf session picker (inside tmux) |
| `t <name>` | Attach or create a named session |
| `t <name> <dir>` | Create named session rooted at directory |
| `t .` | Session named after current git repo |
| `tn <name> [dir]` | Create new named session (errors if exists) |
| `tp` | fzf session picker with live preview and inline kill (`ctrl-x`) |
| `tj [query]` | Jump to project directory (zoxide + fzf), create/attach session |
| `tk [name]` | Kill session (current if no arg) |
| `tl` | List all sessions |

```bash
t                  # attach to last session or create "Work"
t myapp            # attach to "myapp" or create it
t myapp ~/code/app # create "myapp" rooted at ~/code/app
t .                # session named after git repo root
tj                 # fzf pick from all known project directories
tk myapp           # kill "myapp" session
```

### Workspace Configs

Drop a `.tmux-workspace` file in any project directory. When `t` creates a session there, it sources the file automatically.

```bash
# .tmux-workspace — example for a Rails project
tdl cx
tw server "rails server"
tw logs "tail -f log/development.log"
tmux select-window -t :1
```

### Persistence

```bash
tss              # save tmux state as "default"
tss work-friday  # save as "work-friday"
tsr              # restore "default"
tsr work-friday  # restore "work-friday"
```

Saves are stored in `~/.local/share/omacmux/sessions/`. Restores the directory skeleton (sessions, windows, panes with correct working directories) but not running processes.

---

## Window Management

| Command | Description |
|---------|-------------|
| `tw <name> [cmd] [dir]` | Create named window, optionally run a command |
| `twp` | fzf window picker with live preview |
| `to [cmd]` | Popup overlay / scratchpad |

```bash
tw server "rails server"     # window named "server" running rails
tw logs "tail -f log/dev.log"
twp                          # pick a window with fzf
to                           # popup scratchpad shell
to htop                      # popup running htop
```

---

## Layouts

All layout commands run inside tmux.

### `tdl` — Dev Layout

The daily driver. Editor on the left, AI on the right, terminal at the bottom.

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
tdl cx c       # nvim + claude + opencode (stacked) + terminal
```

Proportions configurable via `TDL_AI_WIDTH` (default 30) and `TDL_TERMINAL_HEIGHT` (default 15).

### `tdlm` — Multi-Project Dev Layout

One `tdl` window per subdirectory. Built for monorepos.

```bash
cd ~/projects/my-monorepo && t
tdlm cx    # one window per subdirectory, each with nvim + claude + terminal
```

### `tsl` — Swarm Layout

N tiled panes all running the same command. Parallel AI at scale.

```bash
tsl 4 cx   # 4 tiled panes, each running claude
tsl 4 cxx  # 4 tiled panes, full permissions skip
```

### `tpl` — Pair Layout

Two editors side by side with a shared terminal.

```
┌───────────────────┬───────────────────┐
│                   │                   │
│   nvim (50%)      │   nvim (50%)      │
│                   │                   │
├───────────────────┴───────────────────┤
│           terminal (15%)              │
└───────────────────────────────────────┘
```

### `tml` — Monitor Layout

Main pane on the left, stacked command panes on the right.

```bash
tml "tail -f log/dev.log" "docker stats" "htop"
```

```
┌───────────────────────┬──────────────┐
│                       │ tail -f ...  │
│    main terminal      ├──────────────┤
│       (70%)           │ docker stats │
│                       ├──────────────┤
│                       │    htop      │
└───────────────────────┴──────────────┘
```

---

## Git Worktree Layouts

Worktree-aware layouts for running AI agents across multiple branches simultaneously.

### `twdl` — Worktree Dev Layout

Editor on the left, one AI agent per worktree stacked on the right, terminal at the bottom.

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

### `twsl` — Worktree Swarm Layout

Full-width vertical stack, one AI per worktree. Maximum agent visibility.

```
┌────────────────────────────────────────────┐
│  cx @ master                               │
├────────────────────────────────────────────┤
│  cx @ feat/auth                            │
├────────────────────────────────────────────┤
│  cx @ fix/bug                              │
└────────────────────────────────────────────┘
```

```bash
twsl cxx             # full-width swarm across all worktrees
twsl cx master feat  # specific branches only
```

### `twl` / `twlm` — Worktree Tabs

```bash
twl feat/auth cx    # new tab with full dev layout in feat/auth worktree
twlm cx             # one tab per worktree, each with nvim + claude + terminal
```

### Worktree Management

| Command | Description |
|---------|-------------|
| `gwa <branch> [base]` | Create worktree as sibling directory |
| `gwr [branch]` | Remove worktree (fzf picker if no arg) |
| `gwl` | List all worktrees |
| `gws [branch]` | cd into a worktree (fzf picker if no arg) |
| `twf [path]` | Refocus all AI panes to a specific worktree |

Worktrees are created as sibling directories: `~/myapp` becomes `~/myapp-feat-auth`.

In nvim, press `<leader>gw` to open a telescope worktree picker that switches nvim's cwd and auto-focuses the corresponding agent pane.

---

## Keybindings

### No prefix needed

| Key | Action |
|-----|--------|
| `Ctrl+Option+Arrows` | Navigate between panes |
| `Ctrl+Option+Shift+Arrows` | Resize panes |
| `Option+1-9` | Switch to window 1-9 |
| `Option+Left/Right` | Previous/next window |
| `Option+Up/Down` | Previous/next session |
| `F12` | Toggle nested tmux pass-through |

### With prefix (`Ctrl+B`)

| Key | Action |
|-----|--------|
| `h` | Split below |
| `v` | Split right |
| `x` | Kill pane |
| `c` / `k` / `r` | New / kill / rename window |
| `C` / `K` / `R` | New / kill / rename session |
| `d` | Detach |
| `q` | Reload tmux config |
| `s` | Session picker (fzf popup) |
| `w` | Window picker (fzf popup) |
| `j` | Project jump (fzf popup) |
| `` ` `` | Scratchpad popup |
| `g` | Git log graph (popup) |
| `G` | Worktree switcher (fzf popup) |
| `b` | Branch switcher (fzf popup) |

---

## Aliases

| Alias | Command |
|-------|---------|
| `cx` | `claude` (permissions skip) |
| `cxx` | `claude` (full permissions skip) |
| `c` | `opencode` |
| `n` | `nvim` |
| `g` | `git` |
| `d` | `docker` |
| `ls` | `eza` with icons |
| `ff` | `fzf` with bat preview |

---

## Upgrade / Uninstall

```bash
cd ~/omacmux && ./upgrade.sh     # pull latest, install new deps, link new configs
cd ~/omacmux && ./uninstall.sh   # remove all symlinks, restore backups
```

Homebrew packages are left in place — the uninstaller prints a cleanup command if you want to remove them.

---

## Notes

- **Bash version** — macOS ships bash 3.2. The `tsl` command requires bash 4.3+. The installer offers to set Homebrew bash as your default shell.
- **Mission Control conflicts** — `Ctrl+Option+Shift+Arrows` may conflict with Mission Control. Disable in **System Settings > Keyboard > Keyboard Shortcuts > Mission Control**.
- **First nvim launch** — neovim auto-installs all 46 plugins via lazy.nvim on first run (~30-60s). Subsequent launches are instant.

---

## Credits

Inspired by [omarchy](https://github.com/basecamp/omarchy) by [DHH](https://github.com/dhh) and the Basecamp team.

## License

MIT
