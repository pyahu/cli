package cli

import (
	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/config"
)

func (a *app) newInitCmd() *cobra.Command {
	var preset string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a pyahu.yaml stack file",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := a.opts.file
			if path == "" {
				path = config.DefaultFileName
			}
			if err := a.deps.writePreset(path, preset, force); err != nil {
				return err
			}
			a.info("created %s", path)
			a.info("next: pyahu up")
			return nil
		},
	}
	cmd.Flags().StringVar(&preset, "preset", "minimal", "starter preset: minimal or platform")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing stack file")
	return cmd
}
