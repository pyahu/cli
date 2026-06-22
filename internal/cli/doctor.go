package cli

import (
	"github.com/spf13/cobra"

	"github.com/pyahu/cli/internal/doctor"
	"github.com/pyahu/cli/pkg/schema"
)

func (a *app) newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check local dependencies and ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			var stack *schema.Stack
			if err == nil {
				stack = loaded.Data
			} else {
				stack = defaultDoctorStack()
			}
			clusterExists := a.deps.clusterExists(cmd.Context(), stack)
			checks := a.deps.runDoctor(cmd.Context(), stack, clusterExists)
			if a.opts.output == "json" {
				healthy := doctor.Healthy(checks)
				if err := writeJSON(a.opts.out, map[string]any{"checks": checks, "ok": healthy}); err != nil {
					return err
				}
				if !healthy {
					return dependencyError("doctor found problems")
				}
				return nil
			}
			for _, check := range checks {
				a.renderCheck(check)
			}
			if !doctor.Healthy(checks) {
				return dependencyError("doctor found problems")
			}
			return nil
		},
	}
}
