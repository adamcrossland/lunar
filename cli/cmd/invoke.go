package cmd

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var invokeCmd = &cobra.Command{
	Use:   "invoke <function-id>",
	Short: "Execute a function directly",
	Args:  cobra.ExactArgs(1),
	RunE:  runInvoke,
}

var (
	invokeMethod string
	invokeBody   string
)

func init() {
	rootCmd.AddCommand(invokeCmd)
	invokeCmd.Flags().StringVar(&invokeMethod, "method", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	invokeCmd.Flags().StringVar(&invokeBody, "body", "", `Request body (use "-" to read from stdin)`)
}

func runInvoke(cmd *cobra.Command, args []string) error {
	functionID := args[0]
	method := strings.ToUpper(invokeMethod)

	url := serverURL + "/fn/" + functionID

	var bodyReader io.Reader
	if invokeBody == "-" {
		bodyReader = cmd.InOrStdin()
	} else if invokeBody != "" {
		bodyReader = strings.NewReader(invokeBody)
	}

	req, err := http.NewRequestWithContext(cmd.Context(), method, url, bodyReader)
	if err != nil {
		return err
	}
	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Fprintf(cmd.ErrOrStderr(), "HTTP %d  X-Execution-Id: %s  Duration: %sms\n",
		resp.StatusCode,
		resp.Header.Get("X-Execution-Id"),
		resp.Header.Get("X-Execution-Duration-Ms"),
	)

	_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
	return err
}
