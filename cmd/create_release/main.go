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

// create_release is a tool to create a release manifest for a firmare release.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/golang/glog"
	"golang.org/x/mod/sumdb/note"
)

var (
	repo           = flag.String("repo", "f-secure-foundry/armory-drive", "GitHub repo where release will be uploaded")
	description    = flag.String("description", "", "Release description")
	platformID     = flag.String("platform_id", "", "Specifies the plaform ID that this release is targetting")
	commitHash     = flag.String("commit_hash", "", "Speficies the github commit hash that the release was built from")
	toolChain      = flag.String("tool_chain", "", "Specifies the toolchain used to build the release")
	artifacts      = flag.String("artifacts", `armory-drive.*`, "Space separated list of globs specifying the release artifacts to include")
	revisionTag    = flag.String("revision_tag", "", "The git tag name which identifies the firmware revision")
	privateKeyFile = flag.String("private_key", "", "Path to file containing the private key used to sign the manifest")
)

func main() {
	flag.Parse()
	if err := validateFlags(); err != nil {
		glog.Exitf("Invalid flag(s):\n%s", err)
	}

	sourceURL := fmt.Sprintf("https://github.com/%s/tarball/%s", *repo, *revisionTag)
	sourceHash, err := hashRemote(sourceURL)
	if err != nil {
		glog.Exitf("Failed to hash source tarball (%s): %q", sourceURL, err)
	}

	fr := api.FirmwareRelease{
		Description:  *description,
		PlatformID:   *platformID,
		Revision:     *revisionTag,
		SourceURL:    sourceURL,
		SourceSHA256: sourceHash,
		ToolChain:    *toolChain,
		BuildArgs: map[string]string{
			"REV": *commitHash,
		},
	}

	// TODO(al): consider using the GH API to check that revisionTag resolves to commitHash and
	// warn if that's not the case.

	glog.Info("Hashing release artifacts...")
	artifacts, err := hashArtifacts()
	if err != nil {
		glog.Exitf("Failed to hash artifacts: %q", err)
	}
	fr.ArtifactSHA256 = artifacts

	pp, err := json.MarshalIndent(fr, "", "  ")
	if err != nil {
		glog.Exitf("Failed to marshal FirmwareRelease: %q", err)
	}
	s, err := sign(string(pp))
	if err != nil {
		glog.Exitf("Failed to sign FirmwareRelease JSON: %q", err)
	}
	// Write struct to stdout in case we're being piped.
	fmt.Println(string(s))
}

// sign signs the passed in body using the Go sumdb's note format.
func sign(body string) ([]byte, error) {
	// Note body must end in a trailing new line, so add one if necessary.
	if !strings.HasSuffix(body, "\n") {
		body = fmt.Sprintf("%s\n", body)
	}

	k, err := os.ReadFile(*privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %q", err)
	}
	signer, err := note.NewSigner(string(k))
	if err != nil {
		return nil, fmt.Errorf("failed to initialise key: %q", err)
	}

	return note.Sign(&note.Note{Text: body}, signer)
}

func validateFlags() error {
	errs := make([]string, 0)
	checkEmpty := func(n, s string) {
		if s == "" {
			errs = append(errs, fmt.Sprintf("--%s can't be empty", n))
		}
	}
	checkEmpty("repo", *repo)
	checkEmpty("description", *description)
	checkEmpty("platform_id", *platformID)
	checkEmpty("commit_hash", *commitHash)
	checkEmpty("tool_chain", *toolChain)
	checkEmpty("artifacts", *artifacts)
	checkEmpty("revision_tag", *revisionTag)

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func hashArtifacts() (map[string][]byte, error) {
	r := make(map[string][]byte)
	for _, glob := range strings.Split(*artifacts, " ") {
		match, err := filepath.Glob(glob)
		if err != nil {
			return nil, err
		}
		for _, f := range match {
			h, err := hashFile(f)
			if err != nil {
				return nil, err
			}

			_, name := filepath.Split(f)
			r[name] = h
		}
	}
	return r, nil
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
	return hash(resp.Body)
}

func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return hash(f)
}

func hash(r io.Reader) ([]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, fmt.Errorf("failed to hash content: %q", err)
	}
	return h.Sum(nil), nil
}
