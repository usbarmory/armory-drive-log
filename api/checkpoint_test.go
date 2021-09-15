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

package api

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnmarshalCheckpoint(t *testing.T) {
	for _, test := range []struct {
		desc    string
		m       string
		want    Checkpoint
		wantErr bool
	}{
		{
			desc: "valid one",
			m:    "ArmoryDrive Log v0\n123\nYmFuYW5hcw==\n",
			want: Checkpoint{
				Origin: "ArmoryDrive Log v0",
				Size:   123,
				Hash:   []byte("bananas"),
			},
		}, {
			desc:    "valid with trailing data",
			m:       "ArmoryDrive Log v0\n9944\ndGhlIHZpZXcgZnJvbSB0aGUgdHJlZSB0b3BzIGlzIGdyZWF0IQ==\nHere's some associated data.\n",
			wantErr: true,
		}, {
			desc:    "trailing data lines",
			m:       "ArmoryDrive Log v0\n9944\ndGhlIHZpZXcgZnJvbSB0aGUgdHJlZSB0b3BzIGlzIGdyZWF0IQ==\nlots\nof\nlines\n",
			wantErr: true,
		}, {
			desc:    "valid with trailing newlines",
			m:       "ArmoryDrive Log v0\n9944\ndGhlIHZpZXcgZnJvbSB0aGUgdHJlZSB0b3BzIGlzIGdyZWF0IQ==\n\n\n\n",
			wantErr: true,
		}, {
			desc:    "invalid - insufficient lines",
			m:       "Head\n9944\n",
			wantErr: true,
		}, {
			desc:    "invalid - empty header",
			m:       "\n9944\ndGhlIHZpZXcgZnJvbSB0aGUgdHJlZSB0b3BzIGlzIGdyZWF0IQ==\n",
			wantErr: true,
		}, {
			desc:    "invalid - missing newline on roothash",
			m:       "ArmoryDrive Log v0\n123\nYmFuYW5hcw==",
			wantErr: true,
		}, {
			desc:    "invalid size - not a number",
			m:       "ArmoryDrive Log v0\nbananas\ndGhlIHZpZXcgZnJvbSB0aGUgdHJlZSB0b3BzIGlzIGdyZWF0IQ==\n",
			wantErr: true,
		}, {
			desc:    "invalid size - negative",
			m:       "ArmoryDrive Log v0\n-34\ndGhlIHZpZXcgZnJvbSB0aGUgdHJlZSB0b3BzIGlzIGdyZWF0IQ==\n",
			wantErr: true,
		}, {
			desc:    "invalid size - too large",
			m:       "ArmoryDrive Log v0\n3438945738945739845734895735\ndGhlIHZpZXcgZnJvbSB0aGUgdHJlZSB0b3BzIGlzIGdyZWF0IQ==\n",
			wantErr: true,
		}, {
			desc:    "invalid roothash - not base64",
			m:       "ArmoryDrive Log v0\n123\nThisIsn'tBase64\n",
			wantErr: true,
		},
	} {
		t.Run(string(test.desc), func(t *testing.T) {
			var got Checkpoint
			if gotErr := got.Unmarshal([]byte(test.m)); (gotErr != nil) != test.wantErr {
				t.Fatalf("Unmarshal = %q, wantErr: %T", gotErr, test.wantErr)
			}
			if diff := cmp.Diff(test.want, got); len(diff) != 0 {
				t.Fatalf("Unmarshalled Checkpoint with diff %s", diff)
			}
		})
	}
}
