# create_release

This tool creates a [api.FirmwareRelease](../../api/log_entries.go)
structure.

The FirmwareRelease data is serialised to JSON format, and signed using
the Go sumdb's [signed note](https://pkg.go.dev/golang.org/x/mod/sumdb/note)
format.

The tool expects to find the private key for creating the signature in the file
specified by the `--private_key` flag, this key should be in the note.Signer key
format.

> :frog: You can use the
[generate_keys](https://github.com/f-secure-foundry/armory-drive-log/tree/master/cmd/generate_keys)
> command to create a suitable key pair.

e.g.:

```bash
$ go run ./cmd/create_release/ --logtostderr --description="AD release" --platform_id="platform" --commit_hash=acd1c56 --tool_chain=tamago1.16.3 --revision_tag=v2021.06.25 --artifacts='path/to/release/armory-drive.*' --private_key='path/to/private.key'
I0625 11:41:25.813439 3275756 main.go:75] Hashing release artifacts...
{
  "description": "AD release",
  "platform_id": "platform",
  "revision": "v2021.06.25",
  "artifact_sha256": {
    "armory-drive.csf": "tRaCZ3szSlNXhR2ZAvUWnO4g1qrYnkgP7S8w7Umk8mQ=",
    "armory-drive.imx": "vPI4FVFXkOm5Qi8oTmv8pkB5BRovBH7MqerHlBlr3hc=",
    "armory-drive.ota": "L3LJ4JX30Fw3TxtZpQT928zb9VmnoWuREP8Ht8/BpMA=",
    "armory-drive.sdp": "B+OrhYWfdbgxZz0iv4A/tc4TmARZiH02SQSYHL+mQaU=",
    "armory-drive.sig": "I4aFxDaRAYl4CUpgPFJGDTVa11lQ03xsAxILpWOSeiU=",
    "armory-drive.srk": "NvUBxl9CZYe2al+G072gO+sFzZrWwx0F1iH0FUoDakM="
  },
  "source_url": "https://github.com/f-secure-foundry/armory-drive/tarball/v2021.06.25",
  "source_sha256": "NtFkuGqfXBfSQo9GpcdveVTfxIN6i6CjNvRnVPW7f9M=",
  "tool_chain": "tamago1.16.3",
  "build_args": {
    "REV": "acd1c56"
  }
}

— test-key qaSd8Hs98ad41a11Xlzb9BU9Uoh3kXg39VvcyWlEoQn00SXGIQZ/3+ww7Br+TIKx+Rh0juWLPwGHN3k66potQDka1gU=
```
