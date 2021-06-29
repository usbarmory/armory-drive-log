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

// create_proofbundle is a tool to create and serialise a ProofBundle structure for
// use in an OTA zip.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/golang/glog"
	"github.com/google/trillian-examples/serverless/client"
	"github.com/google/trillian/merkle/logverifier"
	"github.com/google/trillian/merkle/rfc6962/hasher"
	"golang.org/x/mod/sumdb/note"
)

var (
	release    = flag.String("release", "armory-drive.release", "Path to release metadata file")
	logURL     = flag.String("log_url", "https://raw.githubusercontent.com/f-secure-foundry/armory-drive-log/master/log/", "URL identifying the location of the log")
	logPubKey  = flag.String("log_pubkey", "armory-drive-log-test+a5aae457+AbDoiIsZgSk5H0v0LjKPKv5dAMb0IfB47tocFtGmyW44", "The log's public key")
	outputFile = flag.String("output", "", "Path to write output file to, leave unset to write to stdout")
)

func main() {
	flag.Parse()

	if err := checkFlags(); err != nil {
		glog.Exitf("Invalid flags:\n%s", err)
	}

	lSigV, err := note.NewVerifier(*logPubKey)
	if err != nil {
		glog.Exitf("Unable to create new log signature verifier: %v", err)
	}

	releaseRaw, err := os.ReadFile(*release)
	if err != nil {
		glog.Exitf("Failed to read release file %q: %v", *release, err)
	}

	bundle, err := createBundle(*logURL, releaseRaw, lSigV)
	if err != nil {
		glog.Exitf("Failed to create ProofBundle: %v", err)
	}
	bundleRaw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		glog.Exitf("Failed to marshal ProofBundle: %v", err)
	}

	if *outputFile == "" {
		fmt.Println(string(bundleRaw))
	} else {
		if err := os.WriteFile(*outputFile, bundleRaw, 0644); err != nil {
			glog.Exitf("Failed to write to output file %q: %v", *outputFile, err)
		}
		glog.Infof("Wrote proof bundle to %q", *outputFile)
	}
}

func createBundle(logURL string, release []byte, lSigV note.Verifier) (*api.ProofBundle, error) {
	root, err := url.Parse(logURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse log URL %q: %v", logURL, err)
	}
	f, err := newFetcher(root)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetcher: %v", err)
	}

	h := hasher.DefaultHasher

	st, err := client.NewLogStateTracker(f, h, nil, lSigV)
	if err != nil {
		return nil, fmt.Errorf("failed to create new LogStateTracker: %v", err)
	}

	leafHash := h.HashLeaf(release)
	lv := logverifier.New(h)
	// Wait for inclusion
	for {
		if err := st.Update(); err != nil {
			return nil, fmt.Errorf("failed to update LogState: %v", err)
		}
		cp := st.LatestConsistent

		idx, err := client.LookupIndex(f, leafHash)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("failed to look up leaf index: %v", err)
			}
			glog.Infof("Leaf not [yet] sequenced, retrying")
			time.Sleep(5 * time.Second)
			continue
		}

		pb, err := client.NewProofBuilder(cp, h.HashChildren, f)
		if err != nil {
			return nil, fmt.Errorf("failed to create new ProofBuilder: %v", err)
		}

		ip, err := pb.InclusionProof(idx)
		if err != nil {
			return nil, fmt.Errorf("failed to create inclusion proof for leaf %d: %v", idx, err)
		}
		if err := lv.VerifyInclusionProof(int64(idx), int64(cp.Size), ip, cp.Hash, leafHash); err != nil {
			return nil, fmt.Errorf("failed to verify inclusion proof: %q", err)
		}
		glog.Infof("Found leaf at %d", idx)
		break
	}

	allLeafHashes, err := client.FetchLeafHashes(f, 0, st.LatestConsistent.Size, st.LatestConsistent.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch leaf hashes [0, %d): %v", st.LatestConsistent.Size, err)
	}

	return &api.ProofBundle{
		NewCheckpoint:   st.LatestConsistentRaw,
		FirmwareRelease: release,
		LeafHashes:      allLeafHashes,
	}, nil
}

func checkFlags() error {
	errs := make([]string, 0)
	checkNotEmpty := func(name, value string) {
		if value == "" {
			errs = append(errs, fmt.Sprintf("--%s must not be empty", name))
		}
	}
	checkNotEmpty("release", *release)
	checkNotEmpty("log_url", *logURL)
	checkNotEmpty("log_pubkey", *logPubKey)

	if !strings.HasSuffix(*logURL, "/") {
		errs = append(errs, "--log_url must end with a '/'")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

// newFetcher creates a Fetcher for the log at the given root location.
func newFetcher(root *url.URL) (client.Fetcher, error) {
	get := getByScheme[root.Scheme]
	if get == nil {
		return nil, fmt.Errorf("unsupported URL scheme %s", root.Scheme)
	}

	f := func(p string) ([]byte, error) {
		u, err := root.Parse(p)
		if err != nil {
			return nil, err
		}
		return get(u)
	}
	return f, nil
}

var getByScheme = map[string]func(*url.URL) ([]byte, error){
	"http":  readHTTP,
	"https": readHTTP,
	"file": func(u *url.URL) ([]byte, error) {
		return ioutil.ReadFile(u.Path)
	},
}

func readHTTP(u *url.URL) ([]byte, error) {
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
