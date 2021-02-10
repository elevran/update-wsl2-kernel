package main

import (
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"
	"time"
)

type (
	// GithubRelease defines Github release object, abbreviated to include relevant fields only.
	// See https://docs.github.com/en/rest/reference/repos#releases for full description.
	GithubRelease struct {
		Name        string    `json:"name"`
		TagName     string    `json:"tag_name"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		CreatedAt   time.Time `json:"created_at"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			URL                string `json:"url"`
			Name               string `json:"name"`
			Label              string `json:"label"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
		Body string `json:"body"`
	}
)

const (
	kernelImageName = "bzImage"
)

var (
	emptySHA1 = fmt.Sprintf("%x", sha1.New().Sum(nil))

	repo        = flag.String("github-repo", "nathanchance/WSL2-Linux-Kernel", "WSL2 kernel source repository")
	kernelsDir  = flag.String("kernel-dir", "wsl2", "directory used for saving kernel images")
	tag         = flag.Bool("tag", true, "auto-tag downloaded kernel using release.tag_name JSON property")
	autoInstall = flag.Bool("install", false, "auto-install kernel to WSL2 (requires wsl reboot)")
	// add an option to list releases for the repo? 'GET /repos/{owner}/{repo}/releases'
)

func main() {
	flag.Parse()

	home, err := userHomeDirectory()
	// wsl2Kernel, err := parseWSLConfig()

	storedImage := path.Join(home, *kernelsDir, kernelImageName)
	localDigest, err := sha1sum(storedImage)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("current kernel image", storedImage, "sha:", localDigest)

	link := "https://api.github.com/repos/" + *repo + "/releases/latest"
	kernel, remoteDigest, err := downloadReleasedImage(link)
	if err != nil {
		log.Fatal(err)
	}

	if remoteDigest != localDigest {
		_, fn := path.Split(kernel)
		// if auto-install, update the .wslconfig file
		storedImage = path.Join(home, *kernelsDir, fn) // overwrite the existing kernel?
		log.Println("copying new kernel from", kernel, "to", storedImage)
		err := os.Rename(kernel, storedImage)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("latest release already in", storedImage)
	}
}

// download the kernel image from the given release into a temporary location.
// Returns the downloaded image path, its SHA1 digest and any error that may have occurred.
func downloadReleasedImage(link string) (string, string, error) {
	downloadedKernel := path.Join(os.TempDir(), kernelImageName)
	client := http.Client{Timeout: 5 * time.Second}
	r, err := client.Get(link)
	if err != nil {
		return "", emptySHA1, fmt.Errorf("failed to retrieve release %s: %w", link, err)
	}

	release := GithubRelease{}
	err = json.NewDecoder(r.Body).Decode(&release)
	r.Body.Close()
	if err != nil {
		return "", emptySHA1, fmt.Errorf("failed to parse release %s: %w", link, err)
	}

	log.Printf("Latest release in %s tagged %s, published %v.\n",
		link, release.TagName, release.PublishedAt)
	log.Println("Description")
	log.Println(release.Body)
	if release.Draft || release.Prerelease {
		return "", emptySHA1, fmt.Errorf("release %s marked draft/prerelease. Aborting", link)
	}

	for i := 0; i < len(release.Assets); i++ {
		if release.Assets[i].Name == kernelImageName {
			if *tag {
				downloadedKernel = fmt.Sprintf("%s.%s", downloadedKernel, release.TagName)
			}
			downloadedKernel = path.Clean(downloadedKernel)

			err := downloadKernelImage(release.Assets[i].BrowserDownloadURL, downloadedKernel)
			digest, err := sha1sum(downloadedKernel)
			return downloadedKernel, digest, err
		}
	}
	return "", emptySHA1, fmt.Errorf("unable to find asset %s in %s", kernelImageName, link)
}

// download kernel image from URL to local path, returns any error
func downloadKernelImage(assetURL, localPath string) error {
	client := http.Client{Timeout: 5 * time.Second}
	r, err := client.Get(assetURL)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", assetURL, err)
	}
	defer r.Body.Close()

	out, err := os.Create(localPath)
	_, err = io.Copy(out, r.Body)
	out.Close()
	if err != nil {
		return fmt.Errorf("failed to save %s to %s: %w", assetURL, localPath, err)
	}
	return nil
}

// return the SHA1 digest for the named file
func sha1sum(fn string) (string, error) {
	if _, err := os.Stat(fn); err != nil {
		return fmt.Sprintf("%x", sha1.New().Sum(nil)), fmt.Errorf("failed to stat %s: %w", fn, err)
	}

	file, err := os.Open(fn)
	if err != nil {
		return fmt.Sprintf("%x", sha1.New().Sum(nil)), fmt.Errorf("failed to open %s: %w", fn, err)
	}
	defer file.Close()

	h := sha1.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return fmt.Sprintf("%x", sha1.New().Sum(nil)), fmt.Errorf("failed to checksum %s: %w", fn, err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// returns the default location of WSL configuration file
func wslConfigPath() (string, error) {
	home, err := userHomeDirectory()
	if err != nil {
		return "", err
	}
	return path.Join(home, ".wslconfig"), nil
}

// returns the home directory for the current user
func userHomeDirectory() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return u.HomeDir, nil
}

// wsl2Kernel, err := parseWSLConfig()

// parse <home>/.wslconfig if exists - use INI package
// determine kernel dir (if not set via a flag). Use default if neither is defined
// if there is a kernel key in the wsl2 section, use that for 'current'
