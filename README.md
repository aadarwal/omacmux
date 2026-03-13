# omacmux

A terminal-native, AI-first development environment. Structured tmux layouts with Neovim + Claude Code + Codex, modern CLI tools, and coordinated theming — on any platform.

Inspired by [omarchy](https://github.com/basecamp/omarchy). One command to go from a fresh machine to a fully configured AI development setup.

## Install

### macOS
```bash
git clone https://github.com/aadarwal/omacmux.git ~/omacmux
cd ~/omacmux && ./install.sh
```

### Linux (Ubuntu/Debian, Fedora, Arch, Raspberry Pi)
```bash
git clone https://github.com/aadarwal/omacmux.git ~/omacmux
cd ~/omacmux && ./install.sh
```

### Docker
```bash
docker run -it ghcr.io/aadarwal/omacmux
```

### WSL2
Same as Linux — run `install.sh` inside your WSL2 distribution.

The installer auto-detects your OS and package manager (Homebrew, apt, dnf, pacman).

The installer will:
- Install all dependencies via your system package manager (tmux, neovim, starship, eza, fzf, zoxide, etc.)
- Install JetBrainsMono Nerd Font
- Link all config files to their proper locations (backing up any existing configs)
- Set up git identity
- Optionally set Homebrew bash as your default shell (macOS only)

Open a **new terminal window** after install.

## Layout Commands

All commands run inside tmux. Start tmux first with `t`.

### `tdl <ai> [ai2]` - Dev Layout

Creates a 3-pane development layout:

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

### `t` - Quick tmux

Attaches to existing tmux session or creates a new "Work" session.

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
| `d` | Detach (reattach with `t`) |
| `q` | Reload tmux config |

## Tool Aliases

| Alias | Command |
|-------|---------|
| `t` | `tmux attach \|\| tmux new -s Work` |
| `c` | `opencode` |
| `cx` | `claude` (with permissions skip) |
| `cxx` | `claude` (with full permissions skip) |
| `n` | `nvim` |
| `g` | `git` |
| `d` | `docker` |
| `ls` | `eza` with icons |
| `ff` | `fzf` with bat preview |

## What's Included

- **Ghostty** config with JetBrainsMono Nerd Font, hidden titlebar
- **tmux** config with vi-copy mode, mouse support, blue status bar, cross-platform clipboard integration, nested tmux toggle (F12)
- **Neovim** with LazyVim framework, 46 plugins, tokyonight-night theme, 14 colorschemes available, transparent backgrounds, neo-tree file explorer
- **Starship** prompt with git branch/status indicators
- **Git** config with rebase-on-pull, histogram diffs, rerere, GitHub CLI credential helper
- **Bash** with eza, fzf, zoxide, bat, mise, history search, tab-completion cycling
- **Platform support** for macOS, Linux (x86_64/ARM64), WSL2, Docker, and cloud VMs

## Important Notes

### Bash Version
Linux distributions ship bash 5.x by default, so no action is needed. macOS ships bash 3.2 — the `tsl` command requires bash 4.3+ (for `${array[-1]}` syntax). On macOS, the installer installs Homebrew bash and offers to set it as your default shell. If you skip this, `tsl` won't work until you switch.

### Mission Control Conflicts (macOS)
`Ctrl+Option+Shift+Arrows` (pane resize) may conflict with Mission Control shortcuts. Disable them in:
**System Settings > Keyboard > Keyboard Shortcuts > Mission Control**

### First nvim Launch
On first launch, neovim will auto-install all 46 plugins via lazy.nvim. This takes 30-60 seconds. Subsequent launches are instant.

### Clipboard
tmux auto-detects your clipboard tool: `pbcopy` on macOS, `xclip`/`wl-copy` on Linux, `clip.exe` on WSL.

## Supported Platforms

- macOS (Apple Silicon & Intel)
- Linux x86_64 (Ubuntu, Debian, Fedora, RHEL, Arch)
- Linux ARM64 (Raspberry Pi 4/5, other ARM SBCs)
- Windows (WSL2)
- Docker / GitHub Codespaces

## Uninstall

```bash
cd ~/omacmux && ./uninstall.sh
```

This removes all symlinks and restores backed-up configs. Homebrew packages are kept (instructions to remove them are printed).
