package main

import (
	"github.com/Susurrium/PkuHoleStudio/internal/app"

	"github.com/spf13/cobra"
)

func newRebuildSearchIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild-search-index",
		Short: "Rebuild the local SQLite FTS5 search index",
		RunE: func(cmd *cobra.Command, args []string) error {
			application, err := app.Open(cmd.Context(), app.Options{})
			if err != nil {
				return err
			}
			defer application.Close()
			if err := application.Search.RebuildIndex(cmd.Context()); err != nil {
				return err
			}
			cmd.Println("search index rebuilt")
			return nil
		},
	}
}
