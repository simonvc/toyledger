package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/simonvc/miniledger/internal/web"
	"github.com/spf13/cobra"
)

var (
	webPort int
	webHost string
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Launch TUI in browser via Ghostty WASM terminal",
	RunE: func(cmd *cobra.Command, args []string) error {
		ledgerDir := filepath.Join(filepath.Dir(flagDB), "ledgers")
		if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
			return fmt.Errorf("create ledgers dir: %w", err)
		}

		listenAddr := net.JoinHostPort(webHost, fmt.Sprintf("%d", webPort))
		fmt.Printf("miniledger web UI: http://%s\n", listenAddr)

		webSrv := web.NewServer(listenAddr, ledgerDir)
		return webSrv.ListenAndServe()
	},
}

func init() {
	webCmd.Flags().IntVar(&webPort, "port", 8833, "HTTP port for web terminal")
	webCmd.Flags().StringVar(&webHost, "host", "localhost", "HTTP host for web terminal")
	rootCmd.AddCommand(webCmd)
}
