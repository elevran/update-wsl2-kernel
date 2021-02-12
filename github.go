package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-github/v33/github"
)

// could have wrapped the github interaction in its own struct (e.g., bound to a specific
// owner and repository name on creation), but a global variable would do fine here...
var gh = github.NewClient(nil)

// list recent releases in repository, printing out release tag, publish date and status
func listReleases(ctx context.Context, repository string) error {
	owner, repo, err := ghOwnerAndRepo(repository)
	if err != nil {
		return err
	}

	releases, _, err := gh.Repositories.ListReleases(ctx, owner, repo, nil)
	if err != nil {
		return fmt.Errorf("Repositories.ListReleases returned error: %w", err)
	}

	fmt.Println("listing recent releases from", repository)
	for _, release := range releases {
		fmt.Printf("release %s published %v (draft/pre-release: %t)\n",
			*release.TagName, release.PublishedAt.Format("2006-01-02"),
			*release.Draft || *release.Prerelease)
	}
	return nil
}

// get release asset by name from specified release tag
// Caller is responsible forclosing the returned io.ReadCloser
func getReleaseAsset(ctx context.Context, repository, tag, filename string) (io.ReadCloser, string, error) {
	owner, repo, err := ghOwnerAndRepo(repository)
	if err != nil {
		return nil, "", err
	}

	ghRelease := &github.RepositoryRelease{}
	ghReleaseTag := tag

	if tag == "" || tag == "latest" {
		tag = "latest"
		ghRelease, _, err = gh.Repositories.GetLatestRelease(ctx, owner, repo)
	} else {
		ghRelease, _, err = gh.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	}

	if err != nil {
		return nil, "", err
	}

	id := int64(0)
	if ghReleaseTag == "" {
		ghReleaseTag = *ghRelease.TagName
	}

	for _, ra := range ghRelease.Assets {
		if *ra.Name == filename {
			id = *ra.ID
			break
		}
	}
	if id == int64(0) {
		return nil, "", fmt.Errorf("asset %s not found in release %s tagged %s", filename, repository, tag)
	}
	rc, _, err := gh.Repositories.DownloadReleaseAsset(ctx, owner, repo, id, http.DefaultClient)
	return rc, ghReleaseTag, err
}

// split the combined repository string into owner and repository name
func ghOwnerAndRepo(repository string) (string, string, error) {
	components := strings.Split(repository, "/")
	if len(components) != 2 {
		return "", "", errors.New("unexpected repository format, should be <user>/<repo>")
	}
	return components[0], components[1], nil
}
