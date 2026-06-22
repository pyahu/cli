package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pyahu/cli/internal/kube"
	"github.com/pyahu/cli/pkg/schema"
)

func (a *app) newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up local service data to the host",
	}
	cmd.AddCommand(a.newBackupPostgresCmd())
	return cmd
}

func (a *app) newBackupPostgresCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "postgres [database]",
		Short: "Back up a PostgreSQL database to a local dump file",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			loaded, client, err := a.backupRestoreClient(cmd.Context())
			if err != nil {
				return err
			}
			stack := loaded.Data
			if !stack.PostgresEnabled() {
				return usageError("services.postgres is not enabled")
			}
			database := defaultPostgresDatabase(stack)
			if len(args) == 1 {
				database = args[0]
			}
			if err := validateConfiguredPostgresDatabase(stack, database); err != nil {
				return usageError(err.Error())
			}
			if dir == "" {
				dir = filepath.Join(loaded.Dir, ".pyahu", "backups")
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return serviceError(fmt.Sprintf("create backup directory: %v", err))
			}
			path := filepath.Join(dir, backupFileName(stack, database, time.Now().UTC()))
			file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return serviceError(fmt.Sprintf("create backup file: %v", err))
			}
			defer file.Close()

			if err := a.phase("Gerando dump do banco "+database, func() (string, error) {
				if err := client.BackupPostgres(cmd.Context(), stack, database, file); err != nil {
					return "", serviceError(err.Error())
				}
				return "", nil
			}); err != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{
					"service":  "postgres",
					"database": database,
					"path":     path,
				})
			}
			a.info("backup written: %s", displayPath(path))
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "host directory for the dump file (default .pyahu/backups)")
	return cmd
}

func (a *app) newRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore local service data from a dump",
	}
	cmd.AddCommand(a.newRestorePostgresCmd())
	return cmd
}

func (a *app) newRestorePostgresCmd() *cobra.Command {
	var source string
	var s3EndpointURL string
	var clean bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "postgres [database]",
		Short: "Restore a PostgreSQL custom dump from disk or S3",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return usageError("restore postgres requires --source")
			}
			loaded, client, err := a.backupRestoreClient(cmd.Context())
			if err != nil {
				return err
			}
			stack := loaded.Data
			if !stack.PostgresEnabled() {
				return usageError("services.postgres is not enabled")
			}
			database := defaultPostgresDatabase(stack)
			if len(args) == 1 {
				database = args[0]
			}
			if err := validateConfiguredPostgresDatabase(stack, database); err != nil {
				return usageError(err.Error())
			}
			if clean {
				if err := a.confirmDestructiveRestore(database, source, yes); err != nil {
					return err
				}
			}
			reader, localPath, cleanup, err := openRestoreSource(cmd.Context(), source, s3EndpointURL)
			if err != nil {
				return dependencyError(err.Error())
			}
			defer cleanup()
			defer reader.Close()

			if err := a.phase("Restaurando o banco "+database, func() (string, error) {
				if err := client.RestorePostgres(cmd.Context(), stack, database, reader, kube.PostgresRestoreOptions{Clean: clean}); err != nil {
					return "", serviceError(err.Error())
				}
				return "", nil
			}); err != nil {
				return err
			}
			if a.opts.output == "json" {
				return writeJSON(a.opts.out, map[string]any{
					"service":  "postgres",
					"database": database,
					"source":   source,
					"path":     localPath,
					"clean":    clean,
				})
			}
			a.info("restore completed: %s -> %s", displayPath(localPath), database)
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "dump file path or s3:// URI")
	cmd.Flags().StringVar(&s3EndpointURL, "s3-endpoint-url", "", "custom S3-compatible endpoint URL for s3:// sources")
	cmd.Flags().BoolVar(&clean, "clean", true, "drop matching database objects before restoring")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm destructive restore without prompting")
	return cmd
}

func (a *app) confirmDestructiveRestore(database string, source string, yes bool) error {
	if yes {
		return nil
	}
	if a.opts.noInput {
		return usageError("restore postgres with --clean requires --yes when --no-input is set")
	}
	if a.opts.output != "human" {
		return usageError("restore postgres with --clean requires --yes for non-human output")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return usageError("restore postgres with --clean requires --yes in non-interactive mode")
	}
	fmt.Fprintf(a.opts.out, "This will restore %s with --clean from %s and may drop existing objects.\n", database, source)
	fmt.Fprintf(a.opts.out, "Type %s to continue: ", database)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return usageError(fmt.Sprintf("read confirmation: %v", err))
	}
	if strings.TrimSpace(answer) != database {
		return usageError("restore cancelled")
	}
	return nil
}

func (a *app) backupRestoreClient(ctx context.Context) (*loadedStack, localKube, error) {
	loaded, err := a.loadStack()
	if err != nil {
		return nil, nil, usageError(err.Error())
	}
	stack := loaded.Data
	rt := a.deps.newRuntime(a.opts)
	exists, err := rt.Exists(ctx, stack.Cluster.Name)
	if err != nil {
		return nil, nil, clusterError(err.Error())
	}
	if !exists {
		return nil, nil, clusterError(fmt.Sprintf("cluster %s is not running", stack.Cluster.Name))
	}
	kubeconfig, err := rt.Kubeconfig(ctx, stack.Cluster.Name)
	if err != nil {
		return nil, nil, clusterError(err.Error())
	}
	client, err := a.deps.newKube(kubeconfig)
	if err != nil {
		return nil, nil, clusterError(err.Error())
	}
	return &loadedStack{Dir: loaded.Dir, Data: stack}, client, nil
}

type loadedStack struct {
	Dir  string
	Data *schema.Stack
}

func defaultPostgresDatabase(stack *schema.Stack) string {
	if stack.Services.Postgres == nil || len(stack.Services.Postgres.Databases) == 0 {
		return "postgres"
	}
	return stack.Services.Postgres.Databases[0].Name
}

func validateConfiguredPostgresDatabase(stack *schema.Stack, database string) error {
	if strings.TrimSpace(database) == "" {
		return fmt.Errorf("database is required")
	}
	if stack.Services.Postgres == nil {
		return fmt.Errorf("services.postgres is not enabled")
	}
	for _, configured := range stack.Services.Postgres.Databases {
		if configured.Name == database {
			return nil
		}
	}
	return fmt.Errorf("database %q is not configured under services.postgres.databases", database)
}

func backupFileName(stack *schema.Stack, database string, now time.Time) string {
	return fmt.Sprintf("%s-%s-%s.dump", stack.Metadata.Name, database, now.Format("20060102-150405"))
}

func openRestoreSource(ctx context.Context, source string, s3EndpointURL string) (io.ReadCloser, string, func(), error) {
	if strings.HasPrefix(source, "s3://") {
		return downloadS3RestoreSource(ctx, source, s3EndpointURL)
	}
	file, err := os.Open(source)
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("open restore file: %w", err)
	}
	abs, err := filepath.Abs(source)
	if err != nil {
		abs = source
	}
	return file, abs, func() {}, nil
}

func downloadS3RestoreSource(ctx context.Context, source string, s3EndpointURL string) (io.ReadCloser, string, func(), error) {
	if _, err := exec.LookPath("aws"); err != nil {
		return nil, "", func() {}, fmt.Errorf("restore from s3:// requires aws CLI in PATH")
	}
	tmp, err := os.CreateTemp("", "pyahu-restore-*.dump")
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("create temporary restore file: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return nil, "", func() {}, err
	}
	args := []string{"s3", "cp", source, tmpPath}
	if s3EndpointURL != "" {
		args = append(args, "--endpoint-url", s3EndpointURL)
	}
	cmd := exec.CommandContext(ctx, "aws", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmpPath)
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, "", func() {}, fmt.Errorf("aws s3 cp failed: %w: %s", err, msg)
		}
		return nil, "", func() {}, fmt.Errorf("aws s3 cp failed: %w", err)
	}
	file, err := os.Open(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return nil, "", func() {}, fmt.Errorf("open downloaded restore file: %w", err)
	}
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	return file, tmpPath, cleanup, nil
}
