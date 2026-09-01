package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"howett.net/plist"
)

const (
	DEFAULT_FLEETD_DIR = "/srv/fleet/fleetd"
	DEFAULT_FLEETD_URL = "https://download.fleetdm.com/stable"
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

func saveFile(filename string, data []byte) error {
	filepath := fmt.Sprintf("%s/%s", DEFAULT_FLEETD_DIR, filename)

	if err := os.MkdirAll(DEFAULT_FLEETD_DIR, 0o755); err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0o644)
}

func savePackageFile(filename string, resp *http.Response) error {
	filepath := fmt.Sprintf("%s/%s", DEFAULT_FLEETD_DIR, filename)

	if err := os.MkdirAll(DEFAULT_FLEETD_DIR, 0o755); err != nil {
		return err
	}

	file, err := os.Create(filepath)
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

type requestFleetdSyncRequest struct{}

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
	// do manifest sync
	timestamp, err := svc.SyncFleetdManifest(ctx)
	if err != nil {
		return err
	}

	// do metadata sync
	err = SyncFleetdMetadata(ctx)
	if err != nil {
		return err
	}

	// do package  sync
	err = SyncFleetdPackage(ctx, *timestamp)
	if err != nil {
		return err
	}

	return nil
}

func (svc *Service) SyncFleetdManifest(ctx context.Context) (*string, error) {
	rawURL := fmt.Sprintf("%s/fleetd-base-manifest.plist", DEFAULT_FLEETD_URL)
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

	// Save the original PKG URL before modifying it
	var manifest Manifest
	if _, err := plist.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	url := &manifest.Items[0].Assets[0].URL

	appConfig, err := svc.ds.AppConfig(context.Background())
	if err != nil {
		return nil, err
	}

	// Change the PKG URL to point to the server's API endpoint instead of the FleetDM repository
	serverURL := appConfig.ServerSettings.ServerURL
	manifest.Items[0].Assets[0].URL = serverURL + "/api/latest/fleet/fleetd/package"

	manifestBytes, err := plist.Marshal(manifest, plist.XMLFormat)
	if err != nil {
		return nil, err
	}

	// Write to file
	err = saveFile("fleetd-base-manifest.plist", manifestBytes)
	if err != nil {
		return nil, err
	}

	return url, nil
}

func SyncFleetdMetadata(ctx context.Context) error {
	rawURL := fmt.Sprintf("%s/meta.json", DEFAULT_FLEETD_URL)
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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var meta Metadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return err
	}
	return saveFile("meta.json", data)
}

func SyncFleetdPackage(ctx context.Context, timestamp string) error {
	rawURL := fmt.Sprintf("%s/%s/fleetd-base.pkg", DEFAULT_FLEETD_URL, timestamp)
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

	return savePackageFile("fleetd-base.pkg", resp)
}

type getFleetdManifestRequest struct{}

type getFleetdManifestResponse struct{}

func getFleetdManifestEndpoint(ctx context.Context, req any, svc fleet.Service) (fleet.Errorer, error) {
	return nil, nil
}

type getFleetdMetadataRequest struct{}

type getFleetdMetadataResponse struct{}

func getFleetdMetadataEndpoint(ctx context.Context, req any, svc fleet.Service) (fleet.Errorer, error) {
	return nil, nil
}

type getFleetdPackageRequest struct{}

type getFleetdPackageResponse struct{}

func getFleetdPackageEndpoint(ctx context.Context, req any, svc fleet.Service) (fleet.Errorer, error) {
	return nil, nil
}
