# gh-csd Configuration

Configuration file location: `~/.config/gh-csd/config.yaml`

Run `gh csd config` to view current configuration, or `gh csd config --edit` to edit.

## Example Configuration

```yaml
defaults:
  machine: xLargePremiumLinux
  idle_timeout: 240
  devcontainer: .devcontainer/devcontainer.json
  default_permissions: false
  ssh_retry: false
  copy_terminfo: true

repos:
  github/github:
    alias: gh
    machine: xLargePremiumLinux
    default_permissions: true
    ssh_retry: true
    ports:
      - 80
  github/meuse:
    alias: meuse
    ports:
      - 3000
  github/billing-platform:
    alias: bp

hooks:
  post_create:
    - echo "Codespace {name} created for {repo}"

terminal:
  set_tab_title: true
  title_format: "CS: {short_repo}:{branch}"
```

## Configuration Reference

### `defaults`

Global default settings for codespace creation. These can be overridden per-repo.

| Field | Type | Default | gh cs equivalent | Description |
|-------|------|---------|------------------|-------------|
| `machine` | string | `xLargePremiumLinux` | `gh cs create -m` | Machine type for new codespaces |
| `idle_timeout` | int | `240` | `gh cs create --idle-timeout` | Idle timeout in minutes (max 240) |
| `devcontainer` | string | `.devcontainer/devcontainer.json` | `gh cs create --devcontainer-path` | Path to devcontainer config |
| `default_permissions` | bool | `false` | `gh cs create --default-permissions` | Auto-accept codespace permissions without prompting |
| `ssh_retry` | bool | `false` | - | Auto-reconnect SSH on disconnect (gh-csd specific) |
| `copy_terminfo` | bool | `true` | - | Copy Ghostty terminfo after creation (gh-csd specific) |

### `repos`

Per-repository configuration. Settings here override the global defaults.

```yaml
repos:
  owner/repo-name:    # Full repository name
    alias: short      # Short alias for the repo
    machine: string   # Override default machine type
    devcontainer: string  # Override default devcontainer path
    default_permissions: bool  # Override default permissions setting
    ssh_retry: bool   # Override default SSH retry setting
    ports:            # Ports to auto-forward (future feature)
      - 80
      - 3000
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `alias` | string | - | Short name to use instead of full `owner/repo` |
| `machine` | string | (from defaults) | Machine type for this repo |
| `devcontainer` | string | (from defaults) | Devcontainer path for this repo |
| `default_permissions` | bool | (from defaults) | Auto-accept permissions for this repo |
| `ssh_retry` | bool | (from defaults) | Auto-reconnect SSH for this repo |
| `ports` | []int | `[]` | Ports to forward (planned feature) |

#### Example: Trusted vs Untrusted Repos

```yaml
repos:
  # Trusted repo - auto-accept permissions, auto-retry SSH
  github/github:
    alias: gh
    default_permissions: true
    ssh_retry: true

  # Less trusted repo - prompt for permissions, no auto-retry
  random-org/some-repo:
    alias: random
    default_permissions: false
    ssh_retry: false
```

### `hooks`

Commands to run at various lifecycle points. Hooks support placeholder substitution.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `post_create` | []string | `[]` | Commands to run after codespace creation |

#### Available Placeholders

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `{name}` | Codespace name | `super-robot-abc123` |
| `{repo}` | Full repository name | `github/github` |
| `{short_repo}` | Repository name without owner | `github` |
| `{branch}` | Branch name | `main` |

#### Example Hooks

```yaml
hooks:
  post_create:
    # Log creation
    - echo "Created {name} for {repo} on {branch}"
    
    # Run a setup script on the codespace
    - gh cs ssh -c {name} -- ./setup.sh
    
    # Notify via external service
    - curl -X POST "https://api.example.com/notify?cs={name}"
```

### `terminal`

Terminal integration settings for tab titles and other features.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `set_tab_title` | bool | `true` | Set terminal tab title on SSH connect |
| `title_format` | string | `CS: {short_repo}:{branch}` | Format string for tab title |

#### Title Format Placeholders

| Placeholder | Description | Example |
|-------------|-------------|---------|
| `{name}` | Codespace name | `super-robot-abc123` |
| `{repo}` | Full repository name | `github/github` |
| `{short_repo}` | Repository name without owner | `github` |
| `{branch}` | Branch name | `main` |

#### Supported Terminals

Tab titles work with terminals that support OSC escape sequences:
- Ghostty
- iTerm2
- WezTerm
- Alacritty
- kitty
- Apple Terminal
- Most xterm-compatible terminals

## Setting Precedence

Settings are resolved in this order (highest priority first):

1. **Command-line flags** (e.g., `--machine`, `--default-permissions`)
2. **Per-repo config** (e.g., `repos.github/github.machine`)
3. **Global defaults** (e.g., `defaults.machine`)
4. **Built-in defaults** (hardcoded in gh-csd)

### Example

```yaml
defaults:
  machine: smallLinux           # Global default

repos:
  github/github:
    machine: xLargePremiumLinux  # Per-repo override
```

```bash
# Uses xLargePremiumLinux (from per-repo config)
gh csd create gh

# Uses customMachine (flag overrides everything)
gh csd create gh --machine customMachine

# Uses smallLinux (global default, no per-repo config)
gh csd create some-other-repo
```

## Migration from Existing Tools

### From `csw` (codespace wrapper)

The `~/.codespace` file used by csw is replaced by `~/.csd/current`.

| csw command | gh-csd equivalent |
|-------------|-------------------|
| `csw` | `gh csd select` |
| `csw set <name>` | `gh csd select <name>` |
| `csw get` | `gh csd get` |
| `csw ssh` | `gh csd ssh` |
| `csw create ...` | `gh csd create ...` |

### From `dev` script

The `dev` script's repo aliases are now in config:

```yaml
repos:
  github/github:
    alias: gh      # dev gh -> gh csd create gh
  github/meuse:
    alias: meuse   # dev meuse -> gh csd create meuse
  github/billing-platform:
    alias: bp      # dev bp -> gh csd create bp
```

Features carried over:
- Ghostty terminfo copy (`copy_terminfo: true`)
- Desktop notification (built-in)
- rdm socket forwarding (built-in)
- Auto-SSH after creation (built-in)

### From `csd` (delete script)

| csd command | gh-csd equivalent |
|-------------|-------------------|
| `csd` | `gh csd delete` |
| (multi-select) | `gh csd delete` (built-in) |

### From `cssh` alias

| cssh command | gh-csd equivalent |
|-------------|-------------------|
| `cssh` | `gh csd ssh` |
| (rdm forwarding) | Built-in by default |
