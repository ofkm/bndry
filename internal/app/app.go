package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/ofkm/bndry/internal/boundary"
	"github.com/ofkm/bndry/internal/config"
	"github.com/ofkm/bndry/internal/ui"
)

// App is the main entrypoint for the bndry CLI.
type App struct {
	store  *config.Store
	stdout io.Writer
	stderr io.Writer
}

// New creates a configured App instance.
func New() (*App, error) {
	store, err := config.NewStore("")
	if err != nil {
		return nil, err
	}

	return &App{
		store:  store,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}, nil
}

// Run executes the requested command.
func (a *App) Run(ctx context.Context, args []string) error {
	cmd := a.NewRootCommand()
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(ctx)
	if errors.Is(err, ui.ErrCancelled) {
		a.printWarning("Cancelled.")
		return nil
	}

	return err
}

func (a *App) runLogin(ctx context.Context) error {
	cfg, err := a.store.Load()
	if err != nil {
		return err
	}

	addr, err := ui.RunTextInput(
		"Boundary controller URL",
		"https://boundary.example.com",
		cfg.BoundaryAddr,
		validateURL,
	)
	if err != nil {
		return err
	}

	authMethodID, err := ui.RunTextInput(
		"OIDC auth method ID (leave blank to use the default provider)",
		"leave blank to use the primary/default OIDC provider",
		cfg.OIDCAuthMethodID,
		nil,
	)
	if err != nil {
		return err
	}

	client := boundary.NewClient(addr, "")
	a.printInfo("Starting Boundary OIDC login...")
	start, err := client.StartOIDCAuth(ctx, authMethodID)
	if err != nil {
		return err
	}

	a.printInfo("Opening returned authentication URL in your browser...")
	fmt.Fprintln(a.stdout, start.AuthURL)
	if err := openBrowser(start.AuthURL); err != nil {
		a.printWarning(fmt.Sprintf("Unable to open authentication URL in browser: %v", err))
		a.printInfo("Please copy and paste this link into a browser manually:")
		fmt.Fprintln(a.stdout, start.AuthURL)
	}

	token, err := client.WaitForOIDCToken(ctx, start.AuthMethodID, start.TokenID)
	if err != nil {
		return err
	}

	cfg.BoundaryAddr = addr
	cfg.OIDCAuthMethodID = start.AuthMethodID
	cfg.AuthToken = token
	if err := a.store.Save(cfg); err != nil {
		return err
	}

	a.printSuccess(fmt.Sprintf("Saved defaults and auth token to %s", a.store.Path()))
	return nil
}

func (a *App) runSetupSSH(ctx context.Context) error {
	cfg, changed, err := a.ensureBaseConfig()
	if err != nil {
		return err
	}

	client := boundary.NewClient(cfg.BoundaryAddr, cfg.AuthToken)
	scope, err := a.chooseProjectScope(ctx, client, cfg.DefaultProjectScopeID)
	if err != nil {
		return err
	}

	catalogDefault := firstNonEmpty(cfg.DefaultCatalogName, config.DefaultConfig().DefaultCatalogName)
	catalogName, err := ui.RunTextInput(
		"Static host catalog name",
		catalogDefault,
		catalogDefault,
		validateRequired("catalog name"),
	)
	if err != nil {
		return err
	}

	hostName, err := ui.RunTextInput(
		"Host name",
		"boron",
		"",
		validateRequired("host name"),
	)
	if err != nil {
		return err
	}

	hostAddress, err := ui.RunTextInput(
		"Host address or DNS name",
		"172.18.24.5",
		"",
		validateRequired("host address"),
	)
	if err != nil {
		return err
	}

	hostSetDefault := firstNonEmpty(cfg.DefaultHostSetName, config.DefaultConfig().DefaultHostSetName)
	hostSetName, err := ui.RunTextInput(
		"Host set name",
		hostSetDefault,
		hostSetDefault,
		validateRequired("host set name"),
	)
	if err != nil {
		return err
	}

	targetName, err := ui.RunTextInput(
		"Target name",
		hostName,
		hostName,
		validateRequired("target name"),
	)
	if err != nil {
		return err
	}

	defaultPort := cfg.DefaultTargetPort
	if defaultPort <= 0 {
		defaultPort = config.DefaultConfig().DefaultTargetPort
	}
	defaultPortText := strconv.Itoa(defaultPort)
	portText, err := ui.RunTextInput(
		"Target port",
		defaultPortText,
		defaultPortText,
		validatePort,
	)
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(portText)
	if err != nil {
		return fmt.Errorf("parse port: %w", err)
	}

	createRole, err := ui.RunConfirm("Create a role grant for this target now?", cfg.DefaultCreateRole)
	if err != nil {
		return err
	}

	roleName := ""
	principalID := ""
	if createRole {
		roleDefault := firstNonEmpty(cfg.DefaultRoleName, config.DefaultConfig().DefaultRoleName)
		roleName, err = ui.RunTextInput(
			"Role name",
			roleDefault,
			roleDefault,
			validateRequired("role name"),
		)
		if err != nil {
			return err
		}

		principalID, err = ui.RunTextInput(
			"Principal ID to add (optional)",
			"mgoidc_xxxxx",
			cfg.DefaultPrincipalID,
			nil,
		)
		if err != nil {
			return err
		}
	}

	useAsDefault, err := ui.RunConfirm("Use this project scope as your default scope?", cfg.DefaultProjectScopeID == "" || cfg.DefaultProjectScopeID == scope.ID)
	if err != nil {
		return err
	}

	a.printInfo(fmt.Sprintf("Creating Boundary resources in %s (%s)...", displayScope(scope), scope.ID))
	bundle, err := client.CreateSSHBundle(ctx, boundary.CreateSSHBundleRequest{
		ScopeID:     scope.ID,
		CatalogName: catalogName,
		HostName:    hostName,
		HostAddress: hostAddress,
		HostSetName: hostSetName,
		TargetName:  targetName,
		Port:        port,
		CreateRole:  createRole,
		RoleName:    roleName,
		PrincipalID: principalID,
	})
	if err != nil {
		return err
	}

	if useAsDefault {
		cfg.DefaultProjectScopeID = scope.ID
		changed = true
	}
	cfg.LastHostCatalogID = bundle.Catalog.ID
	changed = true

	if changed {
		if err := a.store.Save(cfg); err != nil {
			return err
		}
	}

	a.printSuccess("Boundary resources created")
	fmt.Fprintf(a.stdout, "  Catalog:  %s (%s)\n", bundle.Catalog.Name, bundle.Catalog.ID)
	fmt.Fprintf(a.stdout, "  Host:     %s (%s -> %s)\n", bundle.Host.Name, bundle.Host.ID, bundle.Host.Address)
	fmt.Fprintf(a.stdout, "  Host set: %s (%s)\n", bundle.HostSet.Name, bundle.HostSet.ID)
	fmt.Fprintf(a.stdout, "  Target:   %s (%s)\n", bundle.Target.Name, bundle.Target.ID)
	if bundle.Role != nil {
		fmt.Fprintf(a.stdout, "  Role:     %s (%s)\n", bundle.Role.Name, bundle.Role.ID)
		if strings.TrimSpace(principalID) == "" {
			a.printWarning("Role created without a principal; add one later with Boundary if needed.")
		}
	}

	connectNow, err := ui.RunConfirm("Connect to the new target now?", cfg.DefaultConnectAfterCreate)
	if err != nil {
		return err
	}
	if connectNow {
		return client.ConnectSSH(ctx, bundle.Target.ID, "")
	}

	return nil
}

func (a *App) runAdd(ctx context.Context, ipAddress string, name string, port int, group string, catalog string, noConnect bool) error {
	cfg, changed, err := a.ensureBaseConfig()
	if err != nil {
		return err
	}

	client := boundary.NewClient(cfg.BoundaryAddr, cfg.AuthToken)

	// Select or use default project scope
	scopeID := cfg.DefaultProjectScopeID
	if scopeID == "" {
		scope, err := a.chooseProjectScope(ctx, client, scopeID)
		if err != nil {
			return err
		}
		scopeID = scope.ID
	}

	// Use smart defaults
	if catalog == "" {
		catalog = firstNonEmpty(cfg.DefaultCatalogName, config.DefaultConfig().DefaultCatalogName)
	}
	if group == "" {
		group = firstNonEmpty(cfg.DefaultHostSetName, config.DefaultConfig().DefaultHostSetName)
	}
	if name == "" {
		// Auto-generate name from IP: 10.0.0.5 -> host-10-0-0-5
		name = "host-" + strings.ReplaceAll(ipAddress, ".", "-")
	}
	if port <= 0 {
		port = cfg.DefaultTargetPort
		if port <= 0 {
			port = config.DefaultConfig().DefaultTargetPort
		}
	}

	// Use config defaults for role creation
	createRole := cfg.DefaultCreateRole
	roleName := firstNonEmpty(cfg.DefaultRoleName, config.DefaultConfig().DefaultRoleName)
	principalID := cfg.DefaultPrincipalID

	a.printInfo(fmt.Sprintf("Adding target %s (%s:%d)...", name, ipAddress, port))

	bundle, err := client.CreateSSHBundle(ctx, boundary.CreateSSHBundleRequest{
		ScopeID:     scopeID,
		CatalogName: catalog,
		HostName:    name,
		HostAddress: ipAddress,
		HostSetName: group,
		TargetName:  name,
		Port:        port,
		CreateRole:  createRole,
		RoleName:    roleName,
		PrincipalID: principalID,
	})
	if err != nil {
		return err
	}

	cfg.LastHostCatalogID = bundle.Catalog.ID
	changed = true
	if changed {
		if err := a.store.Save(cfg); err != nil {
			return err
		}
	}

	a.printSuccess("Target created")
	fmt.Fprintf(a.stdout, "  Catalog:  %s (%s)\n", bundle.Catalog.Name, bundle.Catalog.ID)
	fmt.Fprintf(a.stdout, "  Host:     %s (%s -> %s)\n", bundle.Host.Name, bundle.Host.ID, bundle.Host.Address)
	fmt.Fprintf(a.stdout, "  Host set: %s (%s)\n", bundle.HostSet.Name, bundle.HostSet.ID)
	fmt.Fprintf(a.stdout, "  Target:   %s (%s)\n", bundle.Target.Name, bundle.Target.ID)
	if bundle.Role != nil {
		fmt.Fprintf(a.stdout, "  Role:     %s (%s)\n", bundle.Role.Name, bundle.Role.ID)
	}

	// Connect automatically unless --no-connect is set
	shouldConnect := !noConnect && cfg.DefaultConnectAfterCreate
	if shouldConnect {
		fmt.Fprintln(a.stdout)
		a.printInfo("Connecting...")
		return client.ConnectSSH(ctx, bundle.Target.ID, "")
	}

	return nil
}

func (a *App) runSSH(ctx context.Context, targetName string) error {
	cfg, changed, err := a.ensureBaseConfig()
	if err != nil {
		return err
	}
	if changed {
		if err := a.store.Save(cfg); err != nil {
			return err
		}
	}

	client := boundary.NewClient(cfg.BoundaryAddr, cfg.AuthToken)
	if strings.TrimSpace(targetName) != "" {
		// Parse user@target format (like Teleport's tsh)
		var username string
		actualTargetName := targetName
		if idx := strings.Index(targetName, "@"); idx != -1 {
			username = targetName[:idx]
			actualTargetName = targetName[idx+1:]
		}

		target, scope, err := a.resolveTargetByName(ctx, client, actualTargetName, cfg.DefaultProjectScopeID)
		if err != nil {
			return err
		}

		a.printInfo(fmt.Sprintf("Connecting to %s (%s) in %s...", target.Name, target.ID, displayScope(scope)))
		if err := client.ConnectSSH(ctx, target.ID, username); err != nil {
			return a.wrapSSHConnectError(err, target.Name)
		}
		return nil
	}

	scope, err := a.chooseProjectScope(ctx, client, cfg.DefaultProjectScopeID)
	if err != nil {
		return err
	}

	targets, err := client.ListTargets(ctx, scope.ID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return fmt.Errorf("no targets found in scope %s", scope.ID)
	}

	sort.Slice(targets, func(i, j int) bool {
		return strings.ToLower(targets[i].Name) < strings.ToLower(targets[j].Name)
	})

	options := make([]ui.Option, 0, len(targets))
	for _, candidate := range targets {
		options = append(options, ui.Option{
			Label:       candidate.Name,
			Description: fmt.Sprintf("%s • port %d", candidate.ID, candidate.DefaultPort),
			Value:       candidate.ID,
		})
	}

	choice, err := ui.RunSelect("Choose the target to connect to", options, 0)
	if err != nil {
		return err
	}

	var target boundary.Target
	for _, candidate := range targets {
		if candidate.ID == choice.Value {
			target = candidate
			break
		}
	}

	a.printInfo(fmt.Sprintf("Connecting to %s (%s)...", target.Name, target.ID))
	if err := client.ConnectSSH(ctx, target.ID, ""); err != nil {
		return a.wrapSSHConnectError(err, target.Name)
	}
	return nil
}

type targetMatch struct {
	scope  boundary.Scope
	target boundary.Target
}

func (a *App) runSSHList(ctx context.Context) error {
	cfg, changed, err := a.ensureBaseConfig()
	if err != nil {
		return err
	}
	if changed {
		if err := a.store.Save(cfg); err != nil {
			return err
		}
	}

	client := boundary.NewClient(cfg.BoundaryAddr, cfg.AuthToken)
	matches, err := a.listProjectTargets(ctx, client)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return errors.New("no connectable targets found in any project scope")
	}

	tw := tabwriter.NewWriter(a.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPORT\tTARGET ID\tSCOPE\tSCOPE ID")
	for _, match := range matches {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", match.target.Name, match.target.DefaultPort, match.target.ID, displayScope(match.scope), match.scope.ID)
	}
	return tw.Flush()
}

func (a *App) runSSHInspect(ctx context.Context, targetName string) error {
	cfg, changed, err := a.ensureBaseConfig()
	if err != nil {
		return err
	}
	if changed {
		if err := a.store.Save(cfg); err != nil {
			return err
		}
	}

	client := boundary.NewClient(cfg.BoundaryAddr, cfg.AuthToken)

	var (
		target boundary.Target
		scope  boundary.Scope
	)
	if strings.TrimSpace(targetName) != "" {
		target, scope, err = a.resolveTargetByName(ctx, client, targetName, cfg.DefaultProjectScopeID)
		if err != nil {
			return err
		}
	} else {
		match, err := a.chooseTarget(ctx, client, cfg.DefaultProjectScopeID)
		if err != nil {
			return err
		}
		target = match.target
		scope = match.scope
	}

	inspection, err := client.InspectTarget(ctx, target.ID)
	if err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "Target:   %s (%s)\n", inspection.Target.Name, inspection.Target.ID)
	fmt.Fprintf(a.stdout, "Scope:    %s (%s)\n", displayScope(scope), scope.ID)
	fmt.Fprintf(a.stdout, "Type:     %s\n", inspection.Target.Type)
	fmt.Fprintf(a.stdout, "Port:     %d\n", inspection.Target.DefaultPort)
	if strings.TrimSpace(inspection.Target.Address) != "" {
		fmt.Fprintf(a.stdout, "Address:  %s\n", inspection.Target.Address)
	}
	if len(inspection.Target.HostSourceIDs) > 0 {
		fmt.Fprintf(a.stdout, "Sources:  %s\n", strings.Join(inspection.Target.HostSourceIDs, ", "))
	} else {
		fmt.Fprintln(a.stdout, "Sources:  none")
	}

	if strings.TrimSpace(inspection.Target.Address) == "" && len(inspection.HostSources) == 0 {
		a.printWarning("This target has no direct address and no host sources, so Boundary cannot connect to it yet.")
		return nil
	}

	for idx, source := range inspection.HostSources {
		fmt.Fprintln(a.stdout)
		fmt.Fprintf(a.stdout, "Host source %d\n", idx+1)
		fmt.Fprintf(a.stdout, "  Host set:    %s (%s)\n", displayValue(source.HostSet.Name, source.HostSet.ID), source.HostSet.ID)
		fmt.Fprintf(a.stdout, "  Catalog:     %s (%s)\n", displayValue(source.HostCatalog.Name, source.HostCatalog.ID), source.HostCatalog.ID)
		if len(source.Hosts) == 0 {
			fmt.Fprintln(a.stdout, "  Hosts:       none")
			continue
		}

		for _, host := range source.Hosts {
			fmt.Fprintf(a.stdout, "  Host:        %s (%s)\n", displayValue(host.Name, host.ID), host.ID)
			if strings.TrimSpace(host.Address) != "" {
				fmt.Fprintf(a.stdout, "    Address:   %s\n", host.Address)
			}
			if len(host.DNSNames) > 0 {
				fmt.Fprintf(a.stdout, "    DNS:       %s\n", strings.Join(host.DNSNames, ", "))
			}
			if len(host.IPAddresses) > 0 {
				fmt.Fprintf(a.stdout, "    IPs:       %s\n", strings.Join(host.IPAddresses, ", "))
			}
		}
	}

	return nil
}

func (a *App) resolveTargetByName(ctx context.Context, client *boundary.Client, targetName string, defaultScopeID string) (boundary.Target, boundary.Scope, error) {
	matches, err := a.listProjectTargets(ctx, client)
	if err != nil {
		return boundary.Target{}, boundary.Scope{}, err
	}

	matchingTargets := make([]targetMatch, 0)
	defaultIndex := 0
	for _, match := range matches {
		for _, candidate := range boundary.FindTargetsByName([]boundary.Target{match.target}, targetName) {
			if match.scope.ID == defaultScopeID {
				defaultIndex = len(matchingTargets)
			}
			matchingTargets = append(matchingTargets, targetMatch{scope: match.scope, target: candidate})
		}
	}

	switch len(matchingTargets) {
	case 0:
		return boundary.Target{}, boundary.Scope{}, fmt.Errorf("target %q not found in any project scope; run `bndry ssh list` to see available targets", targetName)
	case 1:
		return matchingTargets[0].target, matchingTargets[0].scope, nil
	}

	options := make([]ui.Option, 0, len(matchingTargets))
	for _, match := range matchingTargets {
		description := fmt.Sprintf("%s • %s (%s) • port %d", match.target.ID, displayScope(match.scope), match.scope.ID, match.target.DefaultPort)
		options = append(options, ui.Option{
			Label:       match.target.Name,
			Description: description,
			Value:       match.target.ID,
		})
	}

	choice, err := ui.RunSelect(fmt.Sprintf("Found %d targets named %q; choose one", len(matchingTargets), targetName), options, defaultIndex)
	if err != nil {
		return boundary.Target{}, boundary.Scope{}, err
	}

	for _, match := range matchingTargets {
		if match.target.ID == choice.Value {
			return match.target, match.scope, nil
		}
	}

	return boundary.Target{}, boundary.Scope{}, fmt.Errorf("target %s was selected but could not be resolved", choice.Value)
}

func (a *App) chooseTarget(ctx context.Context, client *boundary.Client, defaultScopeID string) (targetMatch, error) {
	matches, err := a.listProjectTargets(ctx, client)
	if err != nil {
		return targetMatch{}, err
	}
	if len(matches) == 0 {
		return targetMatch{}, errors.New("no connectable targets found in any project scope")
	}

	defaultIndex := 0
	options := make([]ui.Option, 0, len(matches))
	for idx, match := range matches {
		if match.scope.ID == defaultScopeID {
			defaultIndex = idx
		}
		options = append(options, ui.Option{
			Label:       match.target.Name,
			Description: fmt.Sprintf("%s • %s (%s) • port %d", match.target.ID, displayScope(match.scope), match.scope.ID, match.target.DefaultPort),
			Value:       match.target.ID,
		})
	}

	choice, err := ui.RunSelect("Choose the target", options, defaultIndex)
	if err != nil {
		return targetMatch{}, err
	}

	for _, match := range matches {
		if match.target.ID == choice.Value {
			return match, nil
		}
	}

	return targetMatch{}, fmt.Errorf("target %s was selected but could not be resolved", choice.Value)
}

func (a *App) listProjectTargets(ctx context.Context, client *boundary.Client) ([]targetMatch, error) {
	scopes, err := client.ListScopes(ctx)
	if err != nil {
		return nil, err
	}

	projects := boundary.FilterProjectScopes(scopes)
	if len(projects) == 0 {
		return nil, errors.New("no project scopes found")
	}

	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(displayScope(projects[i])) < strings.ToLower(displayScope(projects[j]))
	})

	matches := make([]targetMatch, 0)
	for _, scope := range projects {
		targets, err := client.ListTargets(ctx, scope.ID)
		if err != nil {
			return nil, fmt.Errorf("list targets for scope %s: %w", scope.ID, err)
		}

		sort.Slice(targets, func(i, j int) bool {
			if strings.EqualFold(targets[i].Name, targets[j].Name) {
				return targets[i].ID < targets[j].ID
			}
			return strings.ToLower(targets[i].Name) < strings.ToLower(targets[j].Name)
		})

		for _, target := range targets {
			matches = append(matches, targetMatch{scope: scope, target: target})
		}
	}

	return matches, nil
}

func (a *App) runTargets(ctx context.Context) error {
	cfg, changed, err := a.ensureBaseConfig()
	if err != nil {
		return err
	}
	if changed {
		if err := a.store.Save(cfg); err != nil {
			return err
		}
	}

	client := boundary.NewClient(cfg.BoundaryAddr, cfg.AuthToken)
	scope, err := a.chooseProjectScope(ctx, client, cfg.DefaultProjectScopeID)
	if err != nil {
		return err
	}

	targets, err := client.ListTargets(ctx, scope.ID)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(a.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPORT\tID")
	for _, target := range targets {
		fmt.Fprintf(tw, "%s\t%d\t%s\n", target.Name, target.DefaultPort, target.ID)
	}
	return tw.Flush()
}

func (a *App) runScopes(ctx context.Context) error {
	cfg, changed, err := a.ensureBaseConfig()
	if err != nil {
		return err
	}
	if changed {
		if err := a.store.Save(cfg); err != nil {
			return err
		}
	}

	client := boundary.NewClient(cfg.BoundaryAddr, cfg.AuthToken)
	scopes, err := client.ListScopes(ctx)
	if err != nil {
		return err
	}

	sort.Slice(scopes, func(i, j int) bool {
		return strings.ToLower(displayScope(scopes[i])) < strings.ToLower(displayScope(scopes[j]))
	})

	tw := tabwriter.NewWriter(a.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tID\tPARENT")
	for _, scope := range scopes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", displayScope(scope), scope.Type, scope.ID, scope.ParentScopeID)
	}
	return tw.Flush()
}

func (a *App) runConfigShow() error {
	cfg, err := a.store.Load()
	if err != nil {
		return err
	}

	payload := struct {
		Path   string        `json:"path"`
		Config config.Config `json:"config"`
	}{
		Path:   a.store.Path(),
		Config: redactConfig(cfg),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	fmt.Fprintln(a.stdout, string(data))
	return nil
}

func (a *App) runConfigInit() error {
	if err := a.store.Init(); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("config file already exists at %s", a.store.Path())
		}
		return err
	}

	a.printSuccess(fmt.Sprintf("Created config file at %s", a.store.Path()))
	return nil
}

func (a *App) runConfigPath() error {
	fmt.Fprintln(a.stdout, a.store.Path())
	return nil
}

func (a *App) ensureBaseConfig() (config.Config, bool, error) {
	cfg, err := a.store.Load()
	if err != nil {
		return config.Config{}, false, err
	}

	changed := false
	if strings.TrimSpace(cfg.BoundaryAddr) == "" {
		addr, err := ui.RunTextInput(
			"Boundary controller URL",
			"https://boundary.example.com",
			"",
			validateURL,
		)
		if err != nil {
			return config.Config{}, false, err
		}
		cfg.BoundaryAddr = addr
		changed = true
	}

	if strings.TrimSpace(cfg.AuthToken) == "" && strings.TrimSpace(os.Getenv("BOUNDARY_TOKEN")) == "" {
		return config.Config{}, false, errors.New("no Boundary auth token configured; run bndry login or set BNDRY_AUTH_TOKEN/BOUNDARY_TOKEN")
	}

	return cfg, changed, nil
}

func (a *App) chooseProjectScope(ctx context.Context, client *boundary.Client, defaultScopeID string) (boundary.Scope, error) {
	scopes, err := client.ListScopes(ctx)
	if err != nil {
		return boundary.Scope{}, err
	}

	projects := boundary.FilterProjectScopes(scopes)
	if len(projects) == 0 {
		return boundary.Scope{}, errors.New("no project scopes found")
	}

	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(displayScope(projects[i])) < strings.ToLower(displayScope(projects[j]))
	})

	cfg, err := a.store.Load()
	if err != nil {
		return boundary.Scope{}, err
	}
	if cfg.AutoSelectDefaultScope && strings.TrimSpace(defaultScopeID) != "" {
		for _, scope := range projects {
			if scope.ID == defaultScopeID {
				return scope, nil
			}
		}
	}

	options := make([]ui.Option, 0, len(projects))
	defaultIndex := 0
	for idx, scope := range projects {
		description := fmt.Sprintf("%s • parent %s", scope.ID, scope.ParentScopeID)
		if scope.ID == defaultScopeID {
			description += " • default"
			defaultIndex = idx
		}
		options = append(options, ui.Option{
			Label:       displayScope(scope),
			Description: description,
			Value:       scope.ID,
		})
	}

	choice, err := ui.RunSelect("Choose the Boundary project scope", options, defaultIndex)
	if err != nil {
		return boundary.Scope{}, err
	}

	for _, scope := range projects {
		if scope.ID == choice.Value {
			return scope, nil
		}
	}

	return boundary.Scope{}, fmt.Errorf("scope %s was selected but could not be resolved", choice.Value)
}

func displayScope(scope boundary.Scope) string {
	if strings.TrimSpace(scope.Name) != "" {
		return scope.Name
	}

	return scope.ID
}

func displayValue(name string, fallback string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}

	return fallback
}

func validateRequired(label string) ui.Validator {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
		return nil
	}
}

func validateURL(value string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil {
		return errors.New("enter a full URL like https://boundary.example.com")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("enter a full URL like https://boundary.example.com")
	}

	return nil
}

func validatePort(value string) error {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errors.New("enter a valid TCP port")
	}
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func redactConfig(cfg config.Config) config.Config {
	redacted := cfg
	redacted.AuthToken = redactToken(redacted.AuthToken)
	return redacted
}

func redactToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return strings.Repeat("*", len(trimmed))
	}

	return trimmed[:4] + "…" + trimmed[len(trimmed)-4:]
}

func (a *App) wrapSSHConnectError(err error, targetName string) error {
	if strings.Contains(strings.ToLower(err.Error()), "no host sources or address found") {
		return fmt.Errorf("%w\ninspect the target with `bndry ssh inspect %s`", err, targetName)
	}

	return err
}

func (a *App) printInfo(message string) {
	fmt.Fprintln(a.stdout, ui.RenderInfo(message))
}

func (a *App) printSuccess(message string) {
	fmt.Fprintln(a.stdout, ui.RenderSuccess(message))
}

func (a *App) printWarning(message string) {
	fmt.Fprintln(a.stdout, ui.RenderWarning(message))
}
