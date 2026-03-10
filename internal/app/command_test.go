package app

import (
	"bytes"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ofkm/bndry/internal/config"
	"github.com/spf13/cobra"
)

func TestNewRootCommandContainsExpectedCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path []string
	}{
		{name: "login", path: []string{"login"}},
		{name: "setup", path: []string{"setup"}},
		{name: "setup ssh", path: []string{"setup", "ssh"}},
		{name: "ssh", path: []string{"ssh"}},
		{name: "ssh inspect", path: []string{"ssh", "inspect"}},
		{name: "ssh list", path: []string{"ssh", "list"}},
		{name: "targets", path: []string{"targets"}},
		{name: "scopes", path: []string{"scopes"}},
		{name: "config", path: []string{"config"}},
		{name: "config init", path: []string{"config", "init"}},
		{name: "config path", path: []string{"config", "path"}},
		{name: "config show", path: []string{"config", "show"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := newTestApp(t)
			root := app.NewRootCommand()

			if got := findCommand(root, tt.path...); got == nil {
				t.Fatalf("findCommand(%q) returned nil", strings.Join(tt.path, " "))
			}
		})
	}
}

func TestRunHelpRendersStyledCommandHelp(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := newBufferedTestApp(t, stdout, stderr)
	root := app.NewRootCommand()

	if err := app.Run(context.Background(), []string{"help"}); err != nil {
		t.Fatalf("Run(help) error = %v", err)
	}

	output := stripANSI(stdout.String())
	checks := []string{
		root.Short,
		"bndry setup",
		"bndry ssh [user@]target",
		"bndry config",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("help output missing %q\noutput:\n%s", check, output)
		}
	}
}

func TestRunHelpSSHShowsInspectAndListCommands(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := newBufferedTestApp(t, stdout, stderr)

	if err := app.Run(context.Background(), []string{"help", "ssh"}); err != nil {
		t.Fatalf("Run(help ssh) error = %v", err)
	}

	output := stripANSI(stdout.String())
	checks := []string{
		"bndry ssh list",
		"bndry ssh inspect [target-name]",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("ssh help output missing %q\noutput:\n%s", check, output)
		}
	}
}

func TestRunConfigPathPrintsResolvedPath(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	app := newBufferedTestApp(t, stdout, stderr)

	if err := app.Run(context.Background(), []string{"config", "path"}); err != nil {
		t.Fatalf("Run(config path) error = %v", err)
	}

	output := strings.TrimSpace(stdout.String())
	if output != app.store.Path() {
		t.Fatalf("config path output = %q, want %q", output, app.store.Path())
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	return &App{store: store}
}

func newBufferedTestApp(t *testing.T, stdout *bytes.Buffer, stderr *bytes.Buffer) *App {
	t.Helper()

	app := newTestApp(t)
	app.stdout = stdout
	app.stderr = stderr
	return app
}

func findCommand(root *cobra.Command, path ...string) *cobra.Command {
	current := root
	for _, segment := range path {
		var next *cobra.Command
		for _, child := range current.Commands() {
			if child.Name() == segment {
				next = child
				break
			}
		}

		if next == nil {
			return nil
		}

		current = next
	}

	return current
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiPattern.ReplaceAllString(value, "")
}
