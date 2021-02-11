package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

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
