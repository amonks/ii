# pkg/oci

## Overview

Pure Go library for building and pushing OCI container images using
`google/go-containerregistry`. No Docker daemon required.

Code: [pkg/oci/](../pkg/oci/)

## Types

- `ImageConfig` — container configuration with fields `Cmd []string`,
  `Env []string`, `WorkDir string`.

## Functions

### Layer Construction

- `BinaryLayer(binaryPath, destPath string) (v1.Layer, error)` — creates
  a tar layer containing one executable file with permissions 0755.
  The destPath leading slash is stripped for the tar entry.
- `FilesLayer(mappings map[string]string) (v1.Layer, error)` — creates a
  tar layer with multiple files. Keys are source paths on disk, values
  are destination paths in the container.

### Image Building

- `BuildAppImage(base v1.Image, binary string, files map[string]string, cfg ImageConfig) (v1.Image, error)` —
  composes base image + binary layer (at `/app/<basename>`) + files layer
  + container config. Binary and files layers are optional (empty string
  or nil map skips them).

### Registry Operations

- `Push(img v1.Image, ref string, opts ...remote.Option) error` — push
  an image to a remote registry.
- `FlyAuthOption(token string) remote.Option` — returns a `remote.Option`
  that authenticates with the Fly.io registry (username `x`, password is
  the FLY_API_TOKEN).
