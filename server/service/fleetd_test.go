package service

import (
	"context"
	"testing"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncFleetdManifest(t *testing.T) {
	ds := new(mock.Store)
	svc, ctx := newTestService(t, ds, nil, nil)
	ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
		return &fleet.AppConfig{
			ServerSettings: fleet.ServerSettings{ServerURL: "https://fleet.example.com"},
		}, nil
	}

	url, err := svc.SyncFleetdManifest(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	require.NoError(t, err)
	require.NotNil(t, url)
}

func TestSyncFleetdMetadata(t *testing.T) {
	_, ctx := newTestService(t, new(mock.Store), nil, nil)
	err := SyncFleetdMetadata(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	require.NoError(t, err)
}

func TestSaveFile(t *testing.T) {
	data := []byte("test data")
	err := saveFile("testfile.txt", data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	require.NoError(t, err)
}
