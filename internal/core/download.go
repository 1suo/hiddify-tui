package core

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultCoreVersion is the hiddify-core release the installer fetches.
const DefaultCoreVersion = "v4.1.0"

// releaseAsset returns the standalone core tarball for the current platform.
// Only Linux ships a standalone core; macOS/Windows ship a library bundled with
// the GUI.
func releaseAsset() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("no standalone core for %s; use the Hiddify GUI's bundled core", runtime.GOOS)
	}
	arch, ok := map[string]string{"amd64": "amd64", "arm64": "arm64"}[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("unsupported architecture %s", runtime.GOARCH)
	}
	return fmt.Sprintf("hiddify-core-linux-%s.tar.gz", arch), nil
}

// Download fetches the official hiddify-core release and extracts the core
// binary and its cronet library into destDir, returning the binary path.
func Download(destDir, version string) (string, error) {
	if version == "" {
		version = DefaultCoreVersion
	}
	asset, err := releaseAsset()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://github.com/hiddify/hiddify-core/releases/download/%s/%s", version, asset)
	archive, err := os.CreateTemp("", "hiddify-core-*.tar.gz")
	if err != nil {
		return "", err
	}
	defer os.Remove(archive.Name())

	response, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download core: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download core: status %d", response.StatusCode)
	}
	if _, err := io.Copy(archive, response.Body); err != nil {
		return "", fmt.Errorf("download core: %w", err)
	}
	if err := archive.Close(); err != nil {
		return "", err
	}

	binary, err := extract(archive.Name(), destDir)
	if err != nil {
		return "", err
	}
	return binary, nil
}

func extract(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzipReader.Close()

	binary := ""
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.Base(header.Name)
		if name != "hiddify-core" && name != "libcronet.so" {
			continue
		}
		destination := filepath.Join(destDir, name)
		out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, reader); err != nil {
			out.Close()
			return "", err
		}
		out.Close()
		if name == "hiddify-core" {
			binary = destination
		}
	}
	if binary == "" {
		return "", fmt.Errorf("core binary not found in archive")
	}
	return binary, nil
}
