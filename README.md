# gh-csd

A GitHub CLI extension for a seamless codespaces development experience.

## Motivation

GitHub Codespaces offers a powerful cloud-based development environment, but the experience for developers who prefer working via SSH rather than VS Code can be rough around the edges. Connections drop without automatic reconnection, there's no easy way to copy text to your local clipboard or open URLs in your browser, and managing multiple codespaces requires remembering long auto-generated names.

gh-csd addresses these pain points by wrapping the standard `gh cs` commands with quality-of-life improvements. It introduces the concept of a "current codespace" that commands operate on by default, provides automatic SSH reconnection, integrates with [rdm](https://github.com/keith/rdm) for clipboard and open support, and offers conveniences like repository aliases and automatic port forwarding.

## Installation

Install gh-csd as a GitHub CLI extension:

```
gh extension install luanzeba/gh-csd
```

For interactive codespace selection, you'll also need [fzf](https://github.com/junegunn/fzf) installed.

## Quick Start

The typical workflow centers around the "current codespace" concept. First, select a codespace to work with:

```
gh csd select
```

This opens an interactive picker. Once selected, other commands operate on that codespace by default:

```
gh csd ssh
```

When you're done, delete the current codespace:

```
gh csd delete
```

You can also create a new codespace and immediately start working:

```
gh csd create owner/repo --ssh
```

This creates the codespace, sets it as current, and drops you into an SSH session.

## Features

### Automatic SSH Reconnection

Network hiccups and laptop sleep can disconnect your SSH session. With the `--retry` flag, gh-csd automatically reconnects when the connection drops:

```
gh csd ssh --retry
```

You can configure this as the default behavior for specific repositories in your config file, which is particularly useful for repos where you expect long-running sessions.

### Clipboard and Open Support

When you SSH into a codespace, you lose the ability to copy text to your local clipboard or open URLs in your browser. gh-csd integrates with [rdm](https://github.com/keith/rdm) by automatically forwarding the rdm socket during SSH sessions. With rdm running locally, you can use `rdm copy` and `rdm open` from inside your codespace.

### Port Forwarding

Configure ports to automatically forward when connecting to specific repositories. This is useful for web development where you always want localhost:3000 available:

```yaml
repos:
  owner/web-app:
    ports: [3000, 5432]
```

The ports are forwarded in the background when you run `gh csd ssh` and cleaned up when you disconnect.

### Repository Aliases

Define short aliases for repositories you work with frequently:

```yaml
repos:
  github/github:
    alias: "gh"
  my-org/my-long-repo-name:
    alias: "repo"
```

Then create codespaces using the alias:

```
gh csd create gh
```

### Terminal Tab Title

When working with multiple codespaces, it helps to know which one you're connected to. gh-csd can automatically set your terminal tab title when connecting:

```yaml
terminal:
  set_tab_title: true
  title_format: "{short_repo}:{branch}"
```

This works with Ghostty, iTerm2, WezTerm, Alacritty, kitty, and other xterm-compatible terminals.

### Ghostty Terminfo

If you use [Ghostty](https://ghostty.org/), gh-csd automatically copies the terminfo to new codespaces so that terminal features work correctly. This happens during `gh csd create` and requires no configuration.

### Desktop Notifications

Creating a codespace can take a minute or two. When using `gh csd create --ssh`, you'll receive a desktop notification when the codespace is ready and the SSH connection is established. Disable this with `--no-notify` if preferred.

## Commands

| Command | Description |
|---------|-------------|
| `gh csd create <repo>` | Create a new codespace, optionally SSH in with `--ssh` |
| `gh csd ssh` | SSH into the current codespace |
| `gh csd select` | Select a codespace as current (interactive picker) |
| `gh csd get` | Print the current codespace name |
| `gh csd delete` | Delete the current codespace, or use `--list` for multi-select |
| `gh csd config` | View or edit configuration |

Run any command with `--help` for detailed usage information.

## Configuration

Configuration lives at `~/.config/gh-csd/config.yaml`. Create a default configuration file with:

```
gh csd config --init
```

Here's an example configuration showing the available options:

```yaml
defaults:
  machine: "largePremiumLinux"
  idle_timeout: 240
  ssh_retry: false

repos:
  github/github:
    alias: "gh"
    machine: "xLargePremiumLinux"
    ssh_retry: true
    ports: [80]

terminal:
  set_tab_title: true
  title_format: "{short_repo}:{branch}"
```

Settings cascade from defaults to per-repo configuration to command-line flags, with later values taking precedence.

## License

MIT License. See [LICENSE](LICENSE) for details.
