# Codebase Profile: agent-ab08d4f2

- **Remote:** https://github.com/aadarwal/omacmux.git
- **Total files:** 59

## Languages

- lua (.lua): 11 files
- Shell (.sh): 6 files
- JSON (.json): 5 files
- jpg (.jpg): 3 files
- TOML (.toml): 1 files
- Markdown (.md): 1 files
- hushlogin (.hushlogin): 1 files
- gitignore (.gitignore): 1 files
- conf (.conf): 1 files

## Lines of Code by Directory

- config/: 7591 lines
- mesh/: 26 lines
- shell/: 20 lines
- wallpapers/: 9544 lines

**Total LOC:** 17181

## Directory Structure
```
.
./.claude
./config
./config/bash
./config/claude
./config/ghostty
./config/git
./config/nvim
./config/tmux
./mesh
./shell
./wallpapers
./wallpapers/matte-black
```

## Key Config Files


## README (excerpt)

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


## Recent Commits
```
46aae38 Merge pull request #13 from aadarwal/worktree-agent-a467667d
b738f70 Merge pull request #12 from aadarwal/worktree-agent-ac36c347
a24b15e Merge pull request #11 from aadarwal/worktree-agent-a794be38
0823a42 Merge pull request #10 from aadarwal/worktree-agent-a9b4ff66
0c1c887 Add voice-friendly agent names with NATO phonetic mapping
9ef87aa Merge pull request #9 from aadarwal/worktree-agent-a4bea04a
89bd06c Merge pull request #8 from aadarwal/worktree-agent-a3187423
5a7ffed Add voice-friendly commands (vibe, check, ship, focus, recap)
9afac66 Add audio notification upgrade and sounds utility
4a8bbf3 Add swarm enhancements (swarmx plan, status, merge)
94aebef Add voice mode core (voice on/off/status, TTS, system sounds)
5e43626 Merge pull request #7 from aadarwal/worktree-agent-a309d0f1
bbcf876 Merge pull request #3 from aadarwal/worktree-agent-aeb9fbb3
c02078e Add mid-session agent launch (al, alw) with fzf picker
d7f2fac Add project dashboard (pd, pds) with fzf branch/worktree/swarm overview
a894653 Merge pull request #4 from aadarwal/worktree-agent-a57309fc
0cd4dce Merge origin/master into worktree-agent-a57309fc, resolve tmux.conf conflict
ddb0037 Merge pull request #6 from aadarwal/worktree-agent-af5a2736
08ac16a Merge pull request #1 from aadarwal/feature/autocommit-toggle
```
