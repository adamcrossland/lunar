package cmd

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var llmsCmd = &cobra.Command{
	Use:   "llms",
	Short: "Print the Lua API reference (llms.txt) from the server",
	RunE:  runLLMs,
}

func init() {
	rootCmd.AddCommand(llmsCmd)
}

func runLLMs(cmd *cobra.Command, args []string) error {
	url := serverURL + "/llms.txt"

	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching llms.txt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
	return err
}
