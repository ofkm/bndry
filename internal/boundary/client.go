package boundary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	boundaryapi "github.com/hashicorp/boundary/api"
	apiauthmethods "github.com/hashicorp/boundary/api/authmethods"
	apihostcatalogs "github.com/hashicorp/boundary/api/hostcatalogs"
	apihosts "github.com/hashicorp/boundary/api/hosts"
	apihostsets "github.com/hashicorp/boundary/api/hostsets"
	apiproxy "github.com/hashicorp/boundary/api/proxy"
	apiroles "github.com/hashicorp/boundary/api/roles"
	apiscopes "github.com/hashicorp/boundary/api/scopes"
	apitargets "github.com/hashicorp/boundary/api/targets"
)

const oidcPollInterval = 1500 * time.Millisecond

var localhostAddr = netip.MustParseAddr("127.0.0.1")

var sshHostAliasSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// OIDCAuthStart contains the values needed to complete an OIDC login.
type OIDCAuthStart struct {
	AuthMethodID string
	AuthURL      string
	TokenID      string
}

// Client is a small SDK-backed Boundary client for bndry workflows.
type Client struct {
	addr  string
	token string
}

// NewClient creates a Boundary client.
func NewClient(addr string, token string) *Client {
	return &Client{
		addr:  strings.TrimSpace(addr),
		token: strings.TrimSpace(token),
	}
}

// StartOIDCAuth starts an interactive OIDC authentication flow.
func (c *Client) StartOIDCAuth(ctx context.Context, authMethodID string) (*OIDCAuthStart, error) {
	apiClient, err := c.unauthenticatedAPIClient()
	if err != nil {
		return nil, err
	}

	authMethodID, err = c.resolveOIDCAuthMethodID(ctx, apiClient, authMethodID)
	if err != nil {
		return nil, err
	}

	result, err := apiauthmethods.NewClient(apiClient).Authenticate(ctx, authMethodID, "start", nil)
	if err != nil {
		return nil, fmt.Errorf("start OIDC authentication: %w", err)
	}

	start := new(apiauthmethods.OidcAuthMethodAuthenticateStartResponse)
	if err := json.Unmarshal(result.GetRawAttributes(), start); err != nil {
		return nil, fmt.Errorf("decode OIDC authentication start response: %w", err)
	}
	if strings.TrimSpace(start.AuthUrl) == "" {
		return nil, errors.New("OIDC authentication start response did not include an auth URL")
	}
	if strings.TrimSpace(start.TokenId) == "" {
		return nil, errors.New("OIDC authentication start response did not include a token ID")
	}

	return &OIDCAuthStart{AuthMethodID: authMethodID, AuthURL: start.AuthUrl, TokenID: start.TokenId}, nil
}

// WaitForOIDCToken polls Boundary until the OIDC flow returns an auth token.
func (c *Client) WaitForOIDCToken(ctx context.Context, authMethodID string, tokenID string) (string, error) {
	if strings.TrimSpace(tokenID) == "" {
		return "", errors.New("token ID is required")
	}

	apiClient, err := c.unauthenticatedAPIClient()
	if err != nil {
		return "", err
	}

	authMethodID, err = c.resolveOIDCAuthMethodID(ctx, apiClient, authMethodID)
	if err != nil {
		return "", err
	}

	authClient := apiauthmethods.NewClient(apiClient)
	ticker := time.NewTicker(oidcPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("wait for OIDC token: %w", ctx.Err())
		case <-ticker.C:
			result, err := authClient.Authenticate(ctx, authMethodID, "token", map[string]any{"token_id": tokenID})
			if err != nil {
				return "", fmt.Errorf("fetch OIDC auth token: %w", err)
			}
			if result.GetResponse() != nil && result.GetResponse().StatusCode() == 202 {
				continue
			}

			token, err := result.GetAuthToken()
			if err != nil {
				return "", fmt.Errorf("decode OIDC auth token: %w", err)
			}
			if token == nil || strings.TrimSpace(token.Token) == "" {
				return "", errors.New("OIDC authentication completed without returning a usable auth token")
			}

			return token.Token, nil
		}
	}
}

func (c *Client) resolveOIDCAuthMethodID(ctx context.Context, apiClient *boundaryapi.Client, authMethodID string) (string, error) {
	trimmed := strings.TrimSpace(authMethodID)
	if trimmed != "" {
		return trimmed, nil
	}

	result, err := apiauthmethods.NewClient(apiClient).List(ctx, "global", apiauthmethods.WithRecursive(true))
	if err != nil {
		return "", fmt.Errorf("list auth methods: %w", err)
	}

	return pickOIDCAuthMethodID(result.GetItems(), "")
}

func pickOIDCAuthMethodID(authMethods []*apiauthmethods.AuthMethod, preferred string) (string, error) {
	trimmed := strings.TrimSpace(preferred)
	if trimmed != "" {
		return trimmed, nil
	}

	oidcMethods := make([]*apiauthmethods.AuthMethod, 0, len(authMethods))
	for _, authMethod := range authMethods {
		if authMethod == nil || !strings.EqualFold(authMethod.Type, "oidc") {
			continue
		}
		oidcMethods = append(oidcMethods, authMethod)
		if authMethod.IsPrimary && strings.TrimSpace(authMethod.Id) != "" {
			return authMethod.Id, nil
		}
	}

	switch len(oidcMethods) {
	case 0:
		return "", errors.New("no OIDC auth methods found; configure one or set oidc_auth_method_id")
	case 1:
		if strings.TrimSpace(oidcMethods[0].Id) == "" {
			return "", errors.New("the only available OIDC auth method is missing an ID")
		}
		return oidcMethods[0].Id, nil
	default:
		return "", errors.New("multiple OIDC auth methods found but no primary/default provider is configured; set oidc_auth_method_id")
	}
}

// ListScopes returns all scopes recursively from global.
func (c *Client) ListScopes(ctx context.Context) ([]Scope, error) {
	apiClient, err := c.apiClient()
	if err != nil {
		return nil, err
	}

	result, err := apiscopes.NewClient(apiClient).List(ctx, "global", apiscopes.WithRecursive(true))
	if err != nil {
		return nil, fmt.Errorf("list scopes: %w", err)
	}

	items := result.GetItems()
	scopes := make([]Scope, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		scopes = append(scopes, Scope{
			ID:            item.Id,
			Name:          item.Name,
			Type:          item.Type,
			ParentScopeID: item.ScopeId,
		})
	}

	return scopes, nil
}

// ListTargets lists targets for a scope.
func (c *Client) ListTargets(ctx context.Context, scopeID string) ([]Target, error) {
	if strings.TrimSpace(scopeID) == "" {
		return nil, errors.New("scope ID is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return nil, err
	}

	result, err := apitargets.NewClient(apiClient).List(ctx, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list targets: %w", err)
	}

	items := result.GetItems()
	targets := make([]Target, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}

		defaultPort, err := targetDefaultPort(item)
		if err != nil {
			return nil, fmt.Errorf("read target %s port: %w", item.Id, err)
		}

		targets = append(targets, Target{
			ID:            item.Id,
			Name:          item.Name,
			Type:          item.Type,
			ScopeID:       item.ScopeId,
			DefaultPort:   defaultPort,
			Address:       item.Address,
			HostSourceIDs: append([]string(nil), item.HostSourceIds...),
		})
	}

	return targets, nil
}

// InspectTarget returns the target plus resolved host sources, host sets, and hosts.
func (c *Client) InspectTarget(ctx context.Context, targetID string) (*TargetInspection, error) {
	if strings.TrimSpace(targetID) == "" {
		return nil, errors.New("target ID is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return nil, err
	}

	targetResult, err := apitargets.NewClient(apiClient).Read(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("read target: %w", err)
	}
	item := targetResult.GetItem()
	if item == nil {
		return nil, errors.New("read target returned no item")
	}

	defaultPort, err := targetDefaultPort(item)
	if err != nil {
		return nil, fmt.Errorf("read target %s port: %w", item.Id, err)
	}

	inspection := &TargetInspection{
		Target: Target{
			ID:            item.Id,
			Name:          item.Name,
			Type:          item.Type,
			ScopeID:       item.ScopeId,
			DefaultPort:   defaultPort,
			Address:       item.Address,
			HostSourceIDs: append([]string(nil), item.HostSourceIds...),
		},
	}

	hostSetClient := apihostsets.NewClient(apiClient)
	hostCatalogClient := apihostcatalogs.NewClient(apiClient)
	hostClient := apihosts.NewClient(apiClient)

	for _, hostSourceID := range item.HostSourceIds {
		hostSetResult, err := hostSetClient.Read(ctx, hostSourceID)
		if err != nil {
			return nil, fmt.Errorf("read host set %s: %w", hostSourceID, err)
		}
		hostSetItem := hostSetResult.GetItem()
		if hostSetItem == nil {
			return nil, fmt.Errorf("read host set %s returned no item", hostSourceID)
		}

		hostCatalogResult, err := hostCatalogClient.Read(ctx, hostSetItem.HostCatalogId)
		if err != nil {
			return nil, fmt.Errorf("read host catalog %s: %w", hostSetItem.HostCatalogId, err)
		}
		hostCatalogItem := hostCatalogResult.GetItem()
		if hostCatalogItem == nil {
			return nil, fmt.Errorf("read host catalog %s returned no item", hostSetItem.HostCatalogId)
		}

		details := TargetHostSourceDetails{
			HostSet: HostSet{
				ID:            hostSetItem.Id,
				Name:          hostSetItem.Name,
				Type:          hostSetItem.Type,
				HostCatalogID: hostSetItem.HostCatalogId,
				HostIDs:       append([]string(nil), hostSetItem.HostIds...),
			},
			HostCatalog: HostCatalog{
				ID:      hostCatalogItem.Id,
				Name:    hostCatalogItem.Name,
				Type:    hostCatalogItem.Type,
				ScopeID: hostCatalogItem.ScopeId,
			},
		}

		for _, hostID := range hostSetItem.HostIds {
			hostResult, err := hostClient.Read(ctx, hostID)
			if err != nil {
				return nil, fmt.Errorf("read host %s: %w", hostID, err)
			}
			hostItem := hostResult.GetItem()
			if hostItem == nil {
				return nil, fmt.Errorf("read host %s returned no item", hostID)
			}

			host := Host{
				ID:          hostItem.Id,
				Name:        hostItem.Name,
				Type:        hostItem.Type,
				HostSetIDs:  append([]string(nil), hostItem.HostSetIds...),
				IPAddresses: append([]string(nil), hostItem.IpAddresses...),
				DNSNames:    append([]string(nil), hostItem.DnsNames...),
			}

			if strings.EqualFold(hostItem.Type, "static") {
				attrs, err := hostItem.GetStaticHostAttributes()
				if err != nil {
					return nil, fmt.Errorf("read static host attributes for %s: %w", hostID, err)
				}
				host.Address = attrs.Address
			}

			details.Hosts = append(details.Hosts, host)
		}

		inspection.HostSources = append(inspection.HostSources, details)
	}

	return inspection, nil
}

// CreateStaticHostCatalog creates a static host catalog.
func (c *Client) CreateStaticHostCatalog(ctx context.Context, scopeID string, name string) (HostCatalog, error) {
	if strings.TrimSpace(scopeID) == "" {
		return HostCatalog{}, errors.New("scope ID is required")
	}
	if strings.TrimSpace(name) == "" {
		return HostCatalog{}, errors.New("catalog name is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return HostCatalog{}, err
	}

	result, err := apihostcatalogs.NewClient(apiClient).Create(ctx, "static", scopeID, apihostcatalogs.WithName(name))
	if err != nil {
		return HostCatalog{}, fmt.Errorf("create host catalog: %w", err)
	}
	item := result.GetItem()
	if item == nil {
		return HostCatalog{}, errors.New("create host catalog returned no item")
	}

	return HostCatalog{ID: item.Id, Name: item.Name, Type: item.Type, ScopeID: item.ScopeId}, nil
}

// CreateStaticHost creates a host in a static catalog.
func (c *Client) CreateStaticHost(ctx context.Context, catalogID string, name string, address string) (Host, error) {
	if strings.TrimSpace(catalogID) == "" {
		return Host{}, errors.New("catalog ID is required")
	}
	if strings.TrimSpace(name) == "" {
		return Host{}, errors.New("host name is required")
	}
	if strings.TrimSpace(address) == "" {
		return Host{}, errors.New("host address is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return Host{}, err
	}

	result, err := apihosts.NewClient(apiClient).Create(ctx, catalogID, apihosts.WithName(name), apihosts.WithStaticHostAddress(address))
	if err != nil {
		return Host{}, fmt.Errorf("create host: %w", err)
	}
	item := result.GetItem()
	if item == nil {
		return Host{}, errors.New("create host returned no item")
	}

	attributes, err := item.GetStaticHostAttributes()
	if err != nil {
		return Host{}, fmt.Errorf("read static host attributes: %w", err)
	}

	return Host{ID: item.Id, Name: item.Name, Address: attributes.Address}, nil
}

// CreateStaticHostSet creates a static host set.
func (c *Client) CreateStaticHostSet(ctx context.Context, catalogID string, name string) (HostSet, error) {
	if strings.TrimSpace(catalogID) == "" {
		return HostSet{}, errors.New("catalog ID is required")
	}
	if strings.TrimSpace(name) == "" {
		return HostSet{}, errors.New("host set name is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return HostSet{}, err
	}

	result, err := apihostsets.NewClient(apiClient).Create(ctx, catalogID, apihostsets.WithName(name))
	if err != nil {
		return HostSet{}, fmt.Errorf("create host set: %w", err)
	}
	item := result.GetItem()
	if item == nil {
		return HostSet{}, errors.New("create host set returned no item")
	}

	return HostSet{ID: item.Id, Name: item.Name, Type: item.Type}, nil
}

// AddHostsToSet attaches one or more hosts to a host set.
func (c *Client) AddHostsToSet(ctx context.Context, hostSetID string, hostIDs ...string) error {
	if strings.TrimSpace(hostSetID) == "" {
		return errors.New("host set ID is required")
	}
	if len(hostIDs) == 0 {
		return errors.New("at least one host ID is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return err
	}

	_, err = apihostsets.NewClient(apiClient).AddHosts(ctx, hostSetID, 0, hostIDs, apihostsets.WithAutomaticVersioning(true))
	if err != nil {
		return fmt.Errorf("add hosts to host set: %w", err)
	}

	return nil
}

// CreateTCPTarget creates a TCP target, suitable for SSH.
func (c *Client) CreateTCPTarget(ctx context.Context, scopeID string, name string, port int) (Target, error) {
	if strings.TrimSpace(scopeID) == "" {
		return Target{}, errors.New("scope ID is required")
	}
	if strings.TrimSpace(name) == "" {
		return Target{}, errors.New("target name is required")
	}
	if port <= 0 {
		return Target{}, errors.New("target port must be greater than zero")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return Target{}, err
	}

	result, err := apitargets.NewClient(apiClient).Create(ctx, "tcp", scopeID, apitargets.WithName(name), apitargets.WithTcpTargetDefaultPort(uint32(port)))
	if err != nil {
		return Target{}, fmt.Errorf("create target: %w", err)
	}
	item := result.GetItem()
	if item == nil {
		return Target{}, errors.New("create target returned no item")
	}

	defaultPort, err := targetDefaultPort(item)
	if err != nil {
		return Target{}, fmt.Errorf("read target port: %w", err)
	}

	return Target{ID: item.Id, Name: item.Name, Type: item.Type, ScopeID: item.ScopeId, DefaultPort: defaultPort}, nil
}

// AddHostSetToTarget attaches a host set to a target.
func (c *Client) AddHostSetToTarget(ctx context.Context, targetID string, hostSetID string) error {
	if strings.TrimSpace(targetID) == "" {
		return errors.New("target ID is required")
	}
	if strings.TrimSpace(hostSetID) == "" {
		return errors.New("host set ID is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return err
	}

	_, err = apitargets.NewClient(apiClient).AddHostSources(ctx, targetID, 0, []string{hostSetID}, apitargets.WithAutomaticVersioning(true))
	if err != nil {
		return fmt.Errorf("add host set to target: %w", err)
	}

	return nil
}

// CreateRole creates a role in the supplied scope.
func (c *Client) CreateRole(ctx context.Context, scopeID string, name string) (Role, error) {
	if strings.TrimSpace(scopeID) == "" {
		return Role{}, errors.New("scope ID is required")
	}
	if strings.TrimSpace(name) == "" {
		return Role{}, errors.New("role name is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return Role{}, err
	}

	result, err := apiroles.NewClient(apiClient).Create(ctx, scopeID, apiroles.WithName(name))
	if err != nil {
		return Role{}, fmt.Errorf("create role: %w", err)
	}
	item := result.GetItem()
	if item == nil {
		return Role{}, errors.New("create role returned no item")
	}

	return Role{ID: item.Id, Name: item.Name, ScopeID: item.ScopeId}, nil
}

// AddGrantToRole grants authorize-session on the target.
func (c *Client) AddGrantToRole(ctx context.Context, roleID string, targetID string) error {
	if strings.TrimSpace(roleID) == "" {
		return errors.New("role ID is required")
	}
	if strings.TrimSpace(targetID) == "" {
		return errors.New("target ID is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return err
	}

	grant := fmt.Sprintf("id=%s;actions=authorize-session", targetID)
	_, err = apiroles.NewClient(apiClient).AddGrants(ctx, roleID, 0, []string{grant}, apiroles.WithAutomaticVersioning(true))
	if err != nil {
		return fmt.Errorf("add grant to role: %w", err)
	}

	return nil
}

// AddPrincipalToRole adds a user or managed group principal to a role.
func (c *Client) AddPrincipalToRole(ctx context.Context, roleID string, principalID string) error {
	if strings.TrimSpace(roleID) == "" {
		return errors.New("role ID is required")
	}
	if strings.TrimSpace(principalID) == "" {
		return errors.New("principal ID is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return err
	}

	_, err = apiroles.NewClient(apiClient).AddPrincipals(ctx, roleID, 0, []string{principalID}, apiroles.WithAutomaticVersioning(true))
	if err != nil {
		return fmt.Errorf("add principal to role: %w", err)
	}

	return nil
}

// ConnectSSH starts an interactive SSH session using a target ID.
func (c *Client) ConnectSSH(ctx context.Context, targetID, username, hostAlias string) error {
	if strings.TrimSpace(targetID) == "" {
		return errors.New("target ID is required")
	}

	apiClient, err := c.apiClient()
	if err != nil {
		return err
	}

	authzResult, err := apitargets.NewClient(apiClient).AuthorizeSession(ctx, targetID)
	if err != nil {
		return fmt.Errorf("authorize SSH session: %w", err)
	}

	authz, err := authzResult.GetSessionAuthorization()
	if err != nil {
		return fmt.Errorf("decode session authorization: %w", err)
	}
	if authz == nil {
		return errors.New("authorize SSH session returned no authorization data")
	}

	authzData, err := authz.GetSessionAuthorizationData()
	if err != nil {
		return fmt.Errorf("decode session authorization token: %w", err)
	}

	proxyCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	clientProxy, err := apiproxy.New(
		proxyCtx,
		authz.AuthorizationToken,
		apiproxy.WithApiClient(apiClient),
		apiproxy.WithSessionAuthorizationData(authzData),
		apiproxy.WithListenAddrPort(netip.AddrPortFrom(localhostAddr, 0)),
	)
	if err != nil {
		return fmt.Errorf("start client proxy: %w", err)
	}

	proxyErrCh := make(chan error, 1)
	go func() {
		proxyErrCh <- clientProxy.Start()
	}()

	listenCtx, listenCancel := context.WithTimeout(ctx, 10*time.Second)
	listenerAddr := clientProxy.ListenerAddress(listenCtx)
	listenCancel()
	if listenerAddr == "" {
		cancel()
		proxyErr := <-proxyErrCh
		if proxyErr != nil {
			return proxyErr
		}
		return errors.New("boundary client proxy did not report a listener address")
	}

	sshArgs, cleanup, err := buildSSHArgs(authz, listenerAddr, username, sshHostKeyAlias(targetID, hostAlias))
	if err != nil {
		cancel()
		proxyErr := <-proxyErrCh
		if proxyErr != nil {
			return errors.Join(err, proxyErr)
		}
		return err
	}
	sshErr := runSSHCommand(ctx, sshArgs)
	cancel()
	proxyErr := <-proxyErrCh
	cleanupErr := cleanup()

	return errors.Join(sshErr, proxyErr, cleanupErr)
}

// CreateSSHBundle performs the full host -> host set -> target -> role workflow.
func (c *Client) CreateSSHBundle(ctx context.Context, req CreateSSHBundleRequest) (*CreatedSSHBundle, error) {
	if strings.TrimSpace(req.ScopeID) == "" {
		return nil, errors.New("scope ID is required")
	}
	if strings.TrimSpace(req.CatalogName) == "" {
		return nil, errors.New("catalog name is required")
	}
	if strings.TrimSpace(req.HostName) == "" {
		return nil, errors.New("host name is required")
	}
	if strings.TrimSpace(req.HostAddress) == "" {
		return nil, errors.New("host address is required")
	}
	if strings.TrimSpace(req.HostSetName) == "" {
		return nil, errors.New("host set name is required")
	}
	if strings.TrimSpace(req.TargetName) == "" {
		return nil, errors.New("target name is required")
	}
	if req.Port <= 0 {
		req.Port = 22
	}

	catalog, err := c.CreateStaticHostCatalog(ctx, req.ScopeID, req.CatalogName)
	if err != nil {
		return nil, fmt.Errorf("create host catalog: %w", err)
	}

	host, err := c.CreateStaticHost(ctx, catalog.ID, req.HostName, req.HostAddress)
	if err != nil {
		return nil, fmt.Errorf("create host: %w", err)
	}

	hostSet, err := c.CreateStaticHostSet(ctx, catalog.ID, req.HostSetName)
	if err != nil {
		return nil, fmt.Errorf("create host set: %w", err)
	}

	if err := c.AddHostsToSet(ctx, hostSet.ID, host.ID); err != nil {
		return nil, fmt.Errorf("attach host to host set: %w", err)
	}

	target, err := c.CreateTCPTarget(ctx, req.ScopeID, req.TargetName, req.Port)
	if err != nil {
		return nil, fmt.Errorf("create target: %w", err)
	}

	if err := c.AddHostSetToTarget(ctx, target.ID, hostSet.ID); err != nil {
		return nil, fmt.Errorf("attach host set to target: %w", err)
	}

	bundle := &CreatedSSHBundle{
		Catalog: catalog,
		Host:    host,
		HostSet: hostSet,
		Target:  target,
	}

	if req.CreateRole {
		role, err := c.CreateRole(ctx, req.ScopeID, req.RoleName)
		if err != nil {
			return nil, fmt.Errorf("create role: %w", err)
		}

		if err := c.AddGrantToRole(ctx, role.ID, target.ID); err != nil {
			return nil, fmt.Errorf("grant role access: %w", err)
		}

		if strings.TrimSpace(req.PrincipalID) != "" {
			if err := c.AddPrincipalToRole(ctx, role.ID, req.PrincipalID); err != nil {
				return nil, fmt.Errorf("attach principal to role: %w", err)
			}
		}

		bundle.Role = &role
	}

	return bundle, nil
}

func (c *Client) apiClient() (*boundaryapi.Client, error) {
	return c.newAPIClient(true)
}

func (c *Client) unauthenticatedAPIClient() (*boundaryapi.Client, error) {
	return c.newAPIClient(false)
}

func (c *Client) newAPIClient(includeToken bool) (*boundaryapi.Client, error) {
	conf, err := boundaryapi.DefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("create Boundary SDK config: %w", err)
	}
	if c.addr != "" {
		conf.Addr = c.addr
	}

	client, err := boundaryapi.NewClient(conf)
	if err != nil {
		return nil, fmt.Errorf("create Boundary SDK client: %w", err)
	}

	if includeToken {
		if c.token != "" {
			client.SetToken(c.token)
		}
	} else {
		client.SetToken("")
	}

	return client, nil
}

func targetDefaultPort(item *apitargets.Target) (int, error) {
	if item == nil {
		return 0, errors.New("target is nil")
	}

	switch strings.ToLower(item.Type) {
	case "ssh":
		attributes, err := item.GetSshTargetAttributes()
		if err != nil {
			return 0, err
		}
		return int(attributes.DefaultPort), nil
	case "tcp":
		attributes, err := item.GetTcpTargetAttributes()
		if err != nil {
			return 0, err
		}
		return int(attributes.DefaultPort), nil
	default:
		return 0, nil
	}
}

func buildSSHArgs(authz *apitargets.SessionAuthorization, listenerAddr, overrideUsername, hostKeyAlias string) ([]string, func() error, error) {
	if authz == nil {
		return nil, func() error { return nil }, errors.New("session authorization is nil")
	}

	host, port, err := net.SplitHostPort(listenerAddr)
	if err != nil {
		return nil, func() error { return nil }, fmt.Errorf("parse proxy listener address: %w", err)
	}

	args := []string{"-p", port}
	cleanup := func() error { return nil }
	if strings.TrimSpace(hostKeyAlias) != "" {
		args = append(args, "-o", "HostKeyAlias="+hostKeyAlias)
	}

	credentials, err := apiproxy.ParseCredentials(authz.Credentials)
	if err != nil {
		return nil, cleanup, fmt.Errorf("parse Boundary session credentials: %w", err)
	}

	username := overrideUsername
	if username == "" {
		// Fall back to credentials from Boundary
		if len(credentials.SshPrivateKey) > 0 {
			keyCredential := credentials.SshPrivateKey[0]
			username = keyCredential.Username

			keyFile, err := writeTemporaryPrivateKey(keyCredential.PrivateKey)
			if err != nil {
				return nil, cleanup, err
			}

			cleanup = func() error {
				return os.Remove(keyFile)
			}
			args = append(args, "-i", keyFile, "-o", "IdentitiesOnly=yes")
		}

		if username == "" && len(credentials.UsernamePassword) > 0 {
			username = credentials.UsernamePassword[0].Username
		}
	} else {
		// User specified a custom username, but we may still have a private key to use
		if len(credentials.SshPrivateKey) > 0 {
			keyCredential := credentials.SshPrivateKey[0]
			keyFile, err := writeTemporaryPrivateKey(keyCredential.PrivateKey)
			if err != nil {
				return nil, cleanup, err
			}

			cleanup = func() error {
				return os.Remove(keyFile)
			}
			args = append(args, "-i", keyFile, "-o", "IdentitiesOnly=yes")
		}
	}

	if username != "" {
		args = append(args, "-l", username)
	}

	args = append(args, host)
	return args, cleanup, nil
}

func sshHostKeyAlias(targetID, hostAlias string) string {
	parts := make([]string, 0, 3)
	for _, part := range []string{hostAlias, targetID} {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		sanitized := sshHostAliasSanitizer.ReplaceAllString(trimmed, "-")
		sanitized = strings.Trim(sanitized, "-.")
		if sanitized == "" {
			continue
		}

		parts = append(parts, sanitized)
	}

	if len(parts) == 0 {
		return ""
	}

	return "bndry-" + strings.Join(parts, "-")
}

func writeTemporaryPrivateKey(privateKey string) (string, error) {
	if strings.TrimSpace(privateKey) == "" {
		return "", errors.New("private key is empty")
	}

	file, err := os.CreateTemp("", "bndry-key-*")
	if err != nil {
		return "", fmt.Errorf("create temporary private key file: %w", err)
	}

	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("set private key permissions: %w", err)
	}
	if _, err := file.WriteString(privateKey); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write private key: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close private key file: %w", err)
	}

	return file.Name(), nil
}

func runSSHCommand(ctx context.Context, args []string) error {
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("find ssh client: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh %s: %w", strings.Join(args, " "), err)
	}

	return nil
}
