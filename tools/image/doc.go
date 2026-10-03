// Package image holds the Phase B image build and its QEMU tests. The build is
// shell scripts in this directory (fetch-tools.sh, build-kernel.sh,
// build-base.sh, build-root-image.sh, build-disk.sh, build-bundle.sh); the files
// that go inside the root are in image/ at the top of the repo. The tests need
// QEMU and a long build, so they are behind the build tag "qemu":
//
//	go test -tags qemu -count=1 -timeout 120m ./tools/image
//
// See docs/image.md.
package image
