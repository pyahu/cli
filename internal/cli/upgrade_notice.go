package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/pyahu/cli/internal/update"
)

// upgradeNoticeWait is how long the notice waits for the background check after
// the command is done. Missing the window costs nothing: the check caches what
// it found, so the next command has the answer already.
const upgradeNoticeWait = 700 * time.Millisecond

// printUpgradeNotice writes the "you are behind" message to stderr.
//
// stderr, not stdout, is load-bearing: `eval "$(pyahu env)"` and
// `--output json` consume stdout, and a banner there would be evaluated as
// shell or break the JSON.
func (a *app) printUpgradeNotice(check func(time.Duration) (update.Result, bool)) {
	if check == nil || a.noticeHandled || a.opts.quiet || a.opts.output != "human" {
		return
	}
	result, ok := check(upgradeNoticeWait)
	if !ok {
		return
	}

	s := styler{on: stderrColor(a.opts)}
	_, _ = fmt.Fprintln(a.opts.err)
	_, _ = fmt.Fprintf(a.opts.err, "%s %s\n",
		s.yellow(iconWarn),
		s.yellow(fmt.Sprintf("pyahu %s is out of date — %s is available", result.Current, result.Latest)))
	for _, command := range update.Instructions(executablePath(), result.Latest) {
		_, _ = fmt.Fprintf(a.opts.err, "  %s\n", s.bold(command))
	}
	_, _ = fmt.Fprintf(a.opts.err, "  %s\n", s.dim(update.ReleaseNotesURL(result.Latest)))
	_, _ = fmt.Fprintf(a.opts.err, "  %s\n", s.dim("silence this with "+update.OptOutEnv+"=1"))
}

func executablePath() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	return path
}
