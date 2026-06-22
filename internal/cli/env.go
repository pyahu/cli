package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pyahu/cli/pkg/schema"
)

func (a *app) newEnvCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print local connection environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, err := a.loadStack()
			if err != nil {
				return usageError(err.Error())
			}
			env := loaded.Data.ConnectionEnv()
			switch format {
			case "json":
				return writeJSON(a.opts.out, env)
			case "dotenv":
				for _, key := range schema.SortedEnvKeys(env) {
					fmt.Fprintf(a.opts.out, "%s=%s\n", key, env[key])
				}
			case "shell":
				for _, key := range schema.SortedEnvKeys(env) {
					fmt.Fprintf(a.opts.out, "export %s=%s\n", key, shellQuote(env[key]))
				}
			default:
				return usageError("env --format must be shell, dotenv, or json")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "shell", "format: shell, dotenv, or json")
	return cmd
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
