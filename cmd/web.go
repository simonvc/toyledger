package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/server"
	"github.com/simonvc/miniledger/internal/store"
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
		apiAddr := flagServer

		if !cmd.Flags().Changed("server") {
			st, err := store.Open(flagDB)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer st.Close()

			srv := server.New(st, "127.0.0.1:8888")
			go func() {
				if err := srv.ListenAndServe(); err != nil {
					log.Printf("embedded server error: %v", err)
				}
			}()
			apiAddr = "http://127.0.0.1:8888"

			c := client.New(apiAddr)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for {
				if err := c.Ping(ctx); err == nil {
					break
				}
				if ctx.Err() != nil {
					return fmt.Errorf("timeout waiting for embedded server")
				}
				time.Sleep(50 * time.Millisecond)
			}
		}

		listenAddr := net.JoinHostPort(webHost, fmt.Sprintf("%d", webPort))
		fmt.Printf("miniledger web UI: http://%s\n", listenAddr)

		webSrv := web.NewServer(listenAddr, apiAddr, flagDB)
		return webSrv.ListenAndServe()
	},
}

func init() {
	webCmd.Flags().IntVar(&webPort, "port", 8833, "HTTP port for web terminal")
	webCmd.Flags().StringVar(&webHost, "host", "localhost", "HTTP host for web terminal")
	rootCmd.AddCommand(webCmd)
}
