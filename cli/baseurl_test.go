package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", ""}, {"/", ""}, {"/marketplace", "/marketplace"},
		{"/marketplace/", "/marketplace"}, {"/tools/marketplace", "/tools/marketplace"},
	} {
		got, err := normalizeBaseURL(tc.input)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
	for _, value := range []string{"marketplace", "https://example.com/marketplace", "//host", "/a//b", "/a/../b", "/a?b", "/a#b", "/a%2fb", "/a b", "/a\\b", "/{name}", "/*"} {
		_, err := normalizeBaseURL(value)
		require.Error(t, err, value)
	}
}

func TestBaseURLEnvironmentAndFlag(t *testing.T) {
	// Invalid storage prevents the server from starting after base URL validation.
	for _, tc := range []struct {
		name, env string
		args      []string
		invalid   bool
	}{
		{"environment", "invalid", nil, true},
		{"valid environment", "/marketplace", nil, false},
		{"flag overrides environment", "invalid", []string{"--base-url", "/marketplace"}, false},
		{"empty flag overrides environment", "invalid", []string{"--base-url", ""}, false},
		{"invalid flag", "/marketplace", []string{"--base-url", "invalid"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := Root()
			t.Setenv("BASE_URL", tc.env)
			cmd.SetArgs(append([]string{"server"}, tc.args...))
			err := cmd.Execute()
			require.Error(t, err)
			if tc.invalid {
				require.Contains(t, err.Error(), "BASE_URL / --base-url")
			} else {
				require.NotContains(t, err.Error(), "BASE_URL / --base-url")
			}
		})
	}
}
