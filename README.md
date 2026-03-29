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

```bash
git clone https://github.com/aadarwal/omacmux.git ~/omacmux
cd ~/omacmux && ./install.sh
```

Open a **new Ghostty window**, then:

```bash
t              # start tmux
tdl cx         # dev layout: nvim + claude + terminal
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
| AI | [Claude Code](https://claude.ai/code), [opencode](https://github.com/sst/opencode), or anything that runs in a terminal |

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

## Keybindings

### No prefix

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
| `d` | Detach |
| `q` | Reload config |
| `s` / `w` / `j` | Session / window / project picker (fzf) |
| `` ` `` | Scratchpad popup |
| `g` / `G` / `b` | Git log / worktree / branch picker |

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

## Install / Upgrade / Uninstall

```bash
./install.sh     # brew deps, symlinks, git identity, shell setup
./upgrade.sh     # pull latest, install new deps, link new configs
./uninstall.sh   # remove symlinks, restore backups
```

---

## Notes

- **Bash 4.3+** required for `tsl`. The installer offers to set Homebrew bash as default.
- **Mission Control** — `Ctrl+Option+Shift+Arrows` may conflict. Disable in System Settings > Keyboard > Keyboard Shortcuts > Mission Control.
- **First nvim launch** auto-installs 46 plugins via lazy.nvim (~30-60s).

---

## Credits

The DHH quote is from his work on [omakub](https://omakub.org) / [omarchy](https://github.com/basecamp/omarchy), which inspired the original direction. omacmux has since diverged into something different — an agent-first research environment rather than a general-purpose dev setup.
