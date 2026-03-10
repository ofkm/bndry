package boundary

import (
	"reflect"
	"testing"

	apiauthmethods "github.com/hashicorp/boundary/api/authmethods"
	apitargets "github.com/hashicorp/boundary/api/targets"
)

func TestFilterProjectScopes(t *testing.T) {
	t.Parallel()

	scopes := []Scope{
		{ID: "o_1", Type: "org"},
		{ID: "p_1", Type: "project"},
		{ID: "p_2", Type: "Project"},
	}

	got := FilterProjectScopes(scopes)
	want := []Scope{{ID: "p_1", Type: "project"}, {ID: "p_2", Type: "Project"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FilterProjectScopes() = %#v, want %#v", got, want)
	}
}

func TestFindTargetByName(t *testing.T) {
	t.Parallel()

	targets := []Target{{ID: "ttcp_1", Name: "boron"}, {ID: "ttcp_2", Name: "argon"}}

	tests := []struct {
		name   string
		query  string
		wantID string
		wantOK bool
	}{
		{name: "exact", query: "boron", wantID: "ttcp_1", wantOK: true},
		{name: "case insensitive", query: "ARGON", wantID: "ttcp_2", wantOK: true},
		{name: "missing", query: "neon", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := FindTargetByName(targets, tt.query)
			if ok != tt.wantOK {
				t.Fatalf("FindTargetByName() ok = %v, want %v", ok, tt.wantOK)
			}

			if got.ID != tt.wantID {
				t.Fatalf("FindTargetByName() ID = %q, want %q", got.ID, tt.wantID)
			}
		})
	}
}

func TestFindTargetsByName(t *testing.T) {
	t.Parallel()

	targets := []Target{
		{ID: "ttcp_1", Name: "boron"},
		{ID: "ttcp_2", Name: "BORON"},
		{ID: "ttcp_3", Name: "argon"},
		{ID: "ttcp_4", Name: "boron"},
	}

	tests := []struct {
		name   string
		query  string
		wantID []string
	}{
		{name: "exact matches only", query: "boron", wantID: []string{"ttcp_1", "ttcp_4"}},
		{name: "case insensitive fallback", query: "BoRoN", wantID: []string{"ttcp_1", "ttcp_2", "ttcp_4"}},
		{name: "missing", query: "neon", wantID: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := FindTargetsByName(targets, tt.query)
			gotIDs := make([]string, 0, len(got))
			for _, target := range got {
				gotIDs = append(gotIDs, target.ID)
			}

			if !reflect.DeepEqual(gotIDs, tt.wantID) {
				t.Fatalf("FindTargetsByName() IDs = %#v, want %#v", gotIDs, tt.wantID)
			}
		})
	}
}

func TestTargetDefaultPort(t *testing.T) {
	t.Parallel()

	target := &apitargets.Target{Type: "tcp", Attributes: map[string]any{"default_port": 2222.0}}

	got, err := targetDefaultPort(target)
	if err != nil {
		t.Fatalf("targetDefaultPort() error = %v", err)
	}
	if got != 2222 {
		t.Fatalf("targetDefaultPort() = %d, want 2222", got)
	}
}

func TestPickOIDCAuthMethodID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		preferred   string
		authMethods []*apiauthmethods.AuthMethod
		want        string
		wantErr     bool
	}{
		{
			name:      "uses preferred id when provided",
			preferred: "amoidc_override",
			authMethods: []*apiauthmethods.AuthMethod{
				{Id: "amoidc_primary", Type: "oidc", IsPrimary: true},
			},
			want: "amoidc_override",
		},
		{
			name: "uses primary oidc auth method",
			authMethods: []*apiauthmethods.AuthMethod{
				{Id: "ampwd_1", Type: "password"},
				{Id: "amoidc_primary", Type: "oidc", IsPrimary: true},
				{Id: "amoidc_secondary", Type: "oidc"},
			},
			want: "amoidc_primary",
		},
		{
			name: "falls back to only oidc auth method",
			authMethods: []*apiauthmethods.AuthMethod{
				{Id: "ampwd_1", Type: "password"},
				{Id: "amoidc_only", Type: "oidc"},
			},
			want: "amoidc_only",
		},
		{
			name: "errors when multiple oidc methods exist without primary",
			authMethods: []*apiauthmethods.AuthMethod{
				{Id: "amoidc_one", Type: "oidc"},
				{Id: "amoidc_two", Type: "oidc"},
			},
			wantErr: true,
		},
		{
			name: "errors when no oidc auth methods exist",
			authMethods: []*apiauthmethods.AuthMethod{
				{Id: "ampwd_1", Type: "password"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := pickOIDCAuthMethodID(tt.authMethods, tt.preferred)
			if (err != nil) != tt.wantErr {
				t.Fatalf("pickOIDCAuthMethodID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("pickOIDCAuthMethodID() = %q, want %q", got, tt.want)
			}
		})
	}
}
