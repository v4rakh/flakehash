// Package nixhash patches stale Nix fixed-output-derivation hashes
// (vendorHash, npmDepsHash, pnpmDeps.hash, ...) by rebuilding a flake target
// and replacing whatever hash Nix reports as mismatched, repeating until
// the build succeeds.
package nixhash

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// maxBuildOutput caps how much of a build's output is kept in memory.
const maxBuildOutput = 1 << 20 // 1 MiB

// hashMismatchRe requires the specified/got hashes to follow Nix's own
// fixed-output-derivation error preamble, so arbitrary text a build phase
// prints to stdout/stderr can't be mistaken for a real hash mismatch.
var hashMismatchRe = regexp.MustCompile(
	`(?s)hash mismatch in fixed-output derivation.*?specified:\s*(sha256-[A-Za-z0-9+/=]+).*?got:\s*(sha256-[A-Za-z0-9+/=]+)`,
)

// BuildFunc builds target and returns its combined output, whether or not
// the build succeeded.
type BuildFunc func(ctx context.Context, target string) (string, error)

// Build runs `nix build` for target and returns its combined output.
func Build(ctx context.Context, target string) (string, error) {
	cmd := exec.CommandContext(ctx, "nix", "build", target, "--no-link") //nolint:gosec // target is a caller-supplied flake attr, the whole point of this CLI
	out := &tailBuffer{limit: maxBuildOutput}
	cmd.Stdout = out
	cmd.Stderr = out
	err := cmd.Run()
	return out.String(), err
}

// tailBuffer retains only the last limit bytes written, bounding memory
// use for subprocess output that could otherwise grow without limit.
type tailBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf.Write(p)
	if extra := t.buf.Len() - t.limit; extra > 0 {
		t.buf.Next(extra)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return t.buf.String() }

// FixTarget repeatedly builds target, patching any stale hash reported in
// the build output into files, until the build succeeds or maxIterations
// is exhausted. It reports whether any file was changed.
func FixTarget(ctx context.Context, target string, files []string, maxIterations int, build BuildFunc) (bool, error) {
	changed := false
	for range maxIterations {
		output, err := build(ctx, target)
		if err == nil {
			return changed, nil
		}

		match := hashMismatchRe.FindStringSubmatch(output)
		if match == nil {
			return changed, fmt.Errorf("build failed, no hash mismatch found in output:\n%s", output)
		}

		oldHash, newHash := match[1], match[2]
		if oldHash == newHash {
			return changed, fmt.Errorf("build keeps failing with identical hash %s, not a hash mismatch:\n%s", oldHash, output)
		}

		patched, err := patchFile(files, oldHash, newHash)
		if err != nil {
			return changed, err
		}
		if !patched {
			return changed, fmt.Errorf("hash %s not found in any of %v", oldHash, files)
		}

		fmt.Printf("%s: %s -> %s\n", target, oldHash, newHash)
		changed = true
	}
	return changed, fmt.Errorf("gave up after %d iterations", maxIterations)
}

// patchFile replaces oldHash with newHash in the first file that contains
// it, preserving that file's permissions.
func patchFile(files []string, oldHash, newHash string) (bool, error) {
	for _, file := range files {
		content, err := os.ReadFile(file) //nolint:gosec // file is a caller-supplied --file path, the whole point of this CLI
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, fmt.Errorf("reading %s: %w", file, err)
		}
		if !strings.Contains(string(content), oldHash) {
			continue
		}

		info, err := os.Stat(file)
		if err != nil {
			return false, fmt.Errorf("stat %s: %w", file, err)
		}

		updated := strings.ReplaceAll(string(content), oldHash, newHash)
		if err := writeFileAtomic(file, []byte(updated), info.Mode()); err != nil {
			return false, fmt.Errorf("writing %s: %w", file, err)
		}
		return true, nil
	}
	return false, nil
}

// writeFileAtomic replaces path's contents by writing to a temp file in the
// same directory and renaming it into place, so an interrupt or crash mid-
// write never leaves path truncated.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*") //nolint:gosec // path is a caller-supplied --file path, the whole point of this CLI
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
