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

package verify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/usbarmory/armory-drive-log/api"
	"github.com/transparency-dev/merkle/compact"
	"github.com/transparency-dev/merkle/rfc6962"
	"golang.org/x/mod/sumdb/note"
)

const (
	testLogOrigin = "ArmoryDrive Log v0"
	testLogSignerPrivate = "PRIVATE+KEY+test-log+2b51c375+Ad+qPnxRnV5XOivW9d42+7xewjKwjXwYr3z9SeP+OOVK"
	testLogSignerPublic  = "test-log+2b51c375+Ae73xsZZky/7/mv/jmPEAAVHi3KXBTz4F2DV6H/Htd4P"

	testFirmwarePrivate = "PRIVATE+KEY+test-firmware+ab2fae50+AaB6EfEYBzXsuL9Ad+aFOY7zanhCGIyq/YzdDgVllp7i"
	testFirmwarePublic  = "test-firmware+ab2fae50+ATbJye7l6/LavuMm5iBSu67hmxPv1yx+d9BhcEki1Q4Z"
)

var (
	testLeafHashes = [][]byte{
		[]byte("Many"),
		[]byte("Leaves"),
		[]byte("In"),
		[]byte("Autumn"),
		[]byte("Are"),
		[]byte("Golden"),
	}
)

// buildLog calculates a set of incremental root hashes for a growing log by adding leafHahses one at a time.
func buildLog(t *testing.T, leafHashes [][]byte) [][]byte {
	roots := make([][]byte, 0)
	h := rfc6962.DefaultHasher
	tree := (&compact.RangeFactory{Hash: h.HashChildren}).NewEmptyRange(0)
	for _, lh := range leafHashes {
		if err := tree.Append(lh, nil); err != nil {
			t.Fatalf("Failed to append: %v", err)
		}
		r, err := tree.GetRootHash(nil)
		if err != nil {
			t.Fatalf("Failed to get root: %v", err)
		}

		roots = append(roots, r)
	}
	return roots
}

func TestBundle(t *testing.T) {
	logSig := mustMakeSigner(t, testLogSignerPrivate)
	fwSig := mustMakeSigner(t, testFirmwarePrivate)
	logSigV := mustMakeVerifier(t, testLogSignerPublic)
	fwSigV := mustMakeVerifier(t, testFirmwarePublic)

	h := rfc6962.DefaultHasher
	firmwareImageHash := []byte("Firmware Hash")
	commitArtifacts := map[string][]byte{
		"FirmwareImage": firmwareImageHash,
		"Thingy":        []byte("Magig"),
		"Art":           []byte("Fact"),
	}
	fw := makeFirmwareRelease(t, commitArtifacts, fwSig)
	manifestHash := h.HashLeaf(fw)
	leafHashes := append(testLeafHashes, manifestHash)
	roots := buildLog(t, leafHashes)

	for _, test := range []struct {
		desc          string
		pb            api.ProofBundle
		oldCP         api.Checkpoint
		wantArtifacts map[string][]byte
		wantErr       bool
	}{
		{
			desc: "works",
			pb: api.ProofBundle{
				FirmwareRelease: fw,
				NewCheckpoint:   makeCheckpoint(t, len(leafHashes), roots[len(roots)-1], logSig),
				LeafHashes:      leafHashes,
			},
			oldCP: api.Checkpoint{
				Size: 1,
				Hash: roots[0],
			},
			wantArtifacts: map[string][]byte{
				"FirmwareImage": firmwareImageHash,
				"Thingy":        []byte("Magig"),
			},
		}, {
			desc: "wrong firmware",
			pb: api.ProofBundle{
				FirmwareRelease: fw,
				NewCheckpoint:   makeCheckpoint(t, len(leafHashes), roots[len(roots)-1], logSig),
				LeafHashes:      leafHashes,
			},
			oldCP: api.Checkpoint{
				Size: 1,
				Hash: roots[0],
			},
			// Unexpected FirmwareImage hash
			wantArtifacts: map[string][]byte{
				"FirmwareImage": []byte("Have a banana"),
				"Thingy":        []byte("Magig"),
			},
			wantErr: true,
		}, {
			desc: "missing artifact",
			pb: api.ProofBundle{
				FirmwareRelease: fw,
				NewCheckpoint:   makeCheckpoint(t, len(leafHashes), roots[len(roots)-1], logSig),
				LeafHashes:      leafHashes,
			},
			oldCP: api.Checkpoint{
				Size: 1,
				Hash: roots[0],
			},
			// This artifact isn't committed to by the FirmwareRelease:
			wantArtifacts: map[string][]byte{
				"Sekret": []byte("Squirrel"),
			},
			wantErr: true,
		}, {
			desc: "bad consistency - can't prove old CP",
			pb: api.ProofBundle{
				FirmwareRelease: fw,
				NewCheckpoint:   makeCheckpoint(t, len(leafHashes), roots[len(roots)-1], logSig),
				LeafHashes:      leafHashes,
			},
			oldCP: api.Checkpoint{
				Size: 1,
				// Not going to be able to recreate this hash from the test leaves:
				Hash: []byte("This hash is not reconstructible"),
			},
			wantArtifacts: map[string][]byte{
				"FirmwareImage": firmwareImageHash,
			},
			wantErr: true,
		}, {
			desc: "bad consistency - can't prove new CP",
			pb: api.ProofBundle{
				FirmwareRelease: fw,
				// Provide an inconsistent new CP root hash
				NewCheckpoint: makeCheckpoint(t, len(leafHashes), []byte("This root not present"), logSig),
				LeafHashes:    leafHashes,
			},
			oldCP: api.Checkpoint{
				Size: 1,
				Hash: roots[0],
			},
			wantArtifacts: map[string][]byte{
				"FirmwareImage": firmwareImageHash,
			},
			wantErr: true,
		}, {
			desc: "bad consistency - can't prove manifest",
			pb: api.ProofBundle{
				FirmwareRelease: fw,
				NewCheckpoint:   makeCheckpoint(t, len(leafHashes), roots[len(roots)-1], logSig),
				// Replace manifest hash with one which doesn't match
				LeafHashes: append(append([][]byte{}, leafHashes[0:len(leafHashes)-1]...), []byte("wrong manifest hash")),
			},
			oldCP: api.Checkpoint{
				Size: 1,
				Hash: roots[0],
			},
			wantArtifacts: map[string][]byte{
				"FirmwareImage": firmwareImageHash,
			},
			wantErr: true,
		}, {
			desc: "invalid firmware manifest signature",
			pb: api.ProofBundle{
				// Invalid - signed by log's key
				FirmwareRelease: makeFirmwareRelease(t, commitArtifacts, logSig),
				NewCheckpoint:   makeCheckpoint(t, len(leafHashes), roots[len(roots)-1], logSig),
			},
			oldCP: api.Checkpoint{
				Size: 1,
				Hash: roots[0],
			},
			wantArtifacts: map[string][]byte{
				"FirmwareImage": firmwareImageHash,
			},
			wantErr: true,
		}, {
			desc: "invalid log checkpoint signature",
			pb: api.ProofBundle{
				FirmwareRelease: makeFirmwareRelease(t, commitArtifacts, fwSig),
				// Invalid - signed by firmware key
				NewCheckpoint: makeCheckpoint(t, len(leafHashes), roots[len(roots)-1], fwSig),
				LeafHashes:    leafHashes,
			},
			oldCP: api.Checkpoint{
				Size: 1,
				Hash: roots[0],
			},
			wantArtifacts: map[string][]byte{
				"FirmwareImage": firmwareImageHash,
			},
			wantErr: true,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			err := Bundle(test.pb, test.oldCP, logSigV, fwSigV, test.wantArtifacts, testLogOrigin)
			if gotErr := err != nil; gotErr != test.wantErr {
				t.Fatalf("wantErr: %v, but got: %v", test.wantErr, err)
			}
		})
	}
}

func mustMakeSigner(t *testing.T, secK string) note.Signer {
	t.Helper()
	s, err := note.NewSigner(secK)
	if err != nil {
		t.Fatalf("Failed to create signer from %q: %v", secK, err)
	}
	return s
}

func mustMakeVerifier(t *testing.T, pubK string) note.Verifier {
	t.Helper()
	v, err := note.NewVerifier(pubK)
	if err != nil {
		t.Fatalf("Failed to create verifier from %q: %v", pubK, err)
	}
	return v
}

func makeFirmwareRelease(t *testing.T, artifacts map[string][]byte, sig note.Signer) []byte {
	fr := api.FirmwareRelease{
		Description:    "A release",
		PlatformID:     "7½",
		Revision:       "Helps with tests",
		ArtifactSHA256: artifacts,
		SourceURL:      "https://www.youtube.com/watch?v=IC7l3V1nhWc&t=0s",
		SourceSHA256:   []byte("One two three four five. Six seven eight nine ten. Eleven twelve."),
		ToolChain:      "Snap on",
		BuildArgs: map[string]string{
			"REV": "Lovejoy",
		},
	}
	frRaw, err := json.MarshalIndent(fr, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal FirmwareRelease: %v", err)
	}
	n, err := note.Sign(&note.Note{Text: string(frRaw) + "\n"}, sig)
	if err != nil {
		t.Fatalf("Failed to sign FirmwareRelease: %v", err)
	}
	return n
}

func makeCheckpoint(t *testing.T, size int, hash []byte, sig note.Signer) []byte {
	t.Helper()
	cp := fmt.Sprintf("%s\n%d\n%s\n", testLogOrigin, int64(size), base64.StdEncoding.EncodeToString(hash))
	n, err := note.Sign(&note.Note{Text: cp}, sig)
	if err != nil {
		t.Fatalf("Failed to sign checkpoint: %v", err)
	}
	return n
}
