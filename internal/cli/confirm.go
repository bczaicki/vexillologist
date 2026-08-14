package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/bczaicki/vexillologist/internal/ld"
)

// guardedEnvKeys always require strong confirmation regardless of LD's
// critical flag or user config.
var guardedEnvKeys = map[string]bool{"prod": true, "production": true}

// isGuardedEnv reports whether mutations in env need strong confirmation:
// LD marks it critical, its key is prod/production, or the user listed it
// under guarded_environments in the config file.
func isGuardedEnv(env ld.Environment, extraGuarded []string) bool {
	if env.Critical || guardedEnvKeys[env.Key] {
		return true
	}
	for _, key := range extraGuarded {
		if key == env.Key {
			return true
		}
	}
	return false
}

func stdinIsTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// confirm gates a mutation. --yes always passes. Without a TTY, --yes is
// mandatory (exit 3). On a TTY, guarded targets must type expected verbatim;
// unguarded targets get a y/N prompt.
func (g *globals) confirm(action, expected string, guarded bool) error {
	if g.yes {
		return nil
	}
	if !stdinIsTTY() {
		return confirmationError(fmt.Sprintf("%s requires confirmation: re-run with --yes (no TTY available for a prompt)", action))
	}

	reader := bufio.NewReader(os.Stdin)
	if guarded {
		fmt.Fprintf(os.Stderr, "%s\nThis target is guarded. Type %q to confirm: ", action, expected)
		line, err := reader.ReadString('\n')
		if err != nil {
			return confirmationError("confirmation aborted")
		}
		if strings.TrimSpace(line) != expected {
			return confirmationError(fmt.Sprintf("confirmation did not match %q", expected))
		}
		return nil
	}

	fmt.Fprintf(os.Stderr, "%s — continue? [y/N] ", action)
	line, err := reader.ReadString('\n')
	if err != nil {
		return confirmationError("confirmation aborted")
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return confirmationError("aborted by user")
	}
}
