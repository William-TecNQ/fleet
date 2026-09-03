package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"howett.net/plist"
)

const (
	DEFAULT_FLEETD_DIR         = "/srv/fleet/fleetd"
	DEFAULT_FLEETD_URL         = "https://download.fleetdm.com/stable"
	DEFAULT_FLEETD_ARCHIVE_URL = "https://download.fleetdm.com/archive/stable"
)

type Metadata struct {
	MSIURL           string `json:"fleetd_base_msi_url"`
	MSISha256        string `json:"fleetd_base_msi_sha256"`
	PKGURL           string `json:"fleetd_base_pkg_url"`
	PKGSha256        string `json:"fleetd_base_pkg_sha256"`
	ManifestPlistURL string `json:"fleetd_base_manifest_plist_url"`
	Version          string `json:"version"`
}

type Manifest struct {
	Items []Item `plist:"items"`
}

type Item struct {
	Assets []Asset `plist:"assets"`
}

type Asset struct {
	Kind       string   `plist:"kind"`
	Sha256Size int      `plist:"sha256-size"`
	Sha256     []string `plist:"sha256s"`
	URL        string   `plist:"url"`
}

// FleetdDir is the on-disk location where synced fleetd base artifacts are
// stored and served from. It is populated from config (server.fleetd_dir) at
// construction; the constant is a defensive fallback for a zero-value config
// (e.g. a Service built directly in a test).
func (svc *Service) FleetdDir() string {
	if svc.fleetdDir != "" {
		return svc.fleetdDir
	}
	return DEFAULT_FLEETD_DIR
}

func (svc *Service) FleetdFilePath(ctx context.Context, name string) (string, error) {
	svc.authz.SkipAuthorization(ctx)
	return filepath.Join(svc.FleetdDir(), name), nil
}

func (svc *Service) saveFile(filename string, data []byte) error {
	if err := os.MkdirAll(svc.FleetdDir(), 0o755); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(svc.FleetdDir(), filename), data, 0o644)
}

func (svc *Service) savePackageFile(filename string, resp *http.Response) error {
	// filepath := fmt.Sprintf("%s/%s", svc.FleetdDir(), filename)

	if err := os.MkdirAll(svc.FleetdDir(), 0o755); err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(svc.FleetdDir(), filename))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return file.Close()
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Endpoints and response types for Fleetd synchronization requests.

type requestFleetdSyncResponse struct {
	Err error `json:"error,omitempty"`
}

func (r requestFleetdSyncResponse) Error() error { return r.Err }

func requestFleetdSyncEndpoint(ctx context.Context, req interface{}, svc fleet.Service) (fleet.Errorer, error) {
	if err := svc.SyncFleetd(ctx); err != nil {
		return requestFleetdSyncResponse{Err: err}, nil
	}
	return requestFleetdSyncResponse{}, nil
}

func (svc *Service) SyncFleetd(ctx context.Context) error {
	if err := svc.authz.Authorize(ctx, &fleet.AppConfig{}, fleet.ActionWrite); err != nil {
		return err
	}

	// do metadata sync
	version, err := svc.SyncFleetdMetadata(ctx)
	if err != nil {
		return err
	}

	// do pkg sync
	if err := svc.SyncFleetdPKG(ctx, version); err != nil {
		return err
	}

	// do msi sync
	if err := svc.SyncFleetdMSI(ctx, version); err != nil {
		return err
	}

	// do manifest sync
	if err := svc.SyncFleetdManifest(ctx, version); err != nil {
		return err
	}

	return nil
}

func (svc *Service) SyncFleetdMetadata(ctx context.Context) (*string, error) {
	rawURL := fmt.Sprintf("%s/meta.json", DEFAULT_FLEETD_URL)
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	appConfig, err := svc.ds.AppConfig(context.Background())
	if err != nil {
		return nil, err
	}

	// Get installer version from meta.json
	version := meta.Version

	// Change the PKG URL to point to the server's API endpoint instead of the FleetDM repository
	serverURL := appConfig.ServerSettings.ServerURL
	meta.MSIURL = serverURL + "/api/latest/fleet/fleetd/msi"
	meta.PKGURL = serverURL + "/api/latest/fleet/fleetd/pkg"
	meta.ManifestPlistURL = serverURL + "/api/latest/fleet/fleetd/manifest"

	newMeta, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}

	return &version, svc.saveFile("meta.json", newMeta)
}

func (svc *Service) SyncFleetdMSI(ctx context.Context, version *string) error {
	rawURL := fmt.Sprintf("%s/%v/fleetd-base.msi", DEFAULT_FLEETD_ARCHIVE_URL, *version)
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return svc.savePackageFile("fleetd-base.msi", resp)
}

func (svc *Service) SyncFleetdPKG(ctx context.Context, version *string) error {
	rawURL := fmt.Sprintf("%s/%v/fleetd-base.pkg", DEFAULT_FLEETD_ARCHIVE_URL, *version)
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return svc.savePackageFile("fleetd-base.pkg", resp)
}

func (svc *Service) SyncFleetdManifest(ctx context.Context, version *string) error {
	rawURL := fmt.Sprintf("%s/%v/fleetd-base-manifest.plist", DEFAULT_FLEETD_ARCHIVE_URL, *version)
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d, for site %s", resp.StatusCode, parsedURL.String())
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Save the original PKG URL before modifying it
	var manifest Manifest
	if _, err := plist.Unmarshal(data, &manifest); err != nil {
		return err
	}

	// Get AppConfig from the datastore
	appConfig, err := svc.ds.AppConfig(context.Background())
	if err != nil {
		return err
	}

	// Change the PKG URL to point to the server's API endpoint instead of the FleetDM repository
	serverURL := appConfig.ServerSettings.ServerURL
	manifest.Items[0].Assets[0].URL = serverURL + "/api/latest/fleet/fleetd/pkg"

	manifestBytes, err := plist.Marshal(manifest, plist.XMLFormat)
	if err != nil {
		return err
	}

	return svc.saveFile("fleetd-base-manifest.plist", manifestBytes)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Fleetd file response and endpoints for serving Fleetd-related files (meta, package, MSI, manifest)

type fleetdFileResponse struct {
	path        string // absolute path on disk
	filename    string // download name
	contentType string
	Err         error `json:"error,omitempty"`
}

func (r fleetdFileResponse) Error() error { return r.Err }

func (r fleetdFileResponse) HijackRender(ctx context.Context, w http.ResponseWriter) {
	f, err := os.Open(r.path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", r.contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, r.filename))
	_, _ = io.Copy(w, f)
}

func getFleetdMetadataEndpoint(ctx context.Context, req any, svc fleet.Service) (fleet.Errorer, error) {
	path, err := svc.FleetdFilePath(ctx, "meta.json")
	if err != nil {
		return fleetdFileResponse{
			Err: err,
		}, nil
	}
	return fleetdFileResponse{
		path:        path,
		filename:    "meta.json",
		contentType: "application/json",
	}, nil
}

func getFleetdPKGEndpoint(ctx context.Context, req any, svc fleet.Service) (fleet.Errorer, error) {
	path, err := svc.FleetdFilePath(ctx, "fleetd-base.pkg")
	if err != nil {
		return fleetdFileResponse{
			Err: err,
		}, nil
	}
	return fleetdFileResponse{
		path:        path,
		filename:    "fleetd-base.pkg",
		contentType: "application/octet-stream",
	}, nil
}

func getFleetdMSIEndpoint(ctx context.Context, req any, svc fleet.Service) (fleet.Errorer, error) {
	path, err := svc.FleetdFilePath(ctx, "fleetd-base.msi")
	if err != nil {
		return fleetdFileResponse{
			Err: err,
		}, nil
	}
	return fleetdFileResponse{
		path:        path,
		filename:    "fleetd-base.msi",
		contentType: "application/octet-stream",
	}, nil
}

func getFleetdManifestEndpoint(ctx context.Context, req any, svc fleet.Service) (fleet.Errorer, error) {
	path, err := svc.FleetdFilePath(ctx, "fleetd-base-manifest.plist")
	if err != nil {
		return fleetdFileResponse{
			Err: err,
		}, nil
	}
	return fleetdFileResponse{
		path:        path,
		filename:    "fleetd-base-manifest.plist",
		contentType: "application/xml",
	}, nil
}
