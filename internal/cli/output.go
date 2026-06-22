package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/pyahu/cli/internal/doctor"
	"github.com/pyahu/cli/pkg/schema"
)

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func (a *app) info(format string, args ...any) {
	if a.opts.quiet || a.opts.output != "human" {
		return
	}
	fmt.Fprintf(a.opts.out, format+"\n", args...)
}

func (a *app) step(area string, format string, args ...any) {
	if a.opts.quiet || a.opts.output != "human" {
		return
	}
	message := fmt.Sprintf(format, args...)
	tag := "[" + area + "]"
	if a.colorOn() {
		tag = a.styler().dim(tag)
	}
	fmt.Fprintf(a.opts.out, "%s %s\n", tag, message)
}

func (a *app) printWarnings(checks []doctor.Check) {
	if a.opts.output != "human" || a.opts.quiet {
		return
	}
	for _, check := range checks {
		if doctor.Warning(check) {
			a.renderCheck(check)
		}
	}
}

// renderCheck prints one doctor check, aligning the status word in a 4-wide
// column and coloring it by severity on a terminal.
func (a *app) renderCheck(check doctor.Check) {
	a.info("%-24s %s %s", check.Name, a.field(checkMark(check), 4, a.markColor(check)), check.Message)
}

func (a *app) markColor(check doctor.Check) func(string) string {
	s := a.styler()
	switch {
	case !check.OK:
		return s.red
	case doctor.Warning(check):
		return s.yellow
	default:
		return s.green
	}
}

func checkMark(check doctor.Check) string {
	if !check.OK {
		return "fail"
	}
	if doctor.Warning(check) {
		return "warn"
	}
	return "ok"
}

func (a *app) printSummary(stack *schema.Stack, kubeconfig string) error {
	if a.opts.output == "json" {
		return writeJSON(a.opts.out, map[string]any{
			"cluster":    stack.Cluster.Name,
			"namespace":  stack.Cluster.Namespace,
			"kubeconfig": kubeconfig,
			"services":   stack.EnabledServices(),
			"env":        stack.ConnectionEnv(),
		})
	}
	s := a.styler()
	a.info("")
	a.info("%s %s", s.green(iconOK), s.ok("Pyahu local stack is ready"))
	a.info("%s%s", a.field("cluster:", 12, s.dim), stack.Cluster.Name)
	a.info("%s%s", a.field("namespace:", 12, s.dim), stack.Cluster.Namespace)
	a.info("%s%s", a.field("kubeconfig:", 12, s.dim), kubeconfig)
	a.info("")
	env := stack.ConnectionEnv()
	keys := schema.SortedEnvKeys(env)
	sort.Strings(keys)
	for _, key := range keys {
		a.info("%s %s", a.field(key, 28, s.cyan), env[key])
	}
	a.info("")
	a.info("%s %s", s.dim("next:"), s.dim("eval \"$(pyahu env)\""))
	return nil
}

func clusterState(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
