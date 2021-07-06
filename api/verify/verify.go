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

// Package verify provides verification functions for armory drive transparency.
package verify

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/google/trillian/merkle/compact"
	"github.com/google/trillian/merkle/rfc6962/hasher"
	"golang.org/x/mod/sumdb/note"
)

// Bundle verifies that the Bundle is self-consistent, and consistent with the provided
// smaller checkpoint from the device.
//
// For a ProofBundle to be considered good, we need to:
//  1. check the signature on the new Checkpoint contained within
//  2. verify that the first oldCP.Size leaf hashes provided can reconstruct oldCP.Hash
//  3. verify that the first newCP.Size leaf hashes provided can reconstruct pb.NewCheckpoint.Hash
//  4. verify that the hash of pb.FirmwareRelease is among the list of leaf hashes provided
//  5. check that the signature on the FirmwareRelease manifest is valid
//  6. check that all provided artifact hashes are present in the FirmwareRelease manifist, and are
//     identical to the values the manifest claims they should be.
//
// If all of these checks hold, then we are sufficiently convinced that the firmware update is discoverable by others.
//
// TODO(al): Extend to support witnesses.
func Bundle(pb api.ProofBundle, oldCP api.Checkpoint, logSigV note.Verifier, frSigV note.Verifier, artifactHashes map[string][]byte) error {
	// First, check the signature on the new CP.
	newCP := &api.Checkpoint{}
	{
		newCPRaw, err := note.Open(pb.NewCheckpoint, note.VerifierList(logSigV))
		if err != nil {
			return fmt.Errorf("failed to verify signature on NewCheckpoint: %v", err)
		}
		if err := newCP.Unmarshal([]byte(newCPRaw.Text)); err != nil {
			return fmt.Errorf("failed to unmarshal NewCheckpoint: %v", err)
		}
	}

	if l := uint64(len(pb.LeafHashes)); l != newCP.Size {
		return fmt.Errorf("invalid ProofBundle - %d leafhashes for Checkpoint of size %d", l, newCP.Size)
	}

	// Next, ensure firmware manifest is discoverable:
	//  - prove its inclusion under the new checkpoint, and
	//  - prove that the new checkpoint is consistent with the device's old checkpoint
	h := hasher.DefaultHasher
	manifestHash := h.HashLeaf(pb.FirmwareRelease)
	tree := (&compact.RangeFactory{Hash: h.HashChildren}).NewEmptyRange(0)

	manifestFound := false
	oldCPFound := false
	newCPFound := false

	for i, leafHash := range pb.LeafHashes {
		if err := tree.Append(leafHash, nil); err != nil {
			return fmt.Errorf("error while appending leaf %d", i)
		}
		r, err := tree.GetRootHash(nil)
		if err != nil {
			return fmt.Errorf("failed to get root from compact tree: %v", err)
		}
		if !manifestFound {
			manifestFound = bytes.Equal(leafHash, manifestHash)
		}
		if tree.End() == oldCP.Size {
			oldCPFound = bytes.Equal(r, oldCP.Hash)
		}
		if tree.End() == newCP.Size {
			newCPFound = bytes.Equal(r, newCP.Hash)
		}
	}

	// If we don't have an oldCP (or oldCP is genuinely zero sized), then all future CPs are consistent with it.
	if !oldCPFound && oldCP.Size > 0 {
		return fmt.Errorf("unable to prove consistency - failed to recreate old checkpoint root %x", oldCP.Hash)
	}
	if !newCPFound {
		return fmt.Errorf("unable to prove consistency - failed to locate new checkpoint hash %x", newCP.Hash)
	}
	if !manifestFound {
		return fmt.Errorf("unable to prove inclusion - failed to locate manifest hash %x", manifestHash)
	}

	// Check the signature on the FirmwareRelease as we unmarshal it
	fr := &api.FirmwareRelease{}
	{
		frRaw, err := note.Open(pb.FirmwareRelease, note.VerifierList(frSigV))
		if err != nil {
			return fmt.Errorf("invalid signature on FirmwareRelease: %v", err)
		}
		if err := json.Unmarshal([]byte(frRaw.Text), fr); err != nil {
			return fmt.Errorf("failed to unmarshal FirmwareRelease: %v", err)
		}
	}

	// Lastly, check that the provided artifact hashes are the same as the ones
	// claimed by the FirmwareRelease manifest.
	for artifact, expected := range artifactHashes {
		h, ok := fr.ArtifactSHA256[artifact]
		if !ok {
			return fmt.Errorf("FirmwareRelease does not commit to artifact hash for %q", artifact)
		}
		if !bytes.Equal(expected, h) {
			return fmt.Errorf("expected artifact hash for %q is %x, but FirmwareRelease claims %x", artifact, expected, h)
		}
	}

	return nil
}
