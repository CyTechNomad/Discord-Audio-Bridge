// Package updater handles checking for and applying application updates.
package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	// RepoOwner is the GitHub repository owner
	RepoOwner = "CyTechNomad"
	// RepoName is the GitHub repository name
	RepoName = "Discord-Audio-Bridge"
)

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Updater handles update checking and installation
type Updater struct {
	CurrentVersion string
	HTTPClient     *http.Client
}

// New creates a new Updater with the given current version
func New(version string) *Updater {
	return &Updater{
		CurrentVersion: version,
		HTTPClient:     &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckForUpdates checks if a newer version is available.
// Returns the latest version tag if an update is available, empty string otherwise.
func (u *Updater) CheckForUpdates() string {
	if u.CurrentVersion == "dev" {
		return "" // Skip update check for dev builds
	}

	latest, err := u.GetLatestRelease()
	if err != nil {
		return "" // Silently fail
	}

	if latest.TagName != u.CurrentVersion && latest.TagName > u.CurrentVersion {
		return latest.TagName
	}

	return ""
}

// GetLatestRelease fetches the latest release info from GitHub
func (u *Updater) GetLatestRelease() (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", RepoOwner, RepoName)

	resp, err := u.HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// Update downloads and installs the latest version
func (u *Updater) Update() error {
	fmt.Println("Checking for updates...")

	latest, err := u.GetLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if latest.TagName == u.CurrentVersion {
		fmt.Println("Already running the latest version:", u.CurrentVersion)
		return nil
	}

	fmt.Printf("Updating from %s to %s...\n", u.CurrentVersion, latest.TagName)

	// Find the right asset for this platform
	assetName := GetAssetName()
	var downloadURL string
	var isZip bool
	for _, asset := range latest.Assets {
		if strings.Contains(asset.Name, assetName) {
			downloadURL = asset.BrowserDownloadURL
			isZip = strings.HasSuffix(asset.Name, ".zip")
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no release found for platform: %s", assetName)
	}

	fmt.Printf("Downloading %s...\n", downloadURL)

	// Download the archive
	downloadClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := downloadClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Save archive to temp file
	archiveData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read download: %w", err)
	}

	// Extract binary from archive
	var binaryData []byte
	binaryName := "discord-audio-bridge"
	if runtime.GOOS == "windows" {
		binaryName = "discord-audio-bridge.exe"
	}

	if isZip {
		binaryData, err = ExtractFromZip(archiveData, binaryName)
	} else {
		binaryData, err = ExtractFromTarGz(archiveData, binaryName)
	}
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create temp file for new binary
	tmpFile, err := os.CreateTemp("", "discord-audio-bridge-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write binary to temp file
	_, err = tmpFile.Write(binaryData)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write update: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Replace old binary with new one
	backupPath := execPath + ".old"
	os.Remove(backupPath) // Remove any existing backup

	if err := os.Rename(execPath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install update: %w", err)
	}

	os.Remove(backupPath) // Clean up backup

	fmt.Println("Update successful! Please restart the application.")
	return nil
}

// GetAssetName returns the expected asset name fragment for the current platform
func GetAssetName() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "windows":
		return "windows-amd64"
	case "darwin":
		if goarch == "arm64" {
			return "darwin-arm64"
		}
		return "darwin-amd64"
	case "linux":
		if goarch == "arm64" {
			return "linux-arm64"
		}
		return "linux-amd64"
	default:
		return fmt.Sprintf("%s-%s", goos, goarch)
	}
}

// ExtractFromZip extracts a file from a zip archive
func ExtractFromZip(data []byte, filename string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	for _, file := range reader.File {
		if strings.HasSuffix(file.Name, filename) {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}

	return nil, fmt.Errorf("file %s not found in archive", filename)
}

// ExtractFromTarGz extracts a file from a tar.gz archive
func ExtractFromTarGz(data []byte, filename string) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.HasSuffix(header.Name, filename) {
			return io.ReadAll(tarReader)
		}
	}

	return nil, fmt.Errorf("file %s not found in archive", filename)
}
