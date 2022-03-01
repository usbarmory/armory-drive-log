# Reproducible Build Verifier

This continuously monitors the log to look for claims about builds being published.
The log properties are checked to ensure the log is consistent with any previous
view, and that all claims are verifiably committed to by the log.

For each [`FirmwareRelease`](https://github.com/f-secure-foundry/armory-drive-log/blob/master/api/log_entries.go#L26)
manifest claim that it hasn't seen before, the following steps are taken:
 1. The source repository is cloned at the release tag
 2. The git revision at the tag is checked against the manifest
 3. The imx file is compiled from source
 4. The hash for the imx in the manifest is compared against the locally built version

## Running

In order to control the environment in which the code will be built,
a Dockerfile is supplied which will create a compatible environment.

This image can be built and executed using the following commands:

```bash
docker build . -t armory-drive-monitor -f ./cmd/monitor/Dockerfile
docker run armory-drive-monitor -v=1
```

Note that it is expected that the first entry in the log is not reproducibly
built. This is because of https://github.com/golang/go/issues/48557 which
was fixed in https://github.com/f-secure-foundry/armory-drive/commit/f3a32e3ab3aac6866a3bd8b70a6575d87335ef5d.

## TODO

 * Support for toolchains other than tamago 1.17.1
 * More visible reporting mechanisms than `glog` on success/failure
