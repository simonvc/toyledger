package cmd

import (
	"github.com/simonvc/miniledger/internal/server"
	"github.com/simonvc/miniledger/internal/store"
	"github.com/spf13/cobra"
)

var serveAddr string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.Open(flagDB)
		if err != nil {
			return err
		}
		defer st.Close()

		srv := server.New(st, serveAddr)
		return srv.ListenAndServe()
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":8888", "Listen address")
	rootCmd.AddCommand(serveCmd)
}
