// Copyright 2021 The Project Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// create_release is a tool to create a release manifest from a GitHub release.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/golang/glog"
	"github.com/google/go-github/v35/github"
	"golang.org/x/oauth2"
)

// TokenENV is the name of an environment variable to check for a github personal auth token.
// If you're hitting GitHub API rate limits, setting this will raise the limits.
const TokenENV = "GITHUB_TOKEN"

var (
	repo             = flag.String("repo", "f-secure-foundry/armory-drive", "GitHub repo to search for releases")
	tag              = flag.String("tag", "", "Release tag to fetch, if unset, uses latest release")
	includeArtifacts = flag.String("include_artifacts", `armory-drive\.[[:alnum:]]*$`, "Regex to specify release artifacts to include in manifest")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	owner, repo, err := splitRepoFlag(*repo)
	if err != nil {
		glog.Exitf("Couldn't parse repo: %q", err)
	}

	c := github.NewClient(getHTTPClient(ctx))

	r, err := getRelease(ctx, c, owner, repo, *tag)
	if err != nil {
		glog.Exitf("Failed to fetch releases: %q", err)
	}

	glog.Info("Created FirmwareRelease struct:")

	// Write struct to stdout in case we're being piped.
	pp, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(pp))
}

// getRelease uses the GitHub API to retrieve information about the tagged release, and uses it
// to populate a FirmwareRelease struct.
func getRelease(ctx context.Context, c *github.Client, owner, repo, tag string) (api.FirmwareRelease, error) {
	artifactMatcher, err := regexp.Compile(*includeArtifacts)
	if err != nil {
		return api.FirmwareRelease{}, fmt.Errorf("invalid regex passed to --include_artifacts: %q", err)
	}

	glog.Info("Fetching release info...")
	// First grab the release info from GitHub
	var rel *github.RepositoryRelease
	if tag == "" {
		rel, _, err = c.Repositories.GetLatestRelease(ctx, owner, repo)
		if err != nil {
			return api.FirmwareRelease{}, fmt.Errorf("failed to get latest release: %q", err)
		}
	} else {
		rel, _, err = c.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
		if err != nil {
			return api.FirmwareRelease{}, fmt.Errorf("failed to get release with tag %q: %q", tag, err)
		}
	}
	if glog.V(1) {
		pp, _ := json.MarshalIndent(rel, "", "  ")
		glog.V(1).Infof("Found release:\n%s", pp)
	}

	// Hash the release's source tarball
	glog.Info("Fetching and hashing source tarball...")
	sourceURL := *rel.TarballURL
	sourceHash, err := hashRemote(sourceURL)
	if err != nil {
		return api.FirmwareRelease{}, fmt.Errorf("failed to hash release source: %q", err)
	}

	glog.Info("Identifying commit hash associated with release...")
	releaseCommitSHA, err := fetchReleaseCommit(ctx, c, owner, repo, *rel.TagName)

	// Finally, build the FirmwareRelease structure
	fr := api.FirmwareRelease{
		Description: *rel.Name,
		// TODO(al): This needs to be in the release data somewhere
		PlatformID:   "<unset>",
		Revision:     *rel.TagName,
		SourceURL:    sourceURL,
		SourceSHA256: sourceHash,
		// TODO(al): This needs to be in the release data somewhere
		ToolChain: "tamago1.16.3",
		BuildArgs: map[string]string{
			"REV": releaseCommitSHA,
		},
		ArtifactSHA256: make(map[string][]byte),
	}

	glog.Info("Hashing release artifacts...")
	for _, a := range rel.Assets {
		if !artifactMatcher.MatchString(*a.Name) {
			glog.V(1).Infof("Ignoring artifact %q", *a.Name)
			continue
		}
		h, err := hashRemote(*a.BrowserDownloadURL)
		if err != nil {
			return api.FirmwareRelease{}, fmt.Errorf("failed to hash asset at %q: %q", *a.BrowserDownloadURL, err)
		}
		fr.ArtifactSHA256[*a.Name] = h

	}
	return fr, err
}

// splitRepoFlag separates a short owner/repo string into its consistuent parts.
func splitRepoFlag(repo string) (string, string, error) {
	if repo == "" {
		return "", "", errors.New("--repo must not be empty")
	}
	r := strings.Split(repo, "/")
	if len(r) != 2 {
		return "", "", errors.New("--repo should be of the form 'owner/repo'")
	}
	return r[0], r[1], nil
}

// fetchReleaseCommit returns the git SHA associated with the provided tag.
func fetchReleaseCommit(ctx context.Context, c *github.Client, owner, repo, tag string) (string, error) {
	// Look up the commit hash associated with the release tag
	tagRef := fmt.Sprintf("tags/%s", tag)
	ref, _, err := c.Git.GetRef(ctx, owner, repo, tagRef)
	if err != nil {
		return "", fmt.Errorf("failed to look up ref %q: %q ", tagRef, err)
	}
	relTag, _, err := c.Git.GetTag(ctx, owner, repo, *ref.Object.SHA)
	if err != nil {
		return "", fmt.Errorf("failed to look up tag SHA %q: %q ", *ref.Object.SHA, err)
	}
	if glog.V(1) {
		pp, _ := json.MarshalIndent(relTag, "", "  ")
		glog.V(1).Infof("Found ref:\n%s", pp)
	}
	return *relTag.Object.SHA, nil
}

// hashRemote returns the SHA256 of the contents of the resource pointed to by url.
func hashRemote(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %q: %q", url, err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got non-200 HTTP status when fetching %q: %s", url, resp.Status)
	}
	defer resp.Body.Close()

	h := sha256.New()
	if _, err := io.Copy(h, resp.Body); err != nil {
		return nil, fmt.Errorf("failed to hash content: %q", err)
	}
	return h.Sum(nil), nil
}

// getHTTPClient returns a client for accessing the github API.
//
// If the GITHUB_TOKEN env variable is set, then this client will use its contents
// as an OAUTH token for all requests to the github API (this greatly increases
// the API rate limits.
func getHTTPClient(ctx context.Context) *http.Client {
	tok := os.Getenv(TokenENV)
	if tok != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})
		return oauth2.NewClient(ctx, ts)
	}
	return http.DefaultClient
}
