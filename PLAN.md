# anu — Engine + Flavors Plan

> Where this project is going, and why. Read top-to-bottom. Each phase is
> independently shippable; stop at any phase if priorities shift.

---

## 1. Thesis

anu is **an agent-first IDE built on tmux**. Today it's labeled
"macOS-only" because the install path uses Homebrew, the terminal is Ghostty,
and voice/sounds/clipboard call macOS-specific binaries (`say`, `afplay`,
`pbcopy`, `osascript`).

But the *thing that makes anu interesting* — the swarm system, the layouts,
the worktrees, the review system, the agent
communication — is pure bash + tmux + jq + fzf + git. None of it knows or
cares that it's running on macOS.

**The plan is to name that fact in the architecture.**

We split the codebase into:

- **`engine/`** — the universal core. Knows nothing about any OS. Calls
  abstract verbs (`_say`, `_sound`, `_clip`, `_notify`, `_pkg_install`,
  `_stat_mtime`). Runs identically on every platform.
- **`flavors/<name>/`** — opinionated platform implementations. Each flavor
  defines the abstract verbs concretely and ships its own installer + terminal
  config + package list.

The Mac flavor stays as opinionated as it is today (Ghostty + Homebrew + `say`
+ Apple system sounds — nothing changes for Mac users). The Linux flavor (or
flavors — Arch first, then Debian/Fedora) becomes a real first-class product,
naturally aligned with omarchy.

---

## 2. Why engine + flavors, not "universal"

The simpler approach is to keep one tree and add `case "$(uname)"` branches
inside each function that touches the OS. That works. We're not doing it,
because:

| Property | Universal (one tree, conditionals) | Engine + flavors |
|---|---|---|
| `if uname` location | Scattered in fns, or in shim | Zero in engine; one boot-time pick |
| Adding a new platform | Audit every shim function | Drop in `flavors/<new>/` |
| Engine ships alone | No (entangled with flavor code) | Yes — independent artifact |
| Drift on unused platforms | Silent | Visible — flavor either loads or doesn't |
| Distribution | One package, runtime dispatch | `brew install anu` and `pacman -S anu-arch` are separate |
| Mental model | "anu works on Linux too" | "anu-engine is the thesis. Pick a flavor." |
| Forks | Fork the whole repo | Write a 30-line `flavors/<myflavor>/` |

Engine + flavors is just *the universal approach taken seriously*. The shim
file in the universal version is the proto-engine boundary. We're naming that
boundary, giving it a directory, and making it the public interface.

---

## 3. Current state (concrete numbers)

```
Total shell + tmux + CLI lines:  12,948
Platform-touching lines:             45
Engine ratio:                     99.65%
```

The 45 platform-touching lines live in 6 surfaces:

| Surface | macOS impl | Files | Lines |
|---|---|---|---|
| TTS | `say -r 200` | `config/bash/fns/voice:28-29`, `fns/sounds:39` | 4 |
| Sound effects | `afplay /System/Library/Sounds/*.aiff` | `fns/voice:36-45`, `fns/sounds:31` | 10 |
| Clipboard | `pbcopy` | `config/tmux/tmux.conf:11`, `fns/git:49`, `fns/utils:70` | 3 |
| Notifications | `osascript -e 'display notification ...'` | `config/claude/notify.sh` | 2 |
| Installer / shell | `brew bundle`, `/opt/homebrew/bin/bash` | `bin/anu:19,100-115,470-565`, `Brewfile`, `install.sh:13-16` | ~25 |
| Ghostty Mac keys | `macos-option-as-alt`, `macos-titlebar-style`, `window-save-state` | `config/ghostty/config:21-23` | 3 |

Everything else is engine. Tally of what's already universal:

- **Layouts** (`fns/tmux`, 583 lines): `tdl`, `tsl`, `twdl`, `twsl`, `tdlm`,
  `tpl`, `tml`, `taa`, `tra`, `tscale`, `tss`/`tsr`. Pure tmux 3.x.
- **Swarm** (`fns/swarm`, 2956 lines + `swarmext`, 411 lines): topologies,
  messaging, capture, collect, hub. Bash + tmux + jq + fzf.
- **Mesh/distributed** (`fns/mesh`, 1399 lines): runs over `tailscale ssh`.
  Already heterogeneous; Mac↔Linux works today at `fns/swarm:253` and `:1522`.
- **Review** (472) and **worktrees** (308): pure bash + git + jq.
- **tmux.conf** (183 lines): one `pbcopy` line is the only OS-ism.
- **Neovim/LazyVim, starship, fzf, zoxide, eza, bat, ripgrep, fd, mise, gh,
  jq**: every dep in `Brewfile` is first-class on Linux.

---

## 4. The engine contract

The engine never calls platform-specific binaries directly. It only calls
these verbs, defined by whichever flavor is loaded at boot:

| Verb | Purpose | macOS impl | Linux impl |
|---|---|---|---|
| `_say "<text>"` | TTS, fire-and-forget | `say -r 200 "$*" &` | `piper`, `espeak-ng`, or `spd-say` (or no-op if none) |
| `_sound <event>` | Play a named sound (`done`, `error`, `start`, `tick`, `warning`, `finish`) | `afplay /System/Library/Sounds/<map>.aiff` | `paplay $ANU_PATH/flavors/linux/sounds/<event>.wav` |
| `_clip` | Read stdin into clipboard | `pbcopy` | `wl-copy` ‖ `xclip -sel clip` ‖ OSC52 fallback |
| `_paste` | Write clipboard to stdout | `pbpaste` | `wl-paste` ‖ `xclip -o` |
| `_notify "<title>" "<body>"` | Desktop notification | `osascript -e 'display notification ...'` | `notify-send` |
| `_pkg_install <name>...` | Install OS packages | `brew install` (from `Brewfile` map) | `pacman -S` ‖ `apt install` ‖ `dnf install` |
| `_stat_mtime <path>` | Print file mtime, ISO format | `stat -f "%Sm" -t "%Y-%m-%d"` | `stat -c "%y"` (parse to date) |
| `_default_shell` | Set bash 5 as login shell | brew bash + `/etc/shells` dance | no-op (Linux already has bash 5) |

The contract is **small on purpose**. Anything more specific than these eight
verbs leaks platform knowledge into the engine.

Every flavor MUST implement every verb. Verbs may be no-ops (e.g. headless
flavor's `_say`/`_sound` do nothing) but they must exist so the engine never
has to check.

---

## 5. Target layout

```
anu/
├── PLAN.md                          # this file
├── README.md                        # rewritten — engine first, flavors below
│
├── engine/                          # the universal core (~12,900 lines)
│   ├── config/
│   │   ├── bash/
│   │   │   ├── fns/                 # all functions, OS-agnostic
│   │   │   ├── aliases
│   │   │   ├── envs
│   │   │   ├── init
│   │   │   ├── shell
│   │   │   ├── inputrc
│   │   │   └── platform.sh          # source flavors/$ANU_FLAVOR/platform.sh
│   │   ├── tmux/                    # tmux.conf, pane-swap.sh, tile.sh, session-bar.sh
│   │   ├── nvim/                    # LazyVim
│   │   ├── starship.toml
│   │   ├── git/config
│   │   ├── claude/settings.json     # notify.sh moves to flavor
│   ├── bin/anu                  # CLI; installer logic factored into _flavor_install
│   ├── shell/bashrc                 # sources engine + selected flavor
│   ├── shell/bash_profile
│   ├── links.sh                     # link manifest, flavor-aware
│   └── CONTRACT.md                  # spec for the 8 verbs
│
├── flavors/
│   ├── darwin/
│   │   ├── platform.sh              # _say, _sound, _clip, _paste, _notify, _pkg_install, _stat_mtime, _default_shell
│   │   ├── ghostty/config           # full Mac config with macos-* keys
│   │   ├── Brewfile                 # canonical Mac package list
│   │   ├── claude-notify.sh         # osascript implementation
│   │   ├── install.sh               # brew bootstrap + /etc/shells dance
│   │   └── README.md
│   │
│   ├── arch/                        # omarchy-aligned first Linux flavor
│   │   ├── platform.sh
│   │   ├── ghostty/config           # same minus 3 macos-* keys
│   │   ├── packages.txt             # one-pkg-per-line, mapped from Brewfile
│   │   ├── claude-notify.sh         # notify-send
│   │   ├── sounds/{done,error,start,tick,warning,finish}.wav
│   │   ├── install.sh               # pacman dispatch; offers AUR for non-pacman pkgs
│   │   ├── PKGBUILD                 # for AUR distribution
│   │   └── README.md
│   │
│   ├── debian/                      # apt-based; identical structure
│   ├── fedora/                      # dnf-based; identical structure
│   └── headless/                    # SSH/server: _say/_sound = no-op, _clip = OSC52
│
└── tests/
    ├── engine/                      # tests that work in any flavor (Docker matrix)
    └── flavors/<name>/              # flavor-specific install + smoke tests
```

Selection at boot:

```bash
# engine/config/bash/platform.sh
: "${ANU_FLAVOR:=$(cat "$ANU_STATE/flavor" 2>/dev/null || \
  case "$(uname -s)" in Darwin) echo darwin ;; Linux) echo arch ;; esac)}"
source "$ANU_PATH/flavors/$ANU_FLAVOR/platform.sh"
```

`anu init` writes `$ANU_STATE/flavor` so the choice is sticky and
overridable (`ANU_FLAVOR=headless ssh remote-host` Just Works).

---

## 6. Migration phases

Each phase ends in a green commit. Stop at any phase.

### Phase 0 — Define the contract (no code changes)

**Output:** `engine/CONTRACT.md` (or stage at repo root and move later).
Document the 8 verbs, signatures, no-op semantics, and how flavors are
selected. This is the spec the engine codes to.

**Why first:** the contract is the only thing in this plan that needs
careful design. Everything else is mechanical once it's frozen.

**Done when:** a teammate (or future-you) can read CONTRACT.md and write a new
flavor without reading any other code.

---

### Phase 1 — Extract the platform shim in place

**No directory restructure yet.** Add the shim alongside existing files. Mac
behavior stays bit-identical.

1. Create `config/bash/platform.sh` (the loader) and
   `config/bash/platforms/darwin.sh` (the Mac backend).
2. Convert callsites:
   - `fns/voice:28-29,36-45` → `_say`, `_sound`
   - `fns/sounds:31,39` → `_sound`, `_say`
   - `fns/utils:70` → `_clip`
   - `fns/git:49` → `_clip`
   - `config/tmux/tmux.conf:11` → bind to a small wrapper script that calls `_clip`
   - `config/claude/notify.sh` → call `_notify` (move osascript body into `darwin.sh`)
3. Source `platform.sh` from `config/bash/init` so verbs are loaded into every
   shell.

**Done when:** voice/sounds/clipboard/notifications work identically on Mac,
with zero direct calls to `say`/`afplay`/`pbcopy`/`osascript` in the engine
code path. `grep -rE 'say|afplay|pbcopy|osascript|/System/Library' config/`
returns hits *only* in `config/bash/platforms/darwin.sh`.

---

### Phase 2 — Add the Linux backend + installer dispatch

1. `config/bash/platforms/linux.sh` implementing all 8 verbs:
   - `_say`: `command -v piper && piper --voice ... <<< "$*"` ‖ `espeak-ng` ‖ no-op
   - `_sound`: `paplay "$ANU_PATH/.../sounds/$1.wav"`
   - `_clip`: `wl-copy` ‖ `xclip -sel clip` ‖ OSC52 via `tmux load-buffer`
   - `_paste`: mirror
   - `_notify`: `notify-send "$1" "$2"`
   - `_pkg_install`: detect `pacman`/`apt`/`dnf`, map from a package alias table
   - `_stat_mtime`: GNU `stat -c "%y"` parsed to ISO date
   - `_default_shell`: no-op
2. Bundle royalty-free `.wav` files for the 6 sound events (or generate
   tones with `sox`).
3. `bin/anu`: replace the macOS guard and the
   `HOMEBREW_NO_AUTO_UPDATE=1 brew bundle` block with `_pkg_install` driven by
   a `packages.toml` that maps `Brewfile` entries to per-distro names.
4. `install.sh`: drop the `[[ "$(uname)" != "Darwin" ]] && exit 1` and
   dispatch to the right flavor installer.

**Done when:** on a fresh Arch container, `git clone && ./install.sh && anu init`
produces a working shell where `t`, `tdl cx`, `swarm start 4 cx`,
`who`, `tell alpha "hi"`, and `recap` all work. Mac behavior unchanged.

---

### Phase 3 — Restructure into `engine/` + `flavors/`

This is mechanical once Phases 1–2 are clean.

1. `git mv config/{bash,tmux,nvim,starship.toml,git,claude} engine/config/`
2. `git mv bin shell links.sh engine/`
3. `git mv config/bash/platforms/darwin.sh flavors/darwin/platform.sh`;
   move `Brewfile`, `config/ghostty/config`, the macOS `claude/notify.sh`
   into `flavors/darwin/`.
4. `git mv config/bash/platforms/linux.sh flavors/arch/platform.sh`; move
   sounds, packages list, etc.
5. Update `engine/links.sh` so symlink targets are relative to whichever
   flavor is active.
6. Update `engine/config/bash/platform.sh` to source from
   `$ANU_PATH/flavors/$ANU_FLAVOR/`.

**Done when:** `engine/` contains zero references to specific platforms,
`grep -r 'Darwin\|brew\|pbcopy\|afplay\|/System/Library' engine/` is empty,
and both Mac + Arch installs still work.

---

### Phase 4 — Distribute as separate flavors

1. **Mac:** keep the existing `aadarwal/homebrew-tap` formula. Update it to
   install `engine/` and the `darwin/` flavor.
2. **Arch:** publish `flavors/arch/PKGBUILD` to AUR as `anu` (with
   `anu-engine` as a dep). Tag culturally as omarchy-friendly.
3. **Generic Linux:** `curl -fsSL anu.dev/install.sh | bash` that detects
   distro and pulls the right flavor.
4. Rewrite `README.md`:
   - Lead with "anu is an agent-first IDE built on tmux. Engine runs
     anywhere; flavors give you the chrome."
   - Three install paths: Mac (brew), Arch (AUR), generic (curl).
   - Daily-driver section is shared. Per-flavor sections cover the chrome.

**Done when:** a fresh Arch user can `pacman -S anu` (after AUR setup),
boot tmux, and have a working swarm in under 5 minutes.

---

## 7. Acceptance tests (run after each phase)

A small fixed-cost smoke suite that validates the contract:

```bash
# tests/engine/smoke.sh — runs in any flavor
t test-session
tdl cx                       # layout creates 3 panes
swarm start 2 echo           # 2 workers with stub command
swarm broadcast "ping"       # message delivery
swarm capture agent-1        # output capture
swarm kill                   # clean teardown
who                          # NATO names render
recap                        # AI summary survives no-op git state
_say "hello" && _sound done && echo x | _clip   # contract verbs return 0
```

Run on:
- macOS host (Phase 1+)
- Arch container (Phase 2+)
- Debian container (Phase 4)
- SSH-only headless box (Phase 4)

If any of those fail without an explicit "this flavor does not implement X"
exit code, the contract is wrong, not the test.

---

## 8. Decisions to make (before Phase 1 ends)

These are the choices that shape the contract. Pick before writing the shim:

1. **Flavor selection precedence.** `ANU_FLAVOR` env var → `$ANU_STATE/flavor` file → uname default. Confirm this order; future-you will want override-by-env for SSH cases.
2. **Voice quality on Linux.** Default off, or default on with `espeak-ng`?
   `piper` produces near-`say` quality but is a separate install. Recommendation:
   default off; document `piper` as the "premium" install. Mac stays default-on.
3. **Sound assets.** Bundle `.wav` files per flavor, or use `tput bel`?
   Recommendation: bundle six small (~5KB each) royalty-free `.wav`s in
   `flavors/<linux>/sounds/`. Consistent with Mac UX.
4. **Ghostty on Linux.** Ship a Linux Ghostty config, or stay terminal-agnostic?
   Recommendation: ship Ghostty config (works on Linux now, Wayland + X11)
   AND document Alacritty/Kitty/WezTerm/Foot as supported. The terminal isn't
   load-bearing for the engine; it just needs truecolor + Nerd Font.
5. **Where Brewfile lives.** Keep at repo root for backwards compat, or move
   into `flavors/darwin/`? Recommendation: move. Backwards compat is a
   non-goal; clean structure wins.
6. **Headless flavor — Phase 4 or skip?** It's the cheapest test of whether
   the contract is right. Recommendation: ship it in Phase 4. 60 lines, big
   payoff for SSH/server use.

---

## 9. Non-goals

Things explicitly **not** in scope, to keep the plan honest:

- **Windows / WSL native flavor.** WSL works through the Linux flavor;
  native Windows is its own multi-month project (cmd/powershell/tmux-on-Windows
  is a different beast). Out of scope.
- **GUI frontend.** anu is terminal-first. No separate GUI frontend is in
  scope.
- **Replacing tmux.** Zellij/wezterm-multiplexer/screen are not on the
  roadmap. tmux is the engine's hard runtime dep. The contract pretends
  there's one, and there is.
- **Mac feature regression.** Nothing in this plan changes the Mac
  experience. If a Mac user notices a difference after Phase 3 ships, that's
  a bug.
- **Per-fn platform tweaks.** Once the 8 verbs are defined, we resist adding
  a 9th. If a function needs platform knowledge beyond the contract, refactor
  it to use the existing verbs or move it into the flavor.

---

## 10. The shape of "done"

When this plan is fully executed, a stranger reading the repo sees:

```
$ tree -L 2 anu/
anu/
├── engine/             # one paragraph: "this is the agent IDE thesis"
├── flavors/            # one line each: "Mac chrome", "Arch chrome", ...
├── PLAN.md             # this file
└── README.md           # opens with the thesis, ends with three install paths
```

They can read `engine/CONTRACT.md` in five minutes and write a new flavor.
They can read any file under `engine/` without thinking about an OS. They can
install on whichever distro they run, get the same swarm experience, and put
their machine on a Tailscale mesh with someone else's heterogeneous machine,
and the swarm just works.

That's the goal. Everything in this file is in service of it.
