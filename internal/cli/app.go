package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pyahu/cli/internal/config"
	"github.com/pyahu/cli/internal/doctor"
	"github.com/pyahu/cli/internal/kube"
	"github.com/pyahu/cli/internal/runtime/k3d"
	"github.com/pyahu/cli/pkg/schema"
)

type app struct {
	opts options
	deps dependencies

	colorOnce sync.Once
	colorVal  bool
	ttyVal    bool
}

type options struct {
	file    string
	output  string
	noColor bool
	quiet   bool
	verbose bool
	noInput bool
	version string
	commit  string
	date    string
	out     io.Writer
	err     io.Writer
}

type dependencies struct {
	loadStack     func(path string) (*config.LoadedStack, error)
	writePreset   func(path string, preset string, force bool) error
	newRuntime    func(opts options) localRuntime
	newKube       func(kubeconfig string) (localKube, error)
	runDoctor     func(ctx context.Context, stack *schema.Stack, clusterExists bool) []doctor.Check
	clusterExists func(ctx context.Context, stack *schema.Stack) bool
	readFile      func(path string) ([]byte, error)
}

type localRuntime interface {
	CheckInstalled() error
	Exists(ctx context.Context, name string) (bool, error)
	Create(ctx context.Context, stack *schema.Stack, stackDir string) (bool, error)
	Delete(ctx context.Context, name string) error
	Kubeconfig(ctx context.Context, name string) (string, error)
}

type localKube interface {
	WaitForAPI(ctx context.Context, timeout time.Duration) error
	ApplyStack(ctx context.Context, stack *schema.Stack, stackDir string) error
	WaitForStack(ctx context.Context, stack *schema.Stack) error
	CaptureZitadelPAT(ctx context.Context, stack *schema.Stack) error
	DeleteNamespace(ctx context.Context, namespace string) error
	Status(ctx context.Context, stack *schema.Stack) ([]kube.ServiceStatus, error)
	Logs(ctx context.Context, namespace string, service string, follow bool, tail int64) (io.ReadCloser, error)
	BackupPostgres(ctx context.Context, stack *schema.Stack, database string, out io.Writer) error
	RestorePostgres(ctx context.Context, stack *schema.Stack, database string, in io.Reader, opts kube.PostgresRestoreOptions) error
}

func Execute(version string, commit string, date string) int {
	a := newApp(version, commit, date, os.Stdout, os.Stderr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := a.newRootCmd()
	cmd.SetContext(ctx)
	if err := cmd.Execute(); err != nil {
		es := styler{on: stderrColor(a.opts)}
		if errors.Is(err, context.Canceled) {
			fmt.Fprintf(a.opts.err, "%s %s\n", es.yellow(iconWarn), es.yellow("interrupted"))
			return 130
		}
		fmt.Fprintf(a.opts.err, "%s %s\n", es.bad("error:"), err)
		if hint := errorHint(err); hint != "" {
			fmt.Fprintf(a.opts.err, "%s %s\n", es.dim("hint:"), es.dim(hint))
		}
		return exitCode(err)
	}
	return 0
}

func newApp(version string, commit string, date string, out io.Writer, err io.Writer) *app {
	opts := options{
		output:  "human",
		version: version,
		commit:  commit,
		date:    date,
		out:     out,
		err:     err,
	}
	return &app{
		opts: opts,
		deps: dependencies{
			loadStack:   config.LoadFromFlag,
			writePreset: config.WritePreset,
			newRuntime: func(opts options) localRuntime {
				return k3d.Runtime{Out: opts.out, Err: opts.err, Verbose: opts.verbose}
			},
			newKube: func(kubeconfig string) (localKube, error) {
				return kube.New(kubeconfig)
			},
			runDoctor:     doctor.Run,
			clusterExists: doctor.ClusterExists,
			readFile:      os.ReadFile,
		},
	}
}
