package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/simonvc/miniledger/internal/client"
	"github.com/simonvc/miniledger/internal/server"
	"github.com/simonvc/miniledger/internal/store"
	"github.com/simonvc/miniledger/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive terminal UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverAddr := flagServer

		if !cmd.Flags().Changed("server") {
			// Start embedded server in background
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
			serverAddr = "http://127.0.0.1:8888"

			// Wait for server to be ready
			c := client.New(serverAddr)
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

		c := client.New(serverAddr)
		app := tui.NewApp(c)
		p := tea.NewProgram(app, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
