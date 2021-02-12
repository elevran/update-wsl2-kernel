package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
)

var (
	repository  = flag.String("github-repo", "nathanchance/WSL2-Linux-Kernel", "WSL2 kernel source repository on github")
	downloads   = flag.String("dir", "", "directory used for downloaded kernel image, overrides .wslconfig value if defined")
	imageName   = flag.String("image-name", "bzImage", "kernel image name in release")
	byTag       = flag.String("tag", "", "download a specific release based on its tag, instead of 'latest'")
	tagImage    = flag.Bool("tag-image", true, "use 'release.tag_name' as image file extension")
	autoInstall = flag.Bool("install", false, "auto-install kernel to WSL2 -- requires WSL reboot!")
	listOnly    = flag.Bool("list", false, "list recent releases, without downloading anything")
)

func main() {
	flag.Parse()

	if *listOnly {
		fmt.Println("available releases:")
		ctx := context.Background()
		if err := listReleases(ctx, *repository); err != nil {
			exit(err)
		}
		return
	}

	local, err := wslConfigGetKernelPath()
	if err != nil && !os.IsNotExist(err) {
		exit(err)
	}

	if *downloads == "" { // target download directory is not set
		if local != "" {
			*downloads = path.Dir(local)
		} else { // not set and not defined in wslconfig, use default directory '~/wsl2-kernels'
			const defaultKernelDir = "wsl2-kernels"
			home, err := userHomeDirectory()
			if err != nil {
				exit(err)
			}
			*downloads = path.Join(home, defaultKernelDir)
			if _, err = os.Stat(*downloads); os.IsNotExist(err) {
				fmt.Println("creating download directory for kernel images:", *downloads)
				if err = os.Mkdir(*downloads, 0755); err != nil {
					exit(err)
				}
			}
		}
	}

	localSHA := emptySHA1
	if local != "" {
		localSHA, err = sha1sum(local)
		fmt.Println("local kernel", local, "digest:", localSHA)
		if err != nil {
			exit(err)
		}
	}

	fmt.Println("downloading remote image from", *repository)
	copy, remoteSHA, remoteTag, err := downloadCopyOfReleasedImage()
	if err != nil {
		exit(err)
	}

	fmt.Println("remote kernel tagged", remoteTag, "digest:", remoteSHA)
	if remoteSHA != localSHA {
		destination := path.Join(*downloads, *imageName)
		if *tagImage {
			destination = fmt.Sprintf("%s.%s", destination, remoteTag)
		}
		destination = path.Clean(destination)

		fmt.Println("digests differ, copying new kernel to", destination)
		if err = os.Rename(copy, destination); err != nil {
			exit(err)
		}
		if *autoInstall {
			err = wslConfigSetKernel(destination)
			if err != nil {
				exit(err)
			}
			fmt.Println("WSL configured to use new kernel --- requires a reboot")
		}
	} else {
		fmt.Println("latest release already in", *downloads)
	}
}

func exit(err error) {
	fmt.Println(err)
	os.Exit(1)
}

// download a released image, returns the local copy path, release tag name and SHA1 digest
func downloadCopyOfReleasedImage() (string, string, string, error) {
	destination := path.Join(os.TempDir(), *imageName)
	ctx := context.Background()
	rc, releaseTag, err := getReleaseAsset(ctx, *repository, *byTag, *imageName)
	defer rc.Close()

	out, err := os.Create(destination)
	if err != nil {
		return "", "", "", err
	}
	_, err = io.Copy(out, rc)
	out.Close()

	if err != nil {
		fmt.Println("unable to save downloaded image")
		return "", "", "", err
	}

	digest, err := sha1sum(destination)
	return destination, digest, releaseTag, err
}
