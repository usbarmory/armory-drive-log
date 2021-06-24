create_release
--------------

This tool creates a JSON [api.FirmwareRelease](../../api/log_entries.go)
structure using data from the GitHub API.

e.g.:

```bash
$ go run ./cmd/create_release/ --logtostderr
I0624 16:22:53.603475 2728324 main.go:83] Fetching release info...
I0624 16:22:53.841686 2728324 main.go:103] Fetching and hashing source tarball...
I0624 16:22:54.263937 2728324 main.go:110] Identifying commit hash associated with release...
I0624 16:22:54.589783 2728324 main.go:129] Hashing release artifacts...
I0624 16:22:56.691517 2728324 main.go:73] Create FirmwareMeta:
{
  "description": "v2021.05.03 (beta pre-release)",
  "platform_id": "\u003cunset\u003e",
  "revision": "v2021.05.03",
  "artifact_sha256": {
    "armory-drive.csf": "ihsnc+Y4xLsZvTxVoOojObuRYyp5IB7EluKljp+5aLQ=",
    "armory-drive.imx": "GHf79fqYX3aXTOIiTdiuN2kT3aRvyqNs2mm63+XaOJc=",
    "armory-drive.ota": "Y0bu1Aypdxy3UshH/v4bPrKNxb7rbFcqna+vbzqUerc=",
    "armory-drive.sdp": "6jFHrxiS04Zu9eepCC0CC6YVr6Y3AB51ElNaJzhmCQQ=",
    "armory-drive.sig": "iiZM1po/bVNqChfv5kLp+kNerYKYkgf8tZetNUeZ1tM=",
    "armory-drive.srk": "NvUBxl9CZYe2al+G072gO+sFzZrWwx0F1iH0FUoDakM="
  },
  "source_url": "https://api.github.com/repos/f-secure-foundry/armory-drive/tarball/v2021.05.03",
  "source_sha256": "YJjQfKIS7KTXc6ItugSSvmY5eKdboIR+oCOHM9QgzVA=",
  "tool_chain": "tamago1.16.3",
  "build_args": {
    "REV": "5368a786cd492b025e86c6acc461f12d2d149923"
  }
}
```

Rate limits
===========

GitHub has a fairly low limit on the number of unauthenticated API requests any
given IP address can make, if you hit these you can create a GitHub
personal access token in your [account settings](https://github.com/settings/tokens)
and copy it into an environment veriable called `GITHUB_TOKEN` which will
substantially increase the limits.