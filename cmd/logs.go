package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsLines  int
	logsStream string
)

var logsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "View service logs",
	Long:  `View or stream logs from a managed service. Use --follow for live streaming.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if logsFollow {
			return streamLogs(name)
		}
		return fetchLogs(name)
	},
}

func fetchLogs(name string) error {
	path := fmt.Sprintf("/api/v1/services/%s/logs?lines=%d&stream=%s", name, logsLines, logsStream)
	resp, err := apiRequest("GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readAPIError(resp)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	return scanner.Err()
}

func streamLogs(name string) error {
	addr := apiAddr()
	u := url.URL{
		Scheme:   "ws",
		Host:     addr,
		Path:     fmt.Sprintf("/api/v1/services/%s/logs/stream", name),
		RawQuery: fmt.Sprintf("stream=%s", logsStream),
	}

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("connecting to log stream: %w\nIs the ELNSSM Guardian running?", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	defer conn.Close()

	// Handle Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		conn.Close()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return nil
			}
			return nil // connection closed
		}
		fmt.Print(string(message))
	}
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "stream logs in real-time")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "number of log lines to show")
	logsCmd.Flags().StringVar(&logsStream, "stream", "combined", "log stream: stdout, stderr, combined")
	rootCmd.AddCommand(logsCmd)
}
