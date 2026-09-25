package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/arpitbhalla1801/localpilot/internal/platform"
)

// diagnoseResult is the --json payload for diagnose.
type diagnoseResult struct {
	Target     string  `json:"target"`
	Status     string  `json:"status"` // open, refused, timeout or error
	Error      string  `json:"error,omitempty"`
	LatencyMs  float64 `json:"latencyMs,omitempty"`
	HTTPStatus int     `json:"httpStatus,omitempty"`
	HTTPError  string  `json:"httpError,omitempty"`
	Owner      string  `json:"owner,omitempty"` // local targets only
}

var (
	diagnoseJSON    bool
	diagnoseHTTP    bool
	diagnoseTimeout time.Duration
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose <host:port>",
	Short: "Check whether a host:port is reachable (TCP, optionally HTTP)",
	Example: "  localpilot diagnose localhost:3000\n" +
		"  localpilot diagnose localhost:3000 --http\n" +
		"  localpilot diagnose example.com:443 --timeout 5s --json",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		host, portStr, err := net.SplitHostPort(args[0])
		if err != nil {
			return fmt.Errorf("invalid target %q: expected host:port", args[0])
		}
		port, err := parsePort(portStr)
		if err != nil {
			return err
		}

		res := diagnose(cmd.Context(), host, port, diagnoseTimeout, diagnoseHTTP)

		if diagnoseJSON {
			if err := printJSON(cmd.OutOrStdout(), res); err != nil {
				return err
			}
		} else {
			printDiagnose(cmd.OutOrStdout(), res)
		}
		if res.Status != "open" {
			return fmt.Errorf("%s is not reachable (%s)", res.Target, res.Status)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)
	diagnoseCmd.Flags().BoolVar(&diagnoseJSON, "json", false, "Output as JSON")
	diagnoseCmd.Flags().BoolVar(&diagnoseHTTP, "http", false, "Also send an HTTP GET and report the status code")
	diagnoseCmd.Flags().DurationVar(&diagnoseTimeout, "timeout", 3*time.Second, "Timeout for each check")
}

func diagnose(ctx context.Context, host string, port int, timeout time.Duration, checkHTTP bool) diagnoseResult {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	res := diagnoseResult{Target: addr}

	start := time.Now()
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		res.Error = err.Error()
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			res.Status = "timeout"
		} else if strings.Contains(err.Error(), "refused") {
			res.Status = "refused"
		} else {
			res.Status = "error"
		}
	} else {
		conn.Close()
		res.Status = "open"
		res.LatencyMs = float64(time.Since(start).Microseconds()) / 1000
	}

	if isLocalHost(host) {
		if b, err := platform.FindListeningPort(ctx, port); err == nil && b.InUse {
			if c := platform.ListDockerContainerPorts(ctx)[port]; c != nil {
				res.Owner = "container " + c.Name
			} else if b.Process != nil {
				res.Owner = fmt.Sprintf("%s (PID %d)", b.Process.Name, b.Process.PID)
			}
		}
	}

	if checkHTTP && res.Status == "open" {
		client := &http.Client{Timeout: timeout}
		resp, herr := client.Get("http://" + addr + "/")
		if herr != nil {
			res.HTTPError = herr.Error()
		} else {
			resp.Body.Close()
			res.HTTPStatus = resp.StatusCode
		}
	}
	return res
}

func isLocalHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}

func printDiagnose(w io.Writer, r diagnoseResult) {
	fmt.Fprintf(w, "Target          %s\n", r.Target)
	fmt.Fprintf(w, "TCP             %s", r.Status)
	if r.Status == "open" {
		fmt.Fprintf(w, " (%.1f ms)", r.LatencyMs)
	}
	fmt.Fprintln(w)
	if r.Error != "" {
		fmt.Fprintf(w, "Error           %s\n", r.Error)
	}
	if r.Owner != "" {
		fmt.Fprintf(w, "Owner           %s\n", r.Owner)
	}
	if r.HTTPStatus != 0 {
		fmt.Fprintf(w, "HTTP            %d %s\n", r.HTTPStatus, http.StatusText(r.HTTPStatus))
	}
	if r.HTTPError != "" {
		fmt.Fprintf(w, "HTTP            error: %s\n", r.HTTPError)
	}
}
