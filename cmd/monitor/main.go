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

// monitor starts a long-running process that will continually follow a log
// for new checkpoints. All checkpoints are checked for consistency, and all
// leaves in the tree will be downloaded, verified, and the release info
// will be reproducibly verified.
// This tool has a number of expectations of the environment, such as a working
// tamago installation, git, and other make tooling. See the README and Dockerfile
// in this directory for more details.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/f-secure-foundry/armory-drive-log/keys"
	"github.com/golang/glog"
	"github.com/google/trillian-examples/serverless/client"
	"github.com/google/trillian/merkle/rfc6962"
	"golang.org/x/mod/sumdb/note"
)

var (
	pollInterval  = flag.Duration("poll_interval", 1*time.Minute, "The interval at which the log will be polled for new data")
	stateFile     = flag.String("state_file", "", "File path for where checkpoints should be stored")
	logURL        = flag.String("log_url", "https://raw.githubusercontent.com/f-secure-foundry/armory-drive-log/master/log/", "URL identifying the location of the log")
	logPubKey     = flag.String("log_pubkey", keys.ArmoryDriveLogPub, "The log's public key")
	logOrigin     = flag.String("log_origin", "Armory Drive Prod 2", "The expected first line of checkpoints issued by the log")
	releasePubKey = flag.String("release_pubkey", keys.ArmoryDrivePub, "The release signer's public key")
	cleanup       = flag.Bool("cleanup", true, "Set to false to keep git checkouts and make artifacts around after verification")
)

func main() {
	flag.Parse()
	ctx := context.Background()

	st, isNew, err := stateTrackerFromFlags(ctx)
	if err != nil {
		glog.Exitf("Failed to create new LogStateTracker: %v", err)
	}

	var releaseVerifiers note.Verifiers
	if v, err := note.NewVerifier(*releasePubKey); err != nil {
		glog.Exitf("Failed to construct release note verifier: %v", err)
	} else {
		releaseVerifiers = note.VerifierList(v)
	}

	rbv, err := NewReproducibleBuildVerifier(*cleanup)
	if err != nil {
		glog.Exitf("Failed to create reproducible build verifier: %v", err)
	}

	monitor := Monitor{
		st:               st,
		stateFile:        *stateFile,
		releaseVerifiers: releaseVerifiers,
		handler:          rbv.VerifyManifest,
	}

	if isNew {
		// This monitor has no memory of running before, so let's catch up with the log.
		if err := monitor.From(ctx, 0); err != nil {
			glog.Exitf("monitor.From(%d): %v", 0, err)
		}
	}

	// We've processed all leaves committed to by the tracker's checkpoint, and now we enter polling mode.
	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()
	for {
		lastHead := st.LatestConsistent.Size
		if err := st.Update(ctx); err != nil {
			glog.Exitf("Failed to update checkpoint: %q", err)
		}
		if st.LatestConsistent.Size > lastHead {
			glog.V(1).Infof("Found new checkpoint for tree size %d, fetching new leaves", st.LatestConsistent.Size)
			if err := monitor.From(ctx, lastHead); err != nil {
				glog.Exitf("monitor.From(%d): %v", lastHead, err)
			}
		} else {
			glog.V(2).Infof("Polling: no new data found; tree size is still %d", st.LatestConsistent.Size)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Go around the loop again.
		}
	}
}

// Monitor verifiably checks inclusion of all leaves in a range, and then passes the
// parsed FirmwareRelease to a handler.
type Monitor struct {
	st               client.LogStateTracker
	stateFile        string
	releaseVerifiers note.Verifiers
	handler          func(context.Context, uint64, api.FirmwareRelease) error
}

// From checks the leaves from `start` up to the checkpoint from the state tracker.
// Upon reaching the end of the leaves, the checkpoint is persisted in the state file.
func (m *Monitor) From(ctx context.Context, start uint64) error {
	fromCP := m.st.LatestConsistent
	pb, err := client.NewProofBuilder(ctx, fromCP, m.st.Hasher.HashChildren, m.st.Fetcher)
	if err != nil {
		return fmt.Errorf("failed to construct proof builder: %v", err)
	}
	for i := start; i < fromCP.Size; i++ {
		rawLeaf, err := client.GetLeaf(ctx, m.st.Fetcher, i)
		if err != nil {
			return fmt.Errorf("failed to get leaf at index %d: %v", i, err)
		}
		hash := m.st.Hasher.HashLeaf(rawLeaf)
		ip, err := pb.InclusionProof(ctx, i)
		if err != nil {
			return fmt.Errorf("failed to get inclusion proof for index %d: %v", i, err)
		}

		if err := m.st.Verifier.VerifyInclusionProof(int64(i), int64(fromCP.Size), ip, fromCP.Hash, hash); err != nil {
			return fmt.Errorf("VerifyInclusionProof() %d: %v", i, err)
		}

		releaseNote, err := note.Open([]byte(rawLeaf), m.releaseVerifiers)
		if err != nil {
			if e, ok := err.(*note.UnverifiedNoteError); ok && len(e.Note.UnverifiedSigs) > 0 {
				return fmt.Errorf("unknown signer %q for leaf at index %d: %v", e.Note.UnverifiedSigs[0].Name, i, err)
			}
			return fmt.Errorf("failed to open leaf note at index %d: %v", i, err)
		}

		var release api.FirmwareRelease
		if err := json.Unmarshal([]byte(releaseNote.Text), &release); err != nil {
			return fmt.Errorf("failed to unmarshal release at index %d: %w", i, err)
		}
		if err := m.handler(ctx, i, release); err != nil {
			return fmt.Errorf("handler(): %w", err)
		}
	}
	return ioutil.WriteFile(m.stateFile, m.st.LatestConsistentRaw, 0644)
}

// stateTrackerFromFlags constructs a state tracker based on the flags provided to the main invocation.
// The checkpoint returned will be the checkpoint representing this monitor's view of the log history.
// A boolean is returned that is true iff the checkpoint was fetched from the log to initialize state.
func stateTrackerFromFlags(ctx context.Context) (client.LogStateTracker, bool, error) {
	if len(*stateFile) == 0 {
		return client.LogStateTracker{}, false, errors.New("--state_file required")
	}

	state, err := ioutil.ReadFile(*stateFile)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return client.LogStateTracker{}, false, fmt.Errorf("could not read state file %q: %w", *stateFile, err)
		}
		glog.Infof("State file %q missing. Will trust first checkpoint received from log.", *stateFile)
	}

	root, err := url.Parse(*logURL)
	if err != nil {
		return client.LogStateTracker{}, false, fmt.Errorf("failed to parse log URL %q: %w", *logURL, err)
	}
	f, err := newFetcher(root)
	if err != nil {
		return client.LogStateTracker{}, false, fmt.Errorf("failed to create fetcher: %v", err)
	}

	lSigV, err := note.NewVerifier(*logPubKey)
	if err != nil {
		return client.LogStateTracker{}, false, fmt.Errorf("unable to create new log signature verifier: %w", err)
	}

	lst, err := client.NewLogStateTracker(ctx, f, rfc6962.DefaultHasher, state, lSigV, *logOrigin, client.UnilateralConsensus(f))
	return lst, state == nil, err
}

// newFetcher creates a Fetcher for the log at the given root location.
func newFetcher(root *url.URL) (client.Fetcher, error) {
	if s := root.Scheme; s != "http" && s != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %s", s)
	}

	return func(ctx context.Context, p string) ([]byte, error) {
		u, err := root.Parse(p)
		if err != nil {
			return nil, err
		}
		resp, err := http.Get(u.String())
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return ioutil.ReadAll(resp.Body)
	}, nil
}
