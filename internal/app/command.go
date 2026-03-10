package app

import (
	"fmt"

	"github.com/kmendell/bdry-cli/internal/ui"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the Cobra command tree for bndry.
func (a *App) NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "bndry",
		Short:         "Friendly Boundary CLI for SSH-first workflows",
		Long:          "bndry wraps the HashiCorp Boundary CLI with simpler commands, interactive prompts, and friendlier output.",
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetOut(a.stdout)
	rootCmd.SetErr(a.stderr)
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), ui.RenderCommandHelp(cmd))
	})

	rootCmd.AddCommand(
		a.newLoginCommand(),
		a.newSetupCommand(),
		a.newAddCommand(),
		a.newSSHCommand(),
		a.newTargetsCommand(),
		a.newScopesCommand(),
		a.newConfigCommand(),
	)

	return rootCmd
}

func (a *App) newLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "login",
		Short:   "Authenticate with Boundary using OIDC",
		Args:    cobra.NoArgs,
		Example: "bndry login",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runLogin(cmd.Context())
		},
	}
}

func (a *App) newSetupCommand() *cobra.Command {
	setupCmd := &cobra.Command{
		Use:     "setup",
		Short:   "Guided SSH setup for a Boundary target",
		Long:    "Create a static host catalog, host, host set, target, and optional role grant for SSH access.",
		Args:    cobra.NoArgs,
		Example: "bndry setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSetupSSH(cmd.Context())
		},
	}

	setupCmd.AddCommand(&cobra.Command{
		Use:     "ssh",
		Short:   "Guided SSH setup (compatibility subcommand)",
		Args:    cobra.NoArgs,
		Example: "bndry setup ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSetupSSH(cmd.Context())
		},
	})

	return setupCmd
}

func (a *App) newAddCommand() *cobra.Command {
	var (
		name      string
		port      int
		group     string
		catalog   string
		noConnect bool
	)

	addCmd := &cobra.Command{
		Use:   "add [ip-address]",
		Short: "Quickly add an SSH target with smart defaults",
		Long:  "Create a host, host set, and target for an IP address with minimal flags. Reuses config defaults and auto-generates names.",
		Args:  cobra.ExactArgs(1),
		Example: `  bndry add 10.0.0.5
  bndry add 192.168.1.100 -n web-server -p 2222
  bndry add 172.18.24.5 -g web-servers --no-connect`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runAdd(cmd.Context(), args[0], name, port, group, catalog, noConnect)
		},
	}

	addCmd.Flags().StringVarP(&name, "name", "n", "", "target name (auto-generated from IP if omitted)")
	addCmd.Flags().IntVarP(&port, "port", "p", 0, "target port (uses config default_target_port if omitted)")
	addCmd.Flags().StringVarP(&group, "group", "g", "", "host set name (uses config default_host_set_name if omitted)")
	addCmd.Flags().StringVar(&catalog, "catalog", "", "host catalog name (uses config default_catalog_name if omitted)")
	addCmd.Flags().BoolVar(&noConnect, "no-connect", false, "skip auto-connect after creation")

	return addCmd
}

func (a *App) newSSHCommand() *cobra.Command {
	sshCmd := &cobra.Command{
		Use:     "ssh [target-name]",
		Short:   "Connect to a target by friendly name",
		Args:    cobra.MaximumNArgs(1),
		Example: "bndry ssh boron\nbndry ssh list",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := ""
			if len(args) == 1 {
				targetName = args[0]
			}
			return a.runSSH(cmd.Context(), targetName)
		},
	}

	sshCmd.AddCommand(&cobra.Command{
		Use:     "list",
		Short:   "List targets available for SSH across project scopes",
		Args:    cobra.NoArgs,
		Example: "bndry ssh list",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runSSHList(cmd.Context())
		},
	})

	sshCmd.AddCommand(&cobra.Command{
		Use:     "inspect [target-name]",
		Short:   "Inspect a target's host sources and backing hosts",
		Args:    cobra.MaximumNArgs(1),
		Example: "bndry ssh inspect cobalt-ssh",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetName := ""
			if len(args) == 1 {
				targetName = args[0]
			}
			return a.runSSHInspect(cmd.Context(), targetName)
		},
	})

	return sshCmd
}

func (a *App) newTargetsCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "targets",
		Short:   "List targets in a project scope",
		Args:    cobra.NoArgs,
		Example: "bndry targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runTargets(cmd.Context())
		},
	}
}

func (a *App) newScopesCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "scopes",
		Short:   "List scopes recursively",
		Args:    cobra.NoArgs,
		Example: "bndry scopes",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runScopes(cmd.Context())
		},
	}
}

func (a *App) newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage bndry configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	configCmd.AddCommand(
		&cobra.Command{
			Use:     "show",
			Short:   "Print the effective configuration",
			Args:    cobra.NoArgs,
			Example: "bndry config show",
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.runConfigShow()
			},
		},
		&cobra.Command{
			Use:     "init",
			Short:   "Create a starter config file",
			Args:    cobra.NoArgs,
			Example: "bndry config init",
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.runConfigInit()
			},
		},
		&cobra.Command{
			Use:     "path",
			Short:   "Print the resolved config path",
			Args:    cobra.NoArgs,
			Example: "bndry config path",
			RunE: func(cmd *cobra.Command, args []string) error {
				return a.runConfigPath()
			},
		},
	)

	return configCmd
}
