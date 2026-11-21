package nixhash

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	oldHash = "sha256-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa="
	newHash = "sha256-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb="
)

func mismatchOutput(old, got string) string {
	return "error: hash mismatch in fixed-output derivation\n" +
		"  specified: " + old + "\n" +
		"  got:       " + got + "\n"
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func TestPatchFile(t *testing.T) {
	t.Run("patches the first file containing the hash", func(t *testing.T) {
		dir := t.TempDir()
		a := writeFile(t, dir, "a.nix", "vendorHash = \""+oldHash+"\";\n")
		b := writeFile(t, dir, "b.nix", "unrelated = true;\n")

		patched, err := patchFile([]string{a, b}, oldHash, newHash)
		if err != nil {
			t.Fatalf("patchFile() error = %v", err)
		}
		if !patched {
			t.Fatal("patchFile() = false, want true")
		}

		got, err := os.ReadFile(a)
		if err != nil {
			t.Fatalf("reading %s: %v", a, err)
		}
		if !strings.Contains(string(got), newHash) || strings.Contains(string(got), oldHash) {
			t.Fatalf("file content = %q, want hash replaced", got)
		}
	})

	t.Run("skips missing files", func(t *testing.T) {
		dir := t.TempDir()
		b := writeFile(t, dir, "b.nix", "vendorHash = \""+oldHash+"\";\n")

		patched, err := patchFile([]string{filepath.Join(dir, "missing.nix"), b}, oldHash, newHash)
		if err != nil {
			t.Fatalf("patchFile() error = %v", err)
		}
		if !patched {
			t.Fatal("patchFile() = false, want true")
		}
	})

	t.Run("reports not patched when hash is absent everywhere", func(t *testing.T) {
		dir := t.TempDir()
		a := writeFile(t, dir, "a.nix", "unrelated = true;\n")

		patched, err := patchFile([]string{a}, oldHash, newHash)
		if err != nil {
			t.Fatalf("patchFile() error = %v", err)
		}
		if patched {
			t.Fatal("patchFile() = true, want false")
		}
	})

	t.Run("preserves file permissions across the atomic write", func(t *testing.T) {
		dir := t.TempDir()
		a := writeFile(t, dir, "a.nix", "vendorHash = \""+oldHash+"\";\n")
		if err := os.Chmod(a, 0o640); err != nil {
			t.Fatalf("chmod %s: %v", a, err)
		}

		if _, err := patchFile([]string{a}, oldHash, newHash); err != nil {
			t.Fatalf("patchFile() error = %v", err)
		}

		info, err := os.Stat(a)
		if err != nil {
			t.Fatalf("stat %s: %v", a, err)
		}
		if info.Mode().Perm() != 0o640 {
			t.Fatalf("mode = %v, want 0640", info.Mode().Perm())
		}
	})
}

func TestTailBuffer(t *testing.T) {
	var tb tailBuffer
	tb.limit = 5

	_, _ = tb.Write([]byte("abc"))
	_, _ = tb.Write([]byte("defgh"))

	if got := tb.String(); got != "defgh" {
		t.Fatalf("String() = %q, want %q", got, "defgh")
	}
}

func TestFixTarget(t *testing.T) {
	t.Run("returns immediately when the build already succeeds", func(t *testing.T) {
		calls := 0
		build := func(context.Context, string) (string, error) {
			calls++
			return "", nil
		}

		changed, err := FixTarget(t.Context(), "pkg", nil, 10, build)
		if err != nil {
			t.Fatalf("FixTarget() error = %v", err)
		}
		if changed {
			t.Fatal("FixTarget() changed = true, want false")
		}
		if calls != 1 {
			t.Fatalf("build calls = %d, want 1", calls)
		}
	})

	t.Run("patches the hash and rebuilds until success", func(t *testing.T) {
		dir := t.TempDir()
		file := writeFile(t, dir, "flake.nix", "vendorHash = \""+oldHash+"\";\n")

		calls := 0
		build := func(context.Context, string) (string, error) {
			calls++
			if calls == 1 {
				return mismatchOutput(oldHash, newHash), errors.New("hash mismatch")
			}
			return "", nil
		}

		changed, err := FixTarget(t.Context(), "pkg", []string{file}, 10, build)
		if err != nil {
			t.Fatalf("FixTarget() error = %v", err)
		}
		if !changed {
			t.Fatal("FixTarget() changed = false, want true")
		}
		if calls != 2 {
			t.Fatalf("build calls = %d, want 2", calls)
		}

		got, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		if !strings.Contains(string(got), newHash) {
			t.Fatalf("file content = %q, want new hash", got)
		}
	})

	t.Run("errors when the build output has no hash mismatch", func(t *testing.T) {
		build := func(context.Context, string) (string, error) {
			return "error: something unrelated failed", errors.New("build failed")
		}

		_, err := FixTarget(t.Context(), "pkg", nil, 10, build)
		if err == nil || !strings.Contains(err.Error(), "no hash mismatch found") {
			t.Fatalf("FixTarget() error = %v, want 'no hash mismatch found'", err)
		}
	})

	t.Run("ignores specified/got lines without the real Nix preamble", func(t *testing.T) {
		build := func(context.Context, string) (string, error) {
			return "specified: " + oldHash + "\ngot: " + newHash + "\n", errors.New("build failed")
		}

		_, err := FixTarget(t.Context(), "pkg", nil, 10, build)
		if err == nil || !strings.Contains(err.Error(), "no hash mismatch found") {
			t.Fatalf("FixTarget() error = %v, want 'no hash mismatch found'", err)
		}
	})

	t.Run("errors when specified and got hashes are identical", func(t *testing.T) {
		build := func(context.Context, string) (string, error) {
			return mismatchOutput(oldHash, oldHash), errors.New("hash mismatch")
		}

		_, err := FixTarget(t.Context(), "pkg", nil, 10, build)
		if err == nil || !strings.Contains(err.Error(), "not a hash mismatch") {
			t.Fatalf("FixTarget() error = %v, want 'not a hash mismatch'", err)
		}
	})

	t.Run("errors when the hash is not found in any file", func(t *testing.T) {
		dir := t.TempDir()
		file := writeFile(t, dir, "flake.nix", "unrelated = true;\n")

		build := func(context.Context, string) (string, error) {
			return mismatchOutput(oldHash, newHash), errors.New("hash mismatch")
		}

		_, err := FixTarget(t.Context(), "pkg", []string{file}, 10, build)
		if err == nil || !strings.Contains(err.Error(), "not found in any of") {
			t.Fatalf("FixTarget() error = %v, want 'not found in any of'", err)
		}
	})

	t.Run("gives up after max iterations", func(t *testing.T) {
		dir := t.TempDir()
		// Each build reports a different hash than the last, so every
		// mismatch patches cleanly but the build never actually succeeds.
		file := writeFile(t, dir, "flake.nix", "vendorHash = \""+oldHash+"0\";\n")

		calls := 0
		build := func(context.Context, string) (string, error) {
			specified := fmt.Sprintf("%s%d", oldHash, calls)
			got := fmt.Sprintf("%s%d", oldHash, calls+1)
			calls++
			return mismatchOutput(specified, got), errors.New("hash mismatch")
		}

		_, err := FixTarget(t.Context(), "pkg", []string{file}, 3, build)
		if err == nil || !strings.Contains(err.Error(), "gave up after 3 iterations") {
			t.Fatalf("FixTarget() error = %v, want 'gave up after 3 iterations'", err)
		}
		if calls != 3 {
			t.Fatalf("build calls = %d, want 3", calls)
		}
	})
}
