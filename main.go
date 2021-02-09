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
	repo  = flag.String("github-repo", "nathanchance/WSL2-Linux-Kernel", "WSL2 kernel source repository")
	store = flag.String("dir", "wsl2", "directory used for saving kernel images")
	tag   = flag.Bool("tag", false, "auto-tag downloaded kernel using release.tag_name JSON property")
	/*
		imageName = flag.String("image", kernelImage, "name of kernel image")
		wslconfig = flag.String("config", "<user-home>/.wslconfig", "WSL2 configuration file")
		autoInstall = flag.Bool("install", false, "auto-install kernel to WSL2 (requires wsk reboot)")
	*/
)

func main() {
	flag.Parse()

	u, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to retrieve current user: %v", err)
		return
	}

	storedImage := path.Join(u.HomeDir, *store, kernelImageName)
	localDigest, err := sha1sum(storedImage)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("current kernel image", storedImage, "sha:", localDigest)

	kernel, remoteDigest, err := downloadReleasedImage()
	if err != nil {
		log.Fatal(err)
	}

	if remoteDigest != localDigest {
		_, fn := path.Split(kernel)
		storedImage = path.Join(u.HomeDir, *store, fn) // overwrite the existing kernel?
		log.Println("copying new kernel from", kernel, "to", storedImage)
		err := os.Rename(kernel, storedImage)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("latest release already in", storedImage)
	}
}

func downloadReleasedImage() (string, string, error) {
	downloadedKernel := path.Join(os.TempDir(), kernelImageName)
	empty := fmt.Sprintf("%x", sha1.New().Sum(nil))
	link := "https://api.github.com/repos/" + *repo + "/releases/latest"
	client := http.Client{Timeout: 5 * time.Second}
	r, err := client.Get(link)
	if err != nil {
		return "", empty, fmt.Errorf("failed to retrieve release %s: %w", link, err)
	}

	release := GithubRelease{}
	err = json.NewDecoder(r.Body).Decode(&release)
	r.Body.Close()
	if err != nil {
		return "", empty, fmt.Errorf("failed to parse release %s: %w", link, err)
	}

	log.Printf("Latest release in %s tagged %s, published %v.\n",
		link, release.TagName, release.PublishedAt)
	log.Println("Description")
	log.Println(release.Body)
	if release.Draft || release.Prerelease {
		return "", empty, fmt.Errorf("release %s marked draft/prerelease. Aborting", link)
	}

	for i := 0; i < len(release.Assets); i++ {
		if release.Assets[i].Name == kernelImageName {
			r, err := client.Get(release.Assets[i].BrowserDownloadURL)
			if err != nil {
				return "", empty, fmt.Errorf("failed to download %s: %w",
					release.Assets[i].BrowserDownloadURL, err)
			}
			defer r.Body.Close()

			if *tag {
				downloadedKernel = fmt.Sprintf("%s.%s", downloadedKernel, release.TagName)
			}
			downloadedKernel = path.Clean(downloadedKernel)

			out, err := os.Create(downloadedKernel)
			_, err = io.Copy(out, r.Body)
			if err != nil {
				return "", empty, fmt.Errorf("failed to save %s to %s: %w",
					release.Assets[i].BrowserDownloadURL, downloadedKernel, err)
			}
			out.Close()

			digest, err := sha1sum(downloadedKernel)
			return downloadedKernel, digest, err
		}
	}
	return "", empty, fmt.Errorf("unable to find asset %s in %s", kernelImageName, link)
}

func sha1sum(fn string) (string, error) {
	if _, err := os.Stat(fn); os.IsNotExist(err) {
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
