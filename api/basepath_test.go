package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cdr.dev/slog/sloggers/slogtest"
	"github.com/coder/code-marketplace/api"
	"github.com/coder/code-marketplace/database"
	"github.com/coder/code-marketplace/storage"
	"github.com/coder/code-marketplace/testutil"
	"github.com/stretchr/testify/require"
)

func TestBasePath(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"", "/", "/marketplace", "/marketplace/", "/tools/marketplace"} {
		t.Run(prefix, func(t *testing.T) {
			t.Parallel()
			logger := slogtest.Make(t, nil)
			store, err := storage.NewLocalStorage(&storage.LocalOptions{ExtDir: t.TempDir()}, logger)
			require.NoError(t, err)
			manifest := testutil.ConvertExtensionToManifest(testutil.Extensions[0], storage.Version{Version: "1.0.0"})
			vsix := testutil.CreateVSIXFromManifest(t, manifest)
			_, err = store.AddExtension(context.Background(), manifest, vsix)
			require.NoError(t, err)
			handler := api.New(&api.Options{
				BasePath: prefix, Storage: store, Database: &database.NoDB{Storage: store, Logger: logger},
				Logger: logger, RateLimit: -1,
			}).Handler
			base := prefix
			if len(base) > 0 && base[len(base)-1] == '/' {
				base = base[:len(base)-1]
			}
			public := "https://example.com" + base
			request := func(method, path string, body []byte) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, bytes.NewReader(body))
				req.Header.Set("X-Forwarded-Host", "example.com")
				req.Header.Set("X-Forwarded-Proto", "https")
				rw := httptest.NewRecorder()
				handler.ServeHTTP(rw, req)
				return rw
			}
			for _, path := range []string{"/", "/healthz", "/item"} {
				require.Equal(t, http.StatusOK, request("GET", base+path, nil).Code)
			}
			if base != "" {
				require.Equal(t, http.StatusPermanentRedirect, request("GET", base, nil).Code)
				for _, path := range []string{"/healthz", base + "other/healthz"} {
					require.Equal(t, http.StatusNotFound, request("GET", path, nil).Code)
				}
			}
			payload, err := json.Marshal(api.QueryRequest{Filters: []database.Filter{{Criteria: []database.Criteria{{Type: database.Target, Value: "Microsoft.VisualStudio.Code"}}}}, Flags: database.IncludeVersions | database.IncludeFiles | database.IncludeAssetURI})
			require.NoError(t, err)
			rw := request("POST", base+"/api/extensionquery", payload)
			require.Equal(t, http.StatusOK, rw.Code)
			var query api.QueryResponse
			require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &query))
			require.Len(t, query.Results, 1)
			require.Len(t, query.Results[0].Extensions, 1)
			checkExtension := func(ext *database.Extension) {
				require.Len(t, ext.Versions, 1)
				ver := ext.Versions[0]
				require.Equal(t, public+"/assets/foo/zany/1.0.0", ver.AssetURI)
				require.Equal(t, ver.AssetURI, ver.FallbackAssetURI)
				require.NotEmpty(t, ver.Files)
				for _, file := range ver.Files {
					require.Contains(t, file.Source, public+"/files/foo/zany/1.0.0/")
					require.Equal(t, http.StatusOK, request("GET", file.Source, nil).Code)
				}
			}
			checkExtension(query.Results[0].Extensions[0])
			rw = request("GET", base+"/api/vscode/foo/zany/latest", nil)
			require.Equal(t, http.StatusOK, rw.Code)
			var latest database.Extension
			require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &latest))
			checkExtension(&latest)
			for _, path := range []string{
				"/assets/foo/zany/1.0.0/Microsoft.VisualStudio.Services.VSIXPackage",
				"/publishers/foo/vsextensions/zany/1.0.0/vspackage",
				"/api/publishers/foo/vsextensions/zany/1.0.0/vspackage",
			} {
				rw = request("GET", base+path, nil)
				require.Equal(t, http.StatusMovedPermanently, rw.Code)
				location := rw.Header().Get("Location")
				require.Contains(t, location, public+"/files/foo/zany/1.0.0/")
				download := request("GET", location, nil)
				require.Equal(t, http.StatusOK, download.Code)
				require.Equal(t, vsix, download.Body.Bytes())
			}
		})
	}
}
