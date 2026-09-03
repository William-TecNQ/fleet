package service

import (
	"context"
	"testing"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncFleetdMetadata(t *testing.T) {
	ds := new(mock.Store)
	cfg := config.TestConfig()
	cfg.Server.FleetdDir = t.TempDir()
	svc, ctx := newTestServiceWithConfig(t, ds, cfg, nil, nil)
	ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
		return &fleet.AppConfig{
			ServerSettings: fleet.ServerSettings{ServerURL: "https://fleet.example.com"},
		}, nil
	}

	version, err := svc.SyncFleetdMetadata(ctx)
	require.NoError(t, err)
	require.NotNil(t, version)
}

func TestSyncFleetdManifest(t *testing.T) {
	ds := new(mock.Store)
	cfg := config.TestConfig()
	cfg.Server.FleetdDir = t.TempDir()
	svc, ctx := newTestServiceWithConfig(t, ds, cfg, nil, nil)
	ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
		return &fleet.AppConfig{
			ServerSettings: fleet.ServerSettings{ServerURL: "https://fleet.example.com"},
		}, nil
	}

	version, err := svc.SyncFleetdMetadata(ctx)
	require.NoError(t, err)
	require.NotNil(t, version)

	require.NoError(t, svc.SyncFleetdManifest(ctx, version))
}

func TestSyncFleetdPackage(t *testing.T) {
	ds := new(mock.Store)
	cfg := config.TestConfig()
	cfg.Server.FleetdDir = t.TempDir()
	svc, ctx := newTestServiceWithConfig(t, ds, cfg, nil, nil)
	ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
		return &fleet.AppConfig{
			ServerSettings: fleet.ServerSettings{ServerURL: "https://fleet.example.com"},
		}, nil
	}

	version, err := svc.SyncFleetdMetadata(ctx)
	require.NoError(t, err)
	require.NotNil(t, version)

	require.NoError(t, svc.SyncFleetdPackage(ctx, version))
}
