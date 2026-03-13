# omacmux

Omarchy-style tmux dev layouts + full terminal dev environment for macOS + Ghostty.

Ports the [omarchy](https://github.com/basecamp/omarchy) developer experience to macOS: tmux layout commands, LazyVim, Ghostty config, starship prompt, git config, and shell ergonomics. One command to go from a fresh Mac to a fully configured dev setup.

## Install

```bash
git clone https://github.com/aadarwal/omacmux.git ~/omacmux
cd ~/omacmux && ./install.sh
```

The installer will:
- Install all dependencies via Homebrew (tmux, neovim, starship, eza, fzf, zoxide, etc.)
- Install JetBrainsMono Nerd Font
- Link all config files to their proper locations (backing up any existing configs)
- Set up git identity
- Optionally set Homebrew bash as your default shell

Open a **new Ghostty window** after install.

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

### Examples

```bash
t                  # attach to last session or create "Work"
t myapp            # attach to "myapp" or create it
t myapp ~/code/app # create "myapp" rooted at ~/code/app
t .                # session named after git repo root
tj                 # fzf pick from all known project directories
tj myapp           # jump to zoxide-matched "myapp" directory
tk                 # kill current session
tk myapp           # kill "myapp" session
```

## Window Management

| Command | Description |
|---------|-------------|
| `tw <name> [cmd] [dir]` | Create named window, optionally run a command |
| `twp` | fzf window picker with live preview |
| `to [cmd]` | Popup overlay/scratchpad (tmux popup) |

### Examples

```bash
tw server "rails server"     # window named "server" running rails
tw logs "tail -f log/dev.log"
twp                          # pick a window with fzf
to                           # popup scratchpad shell
to htop                      # popup running htop
```

## Layout Commands

All layout commands run inside tmux.

### `tdl <ai> [ai2]` - Dev Layout

Creates a 3-pane development layout. Proportions configurable via `TDL_AI_WIDTH` (default 30) and `TDL_TERMINAL_HEIGHT` (default 15) environment variables.

```
┌──────────────────────┬─────────────┐
│                      │   AI (30%)  │
│     nvim (70%)       │   e.g. cx   │
│                      │             │
├──────────────────────┴─────────────┤
│           terminal (15%)           │
└────────────────────────────────────┘
```

With two AI assistants (`tdl cx c`):

```
┌──────────────────────┬─────────────┐
│                      │  claude (cx)│
│     nvim (70%)       ├─────────────┤
│                      │ opencode (c)│
├──────────────────────┴─────────────┤
│           terminal (15%)           │
└────────────────────────────────────┘
```

### `tdlm <ai> [ai2]` - Multi-Project Dev Layout

Creates one `tdl` window per subdirectory. Great for monorepos:

```bash
cd ~/projects/my-monorepo
t
tdlm cx    # one window per subdirectory, each with nvim + claude + terminal
```

### `tsl <count> <command>` - Swarm Layout

Creates N tiled panes all running the same command. Great for parallel AI:

```bash
tsl 4 cx   # 4 tiled panes, each running claude
tsl 4 cxx  # 4 tiled panes, each running claude (full permissions skip)
```

### `tpl [dir]` - Pair Layout

Two editors side by side with a terminal at bottom:

```
┌───────────────────┬───────────────────┐
│                   │                   │
│   nvim (50%)      │   nvim (50%)      │
│                   │                   │
├───────────────────┴───────────────────┤
│           terminal (15%)              │
└───────────────────────────────────────┘
```

### `tml <cmd1> [cmd2] ...` - Monitor Layout

Main pane on left with stacked command panes on the right:

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

## Git Worktree Layouts

Worktree-aware layouts for running AI agents across multiple branches simultaneously.

### `twdl <ai>` - Worktree Dev Layout

The 2D layout: nvim on the left, vertically stacked AI agents on the right (one per worktree), terminal at the bottom. Each agent is cd'd into a different worktree.

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
twdl cx              # nvim + one claude per worktree (auto-detect all)
twdl cx master feat  # nvim + agents for specific branches only
```

### `twsl <ai>` - Worktree Swarm Layout

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
twsl cxx             # full-width vertical swarm across all worktrees
twsl cx master feat  # swarm for specific branches only
```

### `twl <branch> <ai> [ai2]` - Single Worktree Layout

Creates a worktree (if needed) and opens a `tdl` in a new window tab.

```bash
twl feat/auth cx    # new tab with full dev layout in feat/auth worktree
```

### `twlm <ai> [ai2]` - Multi-Worktree Tab Layout

One `tdl` window per existing worktree.

```bash
twlm cx    # one tab per worktree, each with nvim + claude + terminal
```

## Worktree Management

| Command | Description |
|---------|-------------|
| `gwa <branch> [base]` | Create worktree as sibling directory |
| `gwr [branch]` | Remove worktree (fzf picker if no arg) |
| `gwl` | List all worktrees |
| `gws [branch]` | cd into a worktree (fzf picker if no arg) |
| `twf [path]` | Refocus all AI panes in a `twdl` window to a specific worktree |

Worktrees are created as sibling directories: `~/myapp` → `~/myapp-feat-auth`.

In nvim, press `<leader>gw` to open a telescope worktree picker that switches nvim's cwd and auto-focuses the corresponding agent pane.

## Session Persistence

| Command | Description |
|---------|-------------|
| `tss [name]` | Save current tmux state (all session/window/pane directories) |
| `tsr [name]` | Restore session layout from a save file |

```bash
tss              # save as "default"
tss work-friday  # save as "work-friday"
tsr              # restore "default"
tsr work-friday  # restore "work-friday"
```

Saves are stored in `~/.local/share/omacmux/sessions/`. Restores the directory skeleton (sessions, windows, panes with correct working directories) but not running processes.

## Workspace Configs

Create a `.tmux-workspace` file in any project directory. When `t` creates a new session in that directory, it sources the file automatically.

```bash
# Example .tmux-workspace for a Rails project
tdl cx
tw server "rails server"
tw logs "tail -f log/development.log"
tmux select-window -t :1
```

## Tmux Keybindings

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
| `h` | Split pane below |
| `v` | Split pane right |
| `x` | Kill pane |
| `c` | New window |
| `k` | Kill window |
| `r` | Rename window |
| `C` | New session |
| `K` | Kill session |
| `R` | Rename session |
| `d` | Detach (reattach with `t`) |
| `q` | Reload tmux config |
| `s` | Session picker (fzf popup) |
| `w` | Window picker (fzf popup) |
| `j` | Project jump (fzf popup) |
| `` ` `` | Scratchpad popup |
| `g` | Git log graph (popup) |
| `G` | Worktree switcher (fzf popup) |
| `b` | Branch switcher (fzf popup) |

## Tool Aliases

| Alias | Command |
|-------|---------|
| `c` | `opencode` |
| `cx` | `claude` (with permissions skip) |
| `cxx` | `claude` (with full permissions skip) |
| `n` | `nvim` |
| `g` | `git` |
| `d` | `docker` |
| `ls` | `eza` with icons |
| `ff` | `fzf` with bat preview |

## What's Included

- **Ghostty** config with `macos-option-as-alt`, JetBrainsMono Nerd Font, hidden titlebar
- **tmux** config with vi-copy mode, mouse support, blue status bar with git branch per tab, pbcopy clipboard integration, nested tmux toggle (F12), git/branch/worktree popup pickers
- **Neovim** with LazyVim framework, 46 plugins, tokyonight-night theme, 14 colorschemes available, transparent backgrounds, neo-tree file explorer, telescope worktree picker
- **Starship** prompt with git branch/status indicators, worktree detection
- **Git** config with rebase-on-pull, histogram diffs, rerere, GitHub CLI credential helper, worktree aliases
- **Bash** with eza, fzf, zoxide, bat, mise, history search, tab-completion cycling

## Important Notes

### Bash Version
macOS ships bash 3.2. The `tsl` command requires bash 4.3+ (for `${array[-1]}` syntax). The installer installs Homebrew bash and offers to set it as your default shell. If you skip this, `tsl` won't work until you switch.

### Mission Control Conflicts
`Ctrl+Option+Shift+Arrows` (pane resize) may conflict with Mission Control shortcuts. Disable them in:
**System Settings > Keyboard > Keyboard Shortcuts > Mission Control**

### First nvim Launch
On first launch, neovim will auto-install all 46 plugins via lazy.nvim. This takes 30-60 seconds. Subsequent launches are instant.

## Uninstall

```bash
cd ~/omacmux && ./uninstall.sh
```

This removes all symlinks and restores backed-up configs. Homebrew packages are kept (instructions to remove them are printed).
