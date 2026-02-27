package update

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type Options struct {
	Repository string
	CheckOnly  bool
	Yes        bool
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func Run(opts Options) error {
	if strings.TrimSpace(opts.Repository) == "" {
		return fmt.Errorf("repository is required")
	}

	rel, err := fetchLatestRelease(opts.Repository)
	if err != nil {
		return err
	}
	if rel.TagName == "" {
		return errors.New("latest release is missing tag_name")
	}

	if "v"+Version == rel.TagName || Version == rel.TagName {
		fmt.Printf("Already up to date (%s)\n", Version)
		return nil
	}
	if opts.CheckOnly {
		fmt.Printf("Update available: current=%s latest=%s\n", Version, rel.TagName)
		return nil
	}

	assetURL, assetName, err := selectAsset(rel)
	if err != nil {
		return err
	}

	if !opts.Yes {
		fmt.Printf("Updating from %s to %s using %s\n", Version, rel.TagName, assetName)
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "zet-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(archivePath, assetURL); err != nil {
		return err
	}

	binaryPath := filepath.Join(tmpDir, "zet")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	if err := extractTarGzBinary(archivePath, binaryPath); err != nil {
		return err
	}
	if err := replaceExecutable(exePath, binaryPath); err != nil {
		return err
	}

	fmt.Printf("Updated successfully to %s\n", rel.TagName)
	return nil
}

func fetchLatestRelease(repo string) (*githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "zet-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github API error: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func selectAsset(rel *githubRelease) (url, name string, err error) {
	target := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, asset := range rel.Assets {
		if strings.Contains(asset.Name, target) && strings.HasSuffix(asset.Name, ".tar.gz") {
			return asset.URL, asset.Name, nil
		}
	}
	return "", "", fmt.Errorf("no release asset found for %s", target)
}

func downloadFile(path, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s", resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarGzBinary(archivePath, outPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		base := filepath.Base(hdr.Name)
		if base != "zet" && base != "zet.exe" {
			continue
		}

		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	}
	return errors.New("binary not found in archive")
}

func replaceExecutable(currentPath, newBinaryPath string) error {
	info, err := os.Stat(currentPath)
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()

	in, err := os.Open(newBinaryPath)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := currentPath + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, currentPath)
}
