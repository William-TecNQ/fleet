package fleetdbase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fleetdm/fleet/v4/server/dev_mode"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/stretchr/testify/require"
)

func TestGetBaseURL(t *testing.T) {
	cfg := fleet.AppConfig{
		ServerSettings: fleet.ServerSettings{
			ServerURL:              "https://fleet.example.com",
			EnableLocalServeFleetd: false,
		},
	}
	t.Run("with env variable", func(t *testing.T) {
		dev_mode.SetOverride("FLEET_DEV_DOWNLOAD_FLEETDM_URL", "https://download-testing.fleetdm.com", t)
		require.Equal(t, "https://download-testing.fleetdm.com", getBaseURL(cfg))
	})

	t.Run("with local serve fleetd enabled", func(t *testing.T) {
		cfg.ServerSettings.EnableLocalServeFleetd = true
		require.Equal(t, "https://fleet.example.com", getBaseURL(cfg))
	})

	t.Run("without env variable", func(t *testing.T) {
		cfg.ServerSettings.EnableLocalServeFleetd = false
		require.Equal(t, "https://download.fleetdm.com", getBaseURL(cfg))
	})
}

func TestGetMetadata(t *testing.T) {
	cfg := fleet.AppConfig{
		ServerSettings: fleet.ServerSettings{
			ServerURL:              "https://fleet.example.com",
			EnableLocalServeFleetd: false,
		},
	}

	t.Run("with local serve fleetd enabled", func(t *testing.T) {
		cfg.ServerSettings.EnableLocalServeFleetd = true
		expectedMetadata := &Metadata{
			MSIURL:           "https://fleet.example.com/api/latest/fleet/fleetd/msi",
			MSISha256:        "456e4f16c437c54d4cfacb54717450f4be582e572b8a7252a0384ac3118fbd11",
			PKGURL:           "https://fleet.example.com/api/latest/fleet/fleetd/pkg",
			PKGSha256:        "4c914def2af5f4e0f5507e397d1d8af5b5991ea23cf606450787b8377e7bcecd",
			ManifestPlistURL: "https://fleet.example.com/api/latest/fleet/fleetd/manifest",
			Version:          "2024-06-25_03-01-17",
		}
		meta, err := GetMetadata(cfg)
		require.NoError(t, err)
		require.Equal(t, expectedMetadata, meta)
	})

	t.Run("without local serve fleetd enabled", func(t *testing.T) {
		cfg.ServerSettings.EnableLocalServeFleetd = false
		expectedMetadata := &Metadata{
			MSIURL:           "https://download-testing.fleetdm.com/archive/stable/2024-06-25_03-01-17/fleetd-base.msi",
			MSISha256:        "456e4f16c437c54d4cfacb54717450f4be582e572b8a7252a0384ac3118fbd11",
			PKGURL:           "https://download-testing.fleetdm.com/archive/stable/2024-06-25_03-01-17/fleetd-base.pkg",
			PKGSha256:        "4c914def2af5f4e0f5507e397d1d8af5b5991ea23cf606450787b8377e7bcecd",
			ManifestPlistURL: "https://download-testing.fleetdm.com/archive/stable/2024-06-25_03-01-17/fleetd-base-manifest.plist",
			Version:          "2024-06-25_03-01-17",
		}

		meta, err := GetMetadata(cfg)
		require.NoError(t, err)
		require.Equal(t, expectedMetadata, meta)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/stable/meta.json", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(expectedMetadata))
		}))
		t.Cleanup(server.Close)
		dev_mode.SetOverride("FLEET_DEV_DOWNLOAD_FLEETDM_URL", server.URL, t)
	})
}

func TestGetMetadataErrorScenarios(t *testing.T) {
	cfg := fleet.AppConfig{
		ServerSettings: fleet.ServerSettings{
			ServerURL:              "https://fleet.example.com",
			EnableLocalServeFleetd: false,
		},
	}
	t.Run("invalid URL", func(t *testing.T) {
		dev_mode.SetOverride("FLEET_DEV_DOWNLOAD_FLEETDM_URL", "://invalid-url", t)
		_, err := GetMetadata(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid URL")
	})

	t.Run("non-200 status code", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		dev_mode.SetOverride("FLEET_DEV_DOWNLOAD_FLEETDM_URL", server.URL, t)

		_, err := GetMetadata(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected status code")
	})

	t.Run("JSON decoding failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("{invalid-json}"))
			require.NoError(t, err)
		}))
		t.Cleanup(server.Close)
		dev_mode.SetOverride("FLEET_DEV_DOWNLOAD_FLEETDM_URL", server.URL, t)

		_, err := GetMetadata(cfg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decode response")
	})
}

func TestGetPKGManifestURL(t *testing.T) {
	cfg := fleet.AppConfig{
		ServerSettings: fleet.ServerSettings{
			ServerURL:              "https://fleet.example.com",
			EnableLocalServeFleetd: false,
		},
	}
	t.Run("with env variable", func(t *testing.T) {
		dev_mode.SetOverride("FLEET_DEV_DOWNLOAD_FLEETDM_URL", "https://download-test.fleetdm.com", t)
		require.Equal(t, "https://download-test.fleetdm.com/stable/fleetd-base-manifest.plist", GetPKGManifestURL(cfg))
	})

	t.Run("with local serve fleetd enabled", func(t *testing.T) {
		cfg.ServerSettings.EnableLocalServeFleetd = true
		require.Equal(t, "https://fleet.example.com/api/latest/fleet/fleetd/manifest", GetPKGManifestURL(cfg))
	})

	t.Run("without env variable", func(t *testing.T) {
		cfg.ServerSettings.EnableLocalServeFleetd = false
		require.Equal(t, "https://download.fleetdm.com/stable/fleetd-base-manifest.plist", GetPKGManifestURL(cfg))
	})
}
