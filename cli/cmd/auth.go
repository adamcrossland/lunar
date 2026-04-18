package cmd

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/dimiro1/lunar/cli/client"
	"github.com/dimiro1/lunar/cli/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate via device authorization flow",
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the stored authentication token",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	c := mustClient()
	out := cmd.OutOrStdout()

	resp, err := c.DeviceRequestWithResponse(cmd.Context())
	if err != nil {
		return fmt.Errorf("device request: %w", err)
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("device request: %w", apiResponseError(resp.StatusCode(), resp.Body))
	}

	req := resp.JSON200
	fmt.Fprintf(out, "Your verification code: %s\n", req.UserCode)
	fmt.Fprintf(out, "Open this URL to approve: %s\n", req.ApprovalUrl)
	openBrowser(req.ApprovalUrl)
	fmt.Fprint(out, "Waiting for approval")

	interval := time.Duration(req.Interval) * time.Second
	expires := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)

	for time.Now().Before(expires) {
		time.Sleep(interval)

		tokenResp, err := c.DeviceTokenWithResponse(cmd.Context(), &client.DeviceTokenParams{
			Code: req.DeviceCode,
		})
		if err != nil {
			return fmt.Errorf("polling: %w", err)
		}
		if tokenResp.JSON200 == nil {
			fmt.Fprintln(out)
			return fmt.Errorf("polling: %w", apiResponseError(tokenResp.StatusCode(), tokenResp.Body))
		}

		switch tokenResp.JSON200.Status {
		case "approved":
			if tokenResp.JSON200.Token == nil {
				return fmt.Errorf("approved but no token returned")
			}
			cfg, _ := config.Load()
			cfg.Token = *tokenResp.JSON200.Token
				if err := config.Save(cfg); err != nil {
					return fmt.Errorf("saving token: %w", err)
				}
				fmt.Fprintln(out, "\nAuthentication successful. Token saved.")
				return nil
			case "denied":
				fmt.Fprintln(out)
				return fmt.Errorf("authorization denied")
			default:
				fmt.Fprint(out, ".")
			}
	}
	fmt.Fprintln(out)
	return fmt.Errorf("authorization timed out")
}

func runLogout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
	return nil
}

var openBrowser = func(url string) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{url}
	case "windows":
		name, args = "cmd", []string{"/c", "start", url}
	default:
		name, args = "xdg-open", []string{url}
	}
	_ = exec.Command(name, args...).Start()
}
