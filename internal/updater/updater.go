package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/VaderChen/Integrate-Terminal/internal/model"
)

const (
	latestReleaseAPI = "https://api.github.com/repos/VaderChen/Integrate-Terminal/releases/latest"
	maximumAssetSize = int64(2 * 1024 * 1024 * 1024)
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Assets  []githubAsset `json:"assets"`
}

type PreparedUpdate struct {
	Target     string
	Downloaded bool
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

func CheckLatest(ctx context.Context, currentVersion string) (model.UpdateCheckResult, error) {
	release, err := fetchLatestRelease(ctx, currentVersion)
	if err != nil {
		return model.UpdateCheckResult{}, err
	}

	latestVersion, err := normalizedVersion(release.TagName)
	if err != nil {
		return model.UpdateCheckResult{}, fmt.Errorf("latest release tag is invalid: %w", err)
	}
	currentVersion, err = normalizedVersion(currentVersion)
	if err != nil {
		return model.UpdateCheckResult{}, fmt.Errorf("current application version is invalid: %w", err)
	}
	comparison, err := compareVersions(latestVersion, currentVersion)
	if err != nil {
		return model.UpdateCheckResult{}, err
	}

	result := model.UpdateCheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		LatestTag:       release.TagName,
		UpdateAvailable: comparison > 0,
	}
	if asset, ok := selectAsset(release.Assets, runtime.GOOS, runtime.GOARCH); ok {
		result.AssetName = asset.Name
		result.CanDownload = true
	}
	return result, nil
}

func PrepareLatest(ctx context.Context, currentVersion string, expectedTag string) (PreparedUpdate, error) {
	release, err := fetchLatestRelease(ctx, currentVersion)
	if err != nil {
		return PreparedUpdate{}, err
	}
	if strings.TrimSpace(expectedTag) == "" || release.TagName != strings.TrimSpace(expectedTag) {
		return PreparedUpdate{}, errors.New("the latest release changed; check for updates again")
	}

	latestVersion, err := normalizedVersion(release.TagName)
	if err != nil {
		return PreparedUpdate{}, err
	}
	currentVersion, err = normalizedVersion(currentVersion)
	if err != nil {
		return PreparedUpdate{}, err
	}
	comparison, err := compareVersions(latestVersion, currentVersion)
	if err != nil {
		return PreparedUpdate{}, err
	}
	if comparison <= 0 {
		return PreparedUpdate{}, errors.New("the installed version is already up to date")
	}

	asset, ok := selectAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if !ok {
		if strings.TrimSpace(release.HTMLURL) == "" {
			return PreparedUpdate{}, errors.New("the release has no compatible download or web page")
		}
		return PreparedUpdate{Target: release.HTMLURL}, nil
	}

	targetPath, err := downloadAsset(ctx, asset, currentVersion)
	if err != nil {
		return PreparedUpdate{}, err
	}
	return PreparedUpdate{
		Downloaded: true,
		Target:     targetPath,
	}, nil
}

func fetchLatestRelease(ctx context.Context, currentVersion string) (githubRelease, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseAPI, nil)
	if err != nil {
		return githubRelease{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "IntegTERM/"+strings.TrimSpace(currentVersion))

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return githubRelease{}, fmt.Errorf("GitHub release request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return githubRelease{}, fmt.Errorf("GitHub release request returned %s", response.Status)
	}

	var release githubRelease
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024))
	if err := decoder.Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("GitHub release response is invalid: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" || strings.TrimSpace(release.HTMLURL) == "" {
		return githubRelease{}, errors.New("GitHub release response is incomplete")
	}
	return release, nil
}

func selectAsset(assets []githubAsset, goos string, goarch string) (githubAsset, bool) {
	type candidate struct {
		asset githubAsset
		score int
	}
	candidates := make([]candidate, 0, len(assets))
	for _, asset := range assets {
		if !validAsset(asset) {
			continue
		}
		score := assetScore(asset.Name, goos, goarch)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, candidate{asset: asset, score: score})
	}
	if len(candidates) == 0 {
		return githubAsset{}, false
	}
	sort.Slice(candidates, func(left int, right int) bool {
		if candidates[left].score != candidates[right].score {
			return candidates[left].score > candidates[right].score
		}
		return candidates[left].asset.Name < candidates[right].asset.Name
	})
	return candidates[0].asset, true
}

func validAsset(asset githubAsset) bool {
	if asset.Size <= 0 || asset.Size > maximumAssetSize {
		return false
	}
	if filepath.Base(asset.Name) != asset.Name || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
		return false
	}
	_, ok := expectedSHA256(asset.Digest)
	return ok
}

func assetScore(name string, goos string, goarch string) int {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	if lowerName == "" || !strings.Contains(lowerName, "integterm") {
		return 0
	}
	if strings.Contains(lowerName, "checksum") || strings.Contains(lowerName, "sha256") || strings.HasSuffix(lowerName, ".sig") {
		return 0
	}
	if hasOtherPlatformMarker(lowerName, goos) || hasOtherArchitectureMarker(lowerName, goarch) {
		return 0
	}
	if strings.HasSuffix(lowerName, ".zip") && !hasPlatformMarker(lowerName, goos) {
		return 0
	}

	score := 0
	switch goos {
	case "darwin":
		score = extensionScore(lowerName, map[string]int{".dmg": 100, ".pkg": 90, ".zip": 50})
	case "windows":
		score = extensionScore(lowerName, map[string]int{".msi": 100, ".exe": 90, ".zip": 50})
	case "linux":
		score = extensionScore(lowerName, map[string]int{".appimage": 100, ".deb": 95, ".rpm": 95, ".tar.gz": 90, ".zip": 50})
		if score == 0 && isLinuxRawExecutableName(lowerName, goarch) {
			score = 80
		}
	}
	if score == 0 {
		return 0
	}
	if hasPlatformMarker(lowerName, goos) {
		score += 10
	}
	if hasArchitectureMarker(lowerName, goarch) {
		score += 5
	}
	return score
}

func extensionScore(name string, scores map[string]int) int {
	best := 0
	for extension, score := range scores {
		if strings.HasSuffix(name, extension) && score > best {
			best = score
		}
	}
	return best
}

func hasPlatformMarker(name string, goos string) bool {
	markers := map[string][]string{
		"darwin":  {"darwin", "macos", "mac-", "osx"},
		"windows": {"windows", "win64", "win-"},
		"linux":   {"linux"},
	}
	return containsAny(name, markers[goos])
}

func hasOtherPlatformMarker(name string, goos string) bool {
	for platform := range map[string]struct{}{"darwin": {}, "windows": {}, "linux": {}} {
		if platform != goos && hasPlatformMarker(name, platform) {
			return true
		}
	}
	return false
}

func hasArchitectureMarker(name string, goarch string) bool {
	markers := map[string][]string{
		"amd64": {"amd64", "x86_64", "x64"},
		"arm64": {"arm64", "aarch64"},
	}
	return containsAny(name, markers[goarch])
}

func hasOtherArchitectureMarker(name string, goarch string) bool {
	for architecture := range map[string]struct{}{"amd64": {}, "arm64": {}} {
		if architecture != goarch && hasArchitectureMarker(name, architecture) {
			return true
		}
	}
	return false
}

func containsAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func downloadAsset(ctx context.Context, asset githubAsset, currentVersion string) (string, error) {
	expectedDigest, ok := expectedSHA256(asset.Digest)
	if !ok {
		return "", errors.New("the update asset does not provide a valid SHA-256 digest")
	}
	cacheDirectory, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(cacheDirectory) == "" {
		cacheDirectory = os.TempDir()
	}
	updateDirectory := filepath.Join(cacheDirectory, "IntegTERM", "updates")
	if err := os.MkdirAll(updateDirectory, 0o755); err != nil {
		return "", fmt.Errorf("cannot create update directory: %w", err)
	}
	targetPath := filepath.Join(updateDirectory, asset.Name)
	if matchesDigest(targetPath, expectedDigest) {
		return targetPath, nil
	}

	temporaryFile, err := os.CreateTemp(updateDirectory, ".integterm-update-*")
	if err != nil {
		return "", fmt.Errorf("cannot create update file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	completed := false
	defer func() {
		_ = temporaryFile.Close()
		if !completed {
			_ = os.Remove(temporaryPath)
		}
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "IntegTERM/"+strings.TrimSpace(currentVersion))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("update download failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update download returned %s", response.Status)
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporaryFile, hash), io.LimitReader(response.Body, maximumAssetSize+1))
	if err != nil {
		return "", fmt.Errorf("cannot save update: %w", err)
	}
	if written > maximumAssetSize || written != asset.Size {
		return "", fmt.Errorf("downloaded update size is invalid: got %d bytes, expected %d", written, asset.Size)
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualDigest, expectedDigest) {
		return "", errors.New("downloaded update failed SHA-256 verification")
	}
	if err := temporaryFile.Sync(); err != nil {
		return "", fmt.Errorf("cannot flush update file: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return "", fmt.Errorf("cannot close update file: %w", err)
	}
	if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cannot replace cached update: %w", err)
	}
	if err := os.Rename(temporaryPath, targetPath); err != nil {
		return "", fmt.Errorf("cannot finalize update file: %w", err)
	}
	if runtime.GOOS == "linux" && isLinuxExecutableAsset(asset.Name) {
		_ = os.Chmod(targetPath, 0o755)
	}
	completed = true
	return targetPath, nil
}

func isLinuxExecutableAsset(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.HasSuffix(lowerName, ".appimage") || isLinuxRawExecutableName(lowerName, runtime.GOARCH)
}

func isLinuxRawExecutableName(name string, goarch string) bool {
	markers := map[string][]string{
		"amd64": {"linux-amd64", "linux-x86_64", "linux-x64"},
		"arm64": {"linux-arm64", "linux-aarch64"},
	}
	return containsAny(name, markers[goarch])
}

func expectedSHA256(digest string) (string, bool) {
	algorithm, value, found := strings.Cut(strings.TrimSpace(digest), ":")
	if !found || !strings.EqualFold(strings.TrimSpace(algorithm), "sha256") {
		return "", false
	}
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return "", false
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", false
	}
	return strings.ToLower(value), true
}

func matchesDigest(targetPath string, expectedDigest string) bool {
	file, err := os.Open(targetPath)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, maximumAssetSize+1)); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedDigest)
}

func normalizedVersion(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimLeftFunc(value, func(character rune) bool {
		return character == 'v' || character == 'V' || unicode.IsSpace(character)
	})
	if separator := strings.IndexAny(value, "+-"); separator >= 0 {
		value = value[:separator]
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 {
		return "", errors.New("version is empty")
	}
	for _, part := range parts {
		if part == "" || strings.IndexFunc(part, func(character rune) bool { return !unicode.IsDigit(character) }) >= 0 {
			return "", fmt.Errorf("version %q is not numeric", value)
		}
	}
	return strings.Join(parts, "."), nil
}

func compareVersions(left string, right string) (int, error) {
	left, err := normalizedVersion(left)
	if err != nil {
		return 0, err
	}
	right, err = normalizedVersion(right)
	if err != nil {
		return 0, err
	}
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	partCount := len(leftParts)
	if len(rightParts) > partCount {
		partCount = len(rightParts)
	}
	for index := 0; index < partCount; index++ {
		leftPart := "0"
		rightPart := "0"
		if index < len(leftParts) {
			leftPart = normalizedNumericPart(leftParts[index])
		}
		if index < len(rightParts) {
			rightPart = normalizedNumericPart(rightParts[index])
		}
		if len(leftPart) != len(rightPart) {
			if len(leftPart) > len(rightPart) {
				return 1, nil
			}
			return -1, nil
		}
		if leftPart != rightPart {
			if leftPart > rightPart {
				return 1, nil
			}
			return -1, nil
		}
	}
	return 0, nil
}

func normalizedNumericPart(value string) string {
	value = strings.TrimLeft(value, "0")
	if value == "" {
		return "0"
	}
	return value
}
