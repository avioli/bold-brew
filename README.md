<div align="center">
  <img src="docs/assets/logo/bbrew-logo-rounded.png" alt="Bold Brew Logo" width="180" height="180">
  <h1>Bold Brew</h1>
  <p><strong>A modern Terminal UI for Homebrew, Flatpak, and Mac App Store</strong></p>

  ![GitHub release](https://img.shields.io/github/v/release/Valkyrie00/bold-brew)
  ![License](https://img.shields.io/github/license/Valkyrie00/bold-brew)
  ![Build](https://img.shields.io/github/actions/workflow/status/Valkyrie00/bold-brew/release.yml)
  ![Quality](https://github.com/Valkyrie00/bold-brew/workflows/Quality/badge.svg)
  ![Security](https://github.com/Valkyrie00/bold-brew/workflows/Security/badge.svg)
  ![Downloads](https://img.shields.io/github/downloads/Valkyrie00/bold-brew/total)
  ![Homebrew Installs](https://img.shields.io/badge/dynamic/json?url=https%3A%2F%2Fformulae.brew.sh%2Fapi%2Fformula%2Fbbrew.json&query=%24.analytics.install.30d.bbrew&label=homebrew%20installs%20%2830d%29&logo=homebrew&color=FBB040)

  [Website](https://bold-brew.com/) · [Changelog](https://github.com/Valkyrie00/bold-brew/releases) · [Report Bug](https://github.com/Valkyrie00/bold-brew/issues/new?labels=bug) · [Request Feature](https://github.com/Valkyrie00/bold-brew/issues/new?labels=enhancement)

</div>

---

<div align="center">
  <img src="docs/assets/demo.gif" alt="Bold Brew Demo" width="800">
  <p><em>Browse, search, filter, and manage thousands of packages without leaving the terminal</em></p>
</div>

---

## Why Bold Brew?

Homebrew is powerful, but managing hundreds of packages from the command line is tedious. Bold Brew gives you a **visual interface** to browse, search, and manage everything — formulae, casks, Flatpak apps, and Mac App Store apps — from a single keyboard-driven TUI.

- **See everything at a glance** — installed status, outdated packages, download popularity
- **Curate with Brewfiles** — load local or remote Brewfiles for themed package collections
- **Stay secure** — scan packages for CVEs with one keystroke
- **Works with Homebrew 4, 5, and 6** — leverages the latest features automatically

---

<div align="center">

### Official Homebrew TUI for Project Bluefin

Bold Brew is the **official Terminal UI** for managing Homebrew in [**Project Bluefin**](https://projectbluefin.io/) and [**Aurora**](https://getaurora.dev), next-generation Linux desktops serving tens of thousands of users worldwide.

*"This application features full package management for homebrew in a nice nerdy interface"*
— [Bluefin Documentation](https://docs.projectbluefin.io/command-line/)

[![Project Bluefin](https://img.shields.io/badge/Featured_in-Project_Bluefin-0091e2?style=for-the-badge&logo=linux)](https://projectbluefin.io/)
[![Aurora](https://img.shields.io/badge/Featured_in-Aurora-9b59b6?style=for-the-badge&logo=linux)](https://getaurora.dev)
[![Universal Blue](https://img.shields.io/badge/Part_of-Universal_Blue-5865f2?style=for-the-badge)](https://universal-blue.org/)

</div>

---

## Features

### Package Management
Manage **Homebrew formulae**, **casks**, **Flatpak**, and **Mac App Store** apps from one interface. The actions menu (`a`) installs, updates, removes, reinstalls, fetches, pins, links, and starts or stops services, with confirmation dialogs and output streamed to the Log panel.

### Discovery and Filtering
Fast search across 15,000+ packages. The filter picker (`f`) combines installed, outdated, or leaves with formulae or casks, and the panel title follows along ("Installed Casks"). Jump to a package's dependencies (`gd`) or to what depends on it (`gu`). Sort by download popularity or name. See type indicators `[F]` `[C]` `[M]` at a glance.

### Details at a Glance
The Details panel shows much of what `brew info` does, without running it: caveats, license, tap, pinned and keg-only status, install date, runtime, build and optional dependencies, installed packages that require it, conflicts, and aliases. The go menu (`g`) opens a package's homepage, source, or GitHub definition, or shows `brew info`, `log`, `livecheck`, or `cat` in the Log.

### Fits Your Terminal
Columns size themselves to the available width. When space runs short, Bold Brew trims padding and borders, shortens headings, hides the Details panel (press `Enter` for a popup, or `d` to toggle it), and lets you scroll the Version, Description, and Downloads columns with `h`/`l` while Type and Name stay put. Status messages appear at the bottom, each for at least three seconds.

### Brewfile Workflows
Load Brewfiles from local paths or remote URLs. Batch install/remove entire collections. Export your installed packages to a `~/Brewfile` with one keystroke. Supports `brew`, `cask`, `tap`, `mas`, and `flatpak` entries.

### Security and Health
On-demand **vulnerability scanning** via `brew vulns` (press `a` then `v`). Deprecated and disabled package warnings with replacement suggestions. Full Homebrew 6.0 compatibility including tap trust and ask mode.

---

## Installation

### Quick Install (Recommended)

Install Homebrew + Bold Brew with a single command:

```sh
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Valkyrie00/bold-brew/main/install.sh)"
```

### Via Homebrew

```sh
brew install bbrew
```

### Manual Download

Grab the latest binary from the [releases page](https://github.com/Valkyrie00/bold-brew/releases).

---

## Quick Start

```sh
# Browse all packages
bbrew

# Load a Brewfile (local or remote)
bbrew -f ~/Brewfile
bbrew -f https://raw.githubusercontent.com/user/repo/main/Brewfile
```

See the `examples/` directory for ready-to-use Brewfiles (dev tools, AI tools, K8s, etc.).

Inside the TUI, press `?` for every shortcut, `a` for package actions, `g` for the go menu, and `f` for filters.

---

<details>
<summary><strong>Keyboard Shortcuts</strong></summary>

### Navigation

| Key | Action |
|-----|--------|
| `/` | Search packages |
| `Up/Down`, `j/k`, `Ctrl+N/P` | Move down/up |
| `gg`/`G`, `Home/End`, `Alt+<`/`Alt+>` | Go to top/bottom |
| `Left/Right`, `h/l` | Scroll columns when they do not all fit (Type and Name stay) |
| `Ctrl+D/U` | Half page down/up |
| `Ctrl+F/B`, `PgDn/PgUp`, `Ctrl+V`/`Alt+V` | Page down/up |
| `Tab` / `Shift+Tab` | Switch active panel (table, details, log) |
| `Enter` | Show details (as a popup when the panel is hidden) |
| `Esc` | Back to table; clear a `gd`/`gu` filter; quit from the table |
| `?` | Help screen |
| `q` | Quit |
| `Q`, `Ctrl+C` | Quit from anywhere. If a command is still running, press again to stop it; its log is printed after the screen is restored |

### Go Menu (`g`)

| Key | Action |
|-----|--------|
| `gg` | Go to top |
| `gh` | Open the homepage (`brew home`) |
| `gs` | Open the source repository (`brew source`, formulae, Homebrew 7+) |
| `gI` | Open the formula or cask definition on GitHub (`brew info --github`) |
| `gi` | Show `brew info` in the Log |
| `gd` | Filter to the dependencies, recursively (like `brew deps`); `Esc` clears |
| `gu` | Filter to the packages that depend on it (like `brew uses`); `Esc` clears |
| `gl` | Show `brew log` in the Log |
| `gL` | Show `brew livecheck` in the Log |
| `gC` | Show `brew cat` (the formula or cask source) in the Log |

### Filters and Sorting

| Key | Action |
|-----|--------|
| `f` | Filter picker: combine Installed/Outdated/Leaves with Formulae/Casks |
| `s` | Cycle sort (None, Downloads, Name) |

### Actions Menu (`a`)

| Key | Action |
|-----|--------|
| `ai` | Install selected |
| `au` | Update selected |
| `ar` | Remove selected |
| `aI` | Reinstall selected |
| `aU` | Update all outdated |
| `aF` | Fetch (download without installing) |
| `ap` / `aP` | Pin / unpin a formula (pinned formulae are skipped by Update all) |
| `al` / `aL` | Link / unlink a formula |
| `as` / `aS` / `aT` | Start / stop / restart a formula's service (`brew services`) |
| `av` | Vulnerability scan |
| `ae` | Export installed packages to ~/Brewfile |

### Brewfile Mode

| Key | Action |
|-----|--------|
| `Ctrl+A` | Install all from Brewfile |
| `Ctrl+R` | Remove all from Brewfile |

### View

| Key | Action |
|-----|--------|
| `b` | Toggle borders (also hidden automatically on narrow terminals) |
| `d` | Toggle the details panel (also hidden automatically on narrow terminals) |

</details>

---

## Screenshots

<div align="center">
  <img src="docs/assets/screenshots/bbrew-search-screenshot.png" alt="Search" width="800">
  <p><em>Fast search across all packages</em></p>

  <img src="docs/assets/screenshots/bbrew-brewfile-screenshot.png" alt="Brewfile Mode" width="800">
  <p><em>Brewfile mode with curated package selection</em></p>
</div>

---

## Compatibility

| Homebrew | Support | Highlights |
|----------|---------|------------|
| 4.x – 5.x | Full | JSON v1 API, standard operations |
| **6.0+** | Full + Enhanced | JSON v2, tap trust, ask mode, `brew vulns` |

| Platform | Support |
|----------|---------|
| macOS (Apple Silicon + Intel) | Full |
| Linux (x86_64 + ARM64) | Full |

---

## Security

- **`brew vulns`** — On-demand CVE scanning for installed packages (press `a` then `v` in the TUI)
- **govulncheck** — Go dependency vulnerability scanning in CI
- **gosec** — Static security analysis in CI
- **Gated releases** — Tests + lint must pass before any release is published

Found a vulnerability? Report it via [GitHub Security Advisories](https://github.com/Valkyrie00/bold-brew/security/advisories).

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for setup instructions, project structure, and guidelines.

Quick start:

```sh
git clone https://github.com/Valkyrie00/bold-brew.git && cd bold-brew
make build-local && make test quality-local
```

## Contributors

<a href="https://github.com/Valkyrie00/bold-brew/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Valkyrie00/bold-brew" />
</a>

## License

MIT — see [LICENSE](LICENSE) for details.

---

<div align="center">
  <sub>Built with care for the Homebrew community</sub>
  <br><br>

  [![GitHub stars](https://img.shields.io/github/stars/Valkyrie00/bold-brew?style=social)](https://github.com/Valkyrie00/bold-brew/stargazers)
  [![GitHub forks](https://img.shields.io/github/forks/Valkyrie00/bold-brew?style=social)](https://github.com/Valkyrie00/bold-brew/network/members)

</div>
