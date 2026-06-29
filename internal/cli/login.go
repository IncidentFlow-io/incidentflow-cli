package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/incidentflow/incidentflow-cli/internal/api"
	"github.com/incidentflow/incidentflow-cli/internal/config"
	"github.com/incidentflow/incidentflow-cli/internal/output"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with IncidentFlow",
	Long: `Authenticate with IncidentFlow using browser-based device login.

Opens a browser to the IncidentFlow login page. If you are not yet signed in,
the existing Google SSO flow handles authentication. After signing in, approve
the CLI access request. Credentials are saved automatically.

Examples:
  incidentflow login              # production (default)
  incidentflow login --env dev    # dev environment
  incidentflow login --env local  # localhost`,
	RunE: func(cmd *cobra.Command, args []string) error {
		envName, _ := cmd.Flags().GetString("env")
		tokenFlag, _ := cmd.Flags().GetString("token")
		workspaceFlag, _ := cmd.Flags().GetString("workspace")

		envCfg, err := config.EnvURLs(envName)
		if err != nil {
			return err
		}

		cfg, _ := config.Load()
		if cfg == nil {
			cfg = &config.Config{}
		}
		cfg.Env = envName
		cfg.AppURL = envCfg.AppURL
		cfg.APIURL = envCfg.APIURL

		// Hidden --token fallback for CI / dev environments.
		if tokenFlag != "" {
			return loginWithToken(cfg, tokenFlag, workspaceFlag)
		}

		return loginWithDevice(cfg)
	},
}

// loginWithDevice runs the browser-based device login flow.
func loginWithDevice(cfg *config.Config) error {
	client := api.NewClient(cfg.APIURL, "", "")

	// 1. Start device session.
	ctx := context.Background()
	start, err := client.DeviceStart(ctx)
	if err != nil {
		return fmt.Errorf("could not start login session: %w", err)
	}

	// 2. Show user code and open browser.
	output.Info(fmt.Sprintf("\nConfirmation code: %s\n", start.UserCode))
	output.Info(fmt.Sprintf("Opening browser: %s\n", start.VerificationURIComplete))
	_ = openBrowser(start.VerificationURIComplete)
	output.Info("Waiting for authorization in the browser…\n")

	// 3. Poll until approved, denied, expired, or local timeout.
	interval := time.Duration(start.Interval) * time.Second
	if interval < 3*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(start.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		resp, err := client.DevicePoll(ctx, start.DeviceCode)
		if err != nil {
			// expired or denied returns an API error
			errStr := err.Error()
			if strings.Contains(errStr, "device_code_expired") || strings.Contains(errStr, "expired") {
				return fmt.Errorf("login timed out. Run:\n  incidentflow login")
			}
			if strings.Contains(errStr, "device_code_denied") || strings.Contains(errStr, "denied") {
				return fmt.Errorf("authorization was denied in the browser")
			}
			// transient network error — keep polling
			continue
		}

		if resp.Status == "pending" {
			continue
		}

		result, err := resp.Approved()
		if err != nil {
			return fmt.Errorf("parsing login response: %w", err)
		}
		if result == nil {
			continue
		}

		// 4. Save credentials.
		cfg.Token = result.Tokens.AccessToken
		cfg.Workspace = result.Workspace
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		output.Success(fmt.Sprintf("Logged in as %s", result.Email))
		if result.Workspace != "" {
			output.Success(fmt.Sprintf("Workspace: %s", result.Workspace))
		}
		output.Success(fmt.Sprintf("Environment: %s (%s)", cfg.Env, cfg.APIURL))
		return nil
	}

	return fmt.Errorf("login timed out. Run:\n  incidentflow login")
}

// loginWithToken is the hidden fallback for CI/dev — skips browser.
func loginWithToken(cfg *config.Config, token, workspace string) error {
	if workspace == "" {
		workspace = prompt("Workspace slug: ")
	}

	client := api.NewClient(cfg.APIURL, token, workspace)
	whoami, err := client.WhoAmI(context.Background())
	if err != nil {
		return fmt.Errorf("login failed: %w\n\nMake sure your token is correct.", err)
	}

	cfg.Token = token
	cfg.Workspace = workspace
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	output.Success(fmt.Sprintf("Logged in as %s", whoami.Email))
	output.Success(fmt.Sprintf("Workspace: %s", workspace))
	output.Success(fmt.Sprintf("Environment: %s (%s)", cfg.Env, cfg.APIURL))
	return nil
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove local credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cfg.Token = ""
		cfg.Workspace = ""
		if err := config.Save(cfg); err != nil {
			return err
		}
		output.Success("Logged out")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if !cfg.IsAuthenticated() {
			return fmt.Errorf("not logged in. Run:\n  incidentflow login")
		}

		client := api.NewClient(cfg.APIURL, cfg.Token, cfg.Workspace)
		whoami, err := client.WhoAmI(context.Background())
		if err != nil {
			return err
		}

		fmt.Printf("Email:       %s\n", whoami.Email)
		fmt.Printf("Workspace:   %s\n", cfg.Workspace)
		fmt.Printf("Environment: %s\n", cfg.Env)
		fmt.Printf("App URL:     %s\n", cfg.AppURL)
		fmt.Printf("API URL:     %s\n", cfg.APIURL)
		return nil
	},
}

func init() {
	loginCmd.Flags().String("env", config.EnvProd, "Environment (prod, dev, local)")
	// --token and --workspace are intentionally hidden; used only in CI/dev.
	loginCmd.Flags().String("token", "", "")
	loginCmd.Flags().String("workspace", "", "")
	_ = loginCmd.Flags().MarkHidden("token")
	_ = loginCmd.Flags().MarkHidden("workspace")
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform")
	}

	return exec.Command(cmd, args...).Start()
}

func prompt(label string) string {
	fmt.Print(label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptSecret(label string) string {
	fmt.Print(label)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}
	return string(b)
}
