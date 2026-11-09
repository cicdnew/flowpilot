#!/bin/bash
# Rebuilds lib/linux_amd64/libsql_experimental.a. Run from third_party/go-libsql:
#   mkdir -p /tmp/libsql-out
#   docker run --rm -v "$PWD/patches/build-linux-amd64.sh:/in/build.sh:ro" \
#     -v "$PWD/patches/0001-null-sqlite3-handle-after-close.patch:/in/fix.patch:ro" \
#     -v /tmp/libsql-out:/out ghcr.io/cross-rs/x86_64-unknown-linux-gnu:0.2.5 bash /in/build.sh
#   cp /tmp/libsql-out/libsql_experimental.a lib/linux_amd64/
set -euxo pipefail
curl -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain 1.85.0
. "$HOME/.cargo/env"
git clone --depth 1 --branch libsql-0.9.25 https://github.com/tursodatabase/libsql /src
cd /src
git apply --verbose /in/fix.patch
git diff
cd bindings/c
cargo build --release --locked
cp /src/target/release/libsql_experimental.a /out/
cp /src/bindings/c/include/libsql.h /out/
{
  echo "libsql tag: libsql-0.9.25 ($(git -C /src rev-parse HEAD))"
  echo "patch: tursodatabase/libsql@4e06af572ddf (PR #2261)"
  rustc --version
  cargo --version
  ldd --version 2>&1 | sed -n 1p
  gcc --version 2>&1 | sed -n 1p
  sha256sum /out/libsql_experimental.a | sed 's| /out/| |'
} > /out/BUILD_INFO.txt
cat /out/BUILD_INFO.txt
