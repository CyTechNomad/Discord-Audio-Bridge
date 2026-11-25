package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"runtime"
	"testing"
)

func TestGetAssetName(t *testing.T) {
	assetName := GetAssetName()

	// Just verify it returns something valid for the current platform
	if assetName == "" {
		t.Error("GetAssetName() returned empty string")
	}

	// Check it matches expected format
	switch runtime.GOOS {
	case "windows":
		if assetName != "windows-amd64" {
			t.Errorf("GetAssetName() = %s, expected windows-amd64", assetName)
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			if assetName != "darwin-arm64" {
				t.Errorf("GetAssetName() = %s, expected darwin-arm64", assetName)
			}
		} else {
			if assetName != "darwin-amd64" {
				t.Errorf("GetAssetName() = %s, expected darwin-amd64", assetName)
			}
		}
	case "linux":
		if runtime.GOARCH == "arm64" {
			if assetName != "linux-arm64" {
				t.Errorf("GetAssetName() = %s, expected linux-arm64", assetName)
			}
		} else {
			if assetName != "linux-amd64" {
				t.Errorf("GetAssetName() = %s, expected linux-amd64", assetName)
			}
		}
	}
}

func TestExtractFromZip(t *testing.T) {
	// Create a test zip file in memory
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Add a test file
	testContent := []byte("test binary content")
	fileWriter, err := zipWriter.Create("test-dir/testfile.exe")
	if err != nil {
		t.Fatalf("Failed to create zip entry: %v", err)
	}
	_, err = fileWriter.Write(testContent)
	if err != nil {
		t.Fatalf("Failed to write zip content: %v", err)
	}

	err = zipWriter.Close()
	if err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	// Test extraction
	extracted, err := ExtractFromZip(buf.Bytes(), "testfile.exe")
	if err != nil {
		t.Fatalf("ExtractFromZip() error: %v", err)
	}

	if !bytes.Equal(extracted, testContent) {
		t.Errorf("ExtractFromZip() content mismatch")
	}
}

func TestExtractFromZip_FileNotFound(t *testing.T) {
	// Create a test zip file without the target file
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	fileWriter, _ := zipWriter.Create("other-file.txt")
	fileWriter.Write([]byte("other content"))
	zipWriter.Close()

	_, err := ExtractFromZip(buf.Bytes(), "nonexistent.exe")
	if err == nil {
		t.Error("ExtractFromZip() should return error for missing file")
	}
}

func TestExtractFromTarGz(t *testing.T) {
	// Create a test tar.gz file in memory
	buf := new(bytes.Buffer)
	gzWriter := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Add a test file
	testContent := []byte("test binary content")
	header := &tar.Header{
		Name: "test-dir/testfile",
		Mode: 0755,
		Size: int64(len(testContent)),
	}
	err := tarWriter.WriteHeader(header)
	if err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	_, err = tarWriter.Write(testContent)
	if err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}

	tarWriter.Close()
	gzWriter.Close()

	// Test extraction
	extracted, err := ExtractFromTarGz(buf.Bytes(), "testfile")
	if err != nil {
		t.Fatalf("ExtractFromTarGz() error: %v", err)
	}

	if !bytes.Equal(extracted, testContent) {
		t.Errorf("ExtractFromTarGz() content mismatch")
	}
}

func TestExtractFromTarGz_FileNotFound(t *testing.T) {
	// Create a test tar.gz file without the target file
	buf := new(bytes.Buffer)
	gzWriter := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gzWriter)

	testContent := []byte("other content")
	header := &tar.Header{
		Name: "other-file.txt",
		Mode: 0644,
		Size: int64(len(testContent)),
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write(testContent)
	tarWriter.Close()
	gzWriter.Close()

	_, err := ExtractFromTarGz(buf.Bytes(), "nonexistent")
	if err == nil {
		t.Error("ExtractFromTarGz() should return error for missing file")
	}
}

func TestUpdater_CheckForUpdates_DevVersion(t *testing.T) {
	u := New("dev")
	latest := u.CheckForUpdates()

	// Dev version should skip update check and return empty
	if latest != "" {
		t.Errorf("CheckForUpdates() with dev version should return empty, got %s", latest)
	}
}

func TestNew(t *testing.T) {
	u := New("v1.0.0")

	if u.CurrentVersion != "v1.0.0" {
		t.Errorf("New() version = %s, expected v1.0.0", u.CurrentVersion)
	}

	if u.HTTPClient == nil {
		t.Error("New() HTTPClient should not be nil")
	}
}
