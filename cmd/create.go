package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/luanzeba/gh-csd/internal/config"
	"github.com/luanzeba/gh-csd/internal/gh"
	"github.com/luanzeba/gh-csd/internal/state"
	"github.com/spf13/cobra"
)

var (
	createMachine            string
	createDevcontainer       string
	createBranch             string
	createNoSSH              bool
	createNoTerminfo         bool
	createNoNotify           bool
	createDefaultPermissions bool
)

var createCmd = &cobra.Command{
	Use:   "create [repo]",
	Short: "Create a codespace and optionally SSH into it",
	Long: `Create a new codespace for the specified repository.

Repo can be a full name (owner/repo) or an alias defined in config.
If omitted, an interactive picker is shown with repos from config plus a
manual owner/repo entry option.
Workflow:
1. Runs pre-create hooks if defined
2. Creates the codespace
3. Copies Ghostty terminfo for terminal support (configurable)
4. Runs post-create hooks if defined
5. Sends a desktop notification when ready
6. SSHes into the codespace with rdm forwarding

Settings like machine type, permissions, and SSH retry can be configured
per-repo in ~/.config/gh-csd/config.yaml.

Use --no-ssh to just create without connecting.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createMachine, "machine", "m", "", "Machine type (default from config)")
	createCmd.Flags().StringVarP(&createDevcontainer, "devcontainer", "d", "", "Devcontainer path (default from config)")
	createCmd.Flags().StringVarP(&createBranch, "branch", "b", "", "Branch to create codespace from")
	createCmd.Flags().BoolVar(&createNoSSH, "no-ssh", false, "Don't SSH after creation")
	createCmd.Flags().BoolVar(&createNoTerminfo, "no-terminfo", false, "Don't copy Ghostty terminfo")
	createCmd.Flags().BoolVar(&createNoNotify, "no-notify", false, "Don't send desktop notification")
	createCmd.Flags().BoolVarP(&createDefaultPermissions, "default-permissions", "y", false, "Accept default permissions (skip prompt)")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	repoInput := ""
	if len(args) > 0 {
		repoInput = args[0]
	} else {
		selectedRepo, err := selectCreateRepoInteractive(cfg)
		if err != nil {
			return err
		}
		repoInput = selectedRepo
	}

	// Resolve alias to full repo name
	repo := cfg.ResolveAlias(repoInput)
	if !strings.Contains(repo, "/") {
		// Assume it's a GitHub org repo
		repo = "github/" + repo
	}

	fmt.Printf("Creating codespace for %s...\n", repo)

	// Get effective settings: flags override per-repo config, which overrides defaults
	machine := cfg.GetEffectiveMachine(repo)
	if cmd.Flags().Changed("machine") {
		machine = createMachine
	}

	devcontainer := cfg.GetEffectiveDevcontainer(repo)
	if cmd.Flags().Changed("devcontainer") {
		devcontainer = createDevcontainer
	}

	useDefaultPermissions := cfg.GetEffectiveDefaultPermissions(repo)
	if cmd.Flags().Changed("default-permissions") {
		useDefaultPermissions = createDefaultPermissions
	}

	// Run pre-create hooks
	runHooks("pre-create", cfg.Hooks.PreCreate, "", repo, createBranch)

	// Build gh cs create command
	createArgs := []string{"cs", "create",
		"-R", repo,
		"-m", machine,
		"--devcontainer-path", devcontainer,
		"--status",
	}
	if createBranch != "" {
		createArgs = append(createArgs, "-b", createBranch)
	}
	if useDefaultPermissions {
		createArgs = append(createArgs, "--default-permissions")
	}

	// Create the codespace
	ghCreateCmd := exec.Command("gh", createArgs...)
	var stdout bytes.Buffer
	ghCreateCmd.Stdout = &stdout
	ghCreateCmd.Stderr = os.Stderr

	if err := ghCreateCmd.Run(); err != nil {
		return fmt.Errorf("failed to create codespace: %w", err)
	}

	name := strings.TrimSpace(stdout.String())
	if name == "" {
		return fmt.Errorf("no codespace name returned")
	}

	fmt.Printf("Created codespace: %s\n", name)

	// Save as current codespace
	if err := state.Set(name); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save current codespace: %v\n", err)
	}

	// Copy Ghostty terminfo (check both flag and config)
	copyTerminfoEnabled := cfg.GetEffectiveCopyTerminfo() && !createNoTerminfo
	if copyTerminfoEnabled {
		fmt.Println("Copying Ghostty terminfo...")
		if err := copyTerminfo(name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to copy terminfo: %v\n", err)
		}
	}

	// Run post-create hooks
	// Get codespace info for placeholders
	cs, _ := gh.GetCodespace(name)
	branch := ""
	if cs != nil {
		branch = cs.Branch
	}
	runHooks("post-create", cfg.Hooks.PostCreate, name, repo, branch)

	// Send notification
	if !createNoNotify {
		sendNotification("Codespace ready", fmt.Sprintf("✅ %s", name))
	}

	if createNoSSH {
		return nil
	}

	// SSH into the codespace, using per-repo retry setting
	fmt.Println("Connecting...")
	sshNoRdm = false
	sshRetry = cfg.GetEffectiveSSHRetry(repo)

	cs, err = gh.GetCodespace(name)
	if err != nil {
		// Fall back to simple SSH if we can't get codespace info
		return sshOnce(name, cfg, repo)
	}

	if sshRetry {
		return sshWithRetry(name, cs, cfg)
	}
	return sshOnce(name, cfg, repo)
}

type createRepoOption struct {
	label    string
	repo     string
	isManual bool
}

func selectCreateRepoInteractive(cfg *config.Config) (string, error) {
	options := buildCreateRepoOptions(cfg)
	lines := make([]string, 0, len(options))
	lookup := make(map[string]createRepoOption, len(options))
	for _, option := range options {
		lines = append(lines, option.label)
		lookup[option.label] = option
	}

	fzfCmd := exec.Command(
		"fzf",
		"--prompt", "Repo> ",
		"--header", "alias<TAB>repository (select last option to type owner/repo)",
	)
	fzfCmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	fzfCmd.Stderr = os.Stderr

	output, err := fzfCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return "", fmt.Errorf("selection cancelled")
		}
		return "", fmt.Errorf("fzf failed: %w", err)
	}

	selected := strings.TrimSpace(string(output))
	if selected == "" {
		return "", fmt.Errorf("no selection made")
	}

	option, ok := lookup[selected]
	if !ok {
		return "", fmt.Errorf("unexpected selection: %q", selected)
	}

	if option.isManual {
		return promptManualRepo()
	}

	return option.repo, nil
}

func buildCreateRepoOptions(cfg *config.Config) []createRepoOption {
	repos := make([]string, 0, len(cfg.Repos))
	for repo := range cfg.Repos {
		repos = append(repos, repo)
	}
	sort.Strings(repos)

	options := make([]createRepoOption, 0, len(repos)+1)
	for _, repo := range repos {
		repoCfg := cfg.Repos[repo]
		alias := strings.TrimSpace(repoCfg.Alias)
		if alias == "" {
			alias = "-"
		}

		options = append(options, createRepoOption{
			label: fmt.Sprintf("%s\t%s", alias, repo),
			repo:  repo,
		})
	}

	options = append(options, createRepoOption{
		label:    "+\tEnter owner/repo manually",
		isManual: true,
	})

	return options
}

func promptManualRepo() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Repository (owner/repo): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read repository: %w", err)
		}

		repo, err := normalizeManualRepoInput(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}

		return repo, nil
	}
}

func normalizeManualRepoInput(input string) (string, error) {
	repo := strings.TrimSpace(input)
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimPrefix(repo, "http://github.com/")
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.Trim(repo, "/")

	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid repository %q (expected owner/repo)", strings.TrimSpace(input))
	}

	return parts[0] + "/" + parts[1], nil
}

// expandRepoAlias is deprecated - use config.ResolveAlias instead
func expandRepoAlias(alias string) string {
	cfg, _ := config.Load()
	if cfg != nil {
		resolved := cfg.ResolveAlias(alias)
		if resolved != alias {
			return resolved
		}
	}

	// If it looks like a full repo name, use as-is
	if strings.Contains(alias, "/") {
		return alias
	}

	// Assume it's a GitHub org repo
	return "github/" + alias
}

func copyTerminfo(name string) error {
	// Get terminfo from local Ghostty
	infocmp := exec.Command("infocmp", "-x")
	var terminfo bytes.Buffer
	infocmp.Stdout = &terminfo
	if err := infocmp.Run(); err != nil {
		return fmt.Errorf("infocmp failed: %w", err)
	}

	// Pipe to tic on the remote, with retry for transient SSH connection failures
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		sshCmd := exec.Command("gh", "cs", "ssh", "-c", name, "--", "tic", "-x", "-")
		// Need a fresh reader for each attempt since stdin is consumed
		sshCmd.Stdin = bytes.NewReader(terminfo.Bytes())

		// Capture stderr to avoid printing RPC errors on each retry attempt
		var stderr bytes.Buffer
		sshCmd.Stderr = &stderr

		if err := sshCmd.Run(); err != nil {
			lastErr = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
			if attempt < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
		} else {
			return nil
		}
	}

	return lastErr
}

func sendNotification(title, message string) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q sound name "Glass"`, message, title)
		exec.Command("osascript", "-e", script).Run()
	case "linux":
		exec.Command("notify-send", title, message).Run()
	}
}

// runHook executes a hook command with placeholder substitution.
// Supported placeholders: {name}, {repo}, {branch}, {short_repo}
// For pre-create hooks, {name} is empty because the codespace doesn't exist yet.
func runHook(hook, name, repo, branch string) error {
	// Extract short repo name
	shortRepo := repo
	if parts := strings.Split(repo, "/"); len(parts) > 1 {
		shortRepo = parts[len(parts)-1]
	}

	// Replace placeholders
	cmd := hook
	cmd = strings.ReplaceAll(cmd, "{name}", name)
	cmd = strings.ReplaceAll(cmd, "{repo}", repo)
	cmd = strings.ReplaceAll(cmd, "{branch}", branch)
	cmd = strings.ReplaceAll(cmd, "{short_repo}", shortRepo)

	fmt.Printf("Running hook: %s\n", cmd)

	// Execute via shell
	hookCmd := exec.Command("sh", "-c", cmd)
	hookCmd.Stdout = os.Stdout
	hookCmd.Stderr = os.Stderr

	return hookCmd.Run()
}

func runHooks(phase string, hooks []string, name, repo, branch string) {
	for _, hook := range hooks {
		if err := runHook(hook, name, repo, branch); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s hook failed: %v\n", phase, err)
		}
	}
}

// Helper function to check if a codespace with the given repo already exists
func findExistingCodespace(repo string) (*gh.Codespace, error) {
	codespaces, err := gh.ListCodespaces()
	if err != nil {
		return nil, err
	}

	for _, cs := range codespaces {
		if cs.Repository == repo {
			return &cs, nil
		}
	}
	return nil, nil
}
