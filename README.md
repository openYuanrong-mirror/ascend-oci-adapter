# ascend-oci-adapter

`ascend-oci-adapter` is a standalone Linux CLI for physical Ascend devices. It
uses MindCluster to discover devices and to produce OCI device, mount, and
environment candidates over a versioned JSON protocol.

The adapter does not own scheduling, resource leases, runtime lifecycle, or
deployment policy. Callers remain responsible for validating and applying the
returned OCI candidates.

## MindCluster dependency

The repository pins the official MindCluster `v26.2.0.beta.1` tag at commit
`2bd0a6e86d3c37925d3c000b0ff1ae59e8989456`. This is the first published
MindCluster tag that contains the `ascend-common/cdi` API used by the adapter.
The dependency is immutable, although the upstream tag is a beta release.

## Build

The adapter requires Linux, CGO, and a C compiler. It loads the host's
`libdcmi.so` at runtime through `dlopen`.

```bash
git submodule update --init --recursive
make build
```

`make build VERSION=v0.1.0` embeds the adapter version in the protocol response.

## CLI

```text
ascend-oci-adapter version  --output=json
ascend-oci-adapter discover --output=json
ascend-oci-adapter edits    --input=- --output=json
```

Requests are read from stdin and responses are written to stdout. Diagnostics
are written to stderr. MindCluster logs default to
`/var/log/ascend-oci-adapter/ascend-oci-adapter.log`; set the absolute path in
`ASCEND_OCI_ADAPTER_LOG_PATH` to override it.

Protocol schema 1 is preserved. The provider version identifies both the
adapter build and pinned MindCluster dependency, for example
`v0.1.0+mindcluster-v26.2.0.beta.1`.

## Release bundle

```bash
make release VERSION=v0.1.0
```

The bundle contains the standalone binary and the licenses required for
redistribution. Callers can pin a release asset and its generated SHA-256 file.
