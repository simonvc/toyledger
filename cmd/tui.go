package cmd

import (
	"fmt"
	"log"
	"net"

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
			st, err := store.Open(flagDB)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer st.Close()

			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}

			srv := server.New(st, ln.Addr().String())
			go func() {
				if err := srv.Serve(ln); err != nil {
					log.Printf("embedded server error: %v", err)
				}
			}()
			serverAddr = "http://" + ln.Addr().String()
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
