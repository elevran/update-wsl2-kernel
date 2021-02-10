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

	// GithubReleaseList is an array of GitHubRelease objects
	GithubReleaseList []GithubRelease
)

const (
	kernelImageName = "bzImage"
	ghReposEndpoint = "https://api.github.com/repos/"
)

var (
	emptySHA1 = fmt.Sprintf("%x", sha1.New().Sum(nil))

	repo = flag.String("github-repo", "nathanchance/WSL2-Linux-Kernel",
		"WSL2 kernel source repository on github")
	kernelsDir = flag.String("kernels-dir", "wsl2-kernels",
		"directory used for downloaded kernel image, overrides .wslconfig value when defined")
	//tagged      = flag.String("-tag", "", "download a specific release based on its tag, instead of 'latest'")
	autoTag     = flag.Bool("tag-image", true, "use 'release.tag_name' as image file extension")
	autoInstall = flag.Bool("install", false, "auto-install kernel to WSL2 -- requires WSL reboot!")
	listOnly    = flag.Bool("list", false, "list recent releases, without downloading anything")
)

func main() {
	flag.Parse()

	if *listOnly {
		if err := listRecentReleases(); err != nil {
			log.Fatal(err)
		}
		return
	}

	home, err := userHomeDirectory()
	// wsl2Kernel, err := parseWSLConfig()

	storedImage := path.Join(home, *kernelsDir, kernelImageName)
	localDigest, err := sha1sum(storedImage)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("current kernel image", storedImage, "sha:", localDigest)

	// split the bzImage download (downloadKernelFromURL)
	// from the link calculation (getLatestReleaseKernelURL)
	// download a specific kernel if *tagged != nil
	link := ghReposEndpoint + *repo + "/releases/latest"
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

// @TODO: split the GH release and their access to a separate file, same for wslconfig handling

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

	log.Printf("Latest release in %s is %s, published %s (draft/prerelease:%t).\n",
		*repo, release.TagName, release.PublishedAt.Format("2006-01-02"),
		release.Draft || release.Prerelease)
	log.Println("Description")
	log.Println(release.Body)

	for i := 0; i < len(release.Assets); i++ {
		if release.Assets[i].Name == kernelImageName {
			if *autoTag {
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
		return emptySHA1, fmt.Errorf("failed to stat %s: %w", fn, err)
	}

	file, err := os.Open(fn)
	if err != nil {
		return emptySHA1, fmt.Errorf("failed to open %s: %w", fn, err)
	}
	defer file.Close()

	h := sha1.New()
	_, err = io.Copy(h, file)
	if err != nil {
		return emptySHA1, fmt.Errorf("failed to checksum %s: %w", fn, err)
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

// list recent releases and their status
func listRecentReleases() error {
	releases := ghReposEndpoint + *repo + "/releases"
	client := http.Client{Timeout: 5 * time.Second}
	r, err := client.Get(releases)

	if err != nil {
		return err
	}

	recent := GithubReleaseList{}
	err = json.NewDecoder(r.Body).Decode(&recent)
	r.Body.Close()
	if err != nil {
		return err
	}
	log.Println("listing releases from", *repo)
	for i := 0; i < len(recent); i++ {
		log.Printf("release %s published %v (draft/pre-release:%t)",
			recent[i].TagName, recent[i].PublishedAt.Format("2006-01-02"),
			recent[i].Draft || recent[i].Prerelease)
	}
	return nil
}

// wsl2Kernel, err := parseWSLConfig()

// parse <home>/.wslconfig if exists - use INI package
// determine kernel dir (if not set via a flag). Use default if neither is defined
// if there is a kernel key in the wsl2 section, use that for 'current' digest
