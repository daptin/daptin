package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "virtual root", input: "/", want: ""},
		{name: "leading slash", input: "/images/photo.jpg", want: "images/photo.jpg"},
		{name: "normalizes within root", input: "images/../photo.jpg", want: "photo.jpg"},
		{name: "parent escape", input: "../outside", wantErr: true},
		{name: "nested parent escape", input: "images/../../outside", wantErr: true},
		{name: "backslash parent escape", input: "..\\outside", wantErr: true},
		{name: "drive path", input: "C:\\outside", wantErr: true},
		{name: "dotdot filename", input: "..notes", want: "..notes"},
		{name: "nul", input: "file\x00name", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidatePath(test.input)
			if test.wantErr {
				if !errors.Is(err, ErrPathEscapesRoot) {
					t.Fatalf("ValidatePath(%q) error = %v, want ErrPathEscapesRoot", test.input, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidatePath(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("ValidatePath(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestResolveLocalPathRejectsLexicalAndSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	inside, err := ResolveLocalPath(root, "/nested/file.txt")
	if err != nil {
		t.Fatalf("resolve inside path: %v", err)
	}
	if want := filepath.Join(root, "nested", "file.txt"); inside != want {
		t.Fatalf("inside path = %q, want %q", inside, want)
	}

	if _, err := ResolveLocalPath(root, "../outside.txt"); !errors.Is(err, ErrPathEscapesRoot) {
		t.Fatalf("lexical escape error = %v, want ErrPathEscapesRoot", err)
	}

	if runtime.GOOS == "windows" {
		return
	}
	link := filepath.Join(root, "outside-link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "existing.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"outside-link", "outside-link/existing.txt", "outside-link/new.txt"} {
		if _, err := ResolveLocalPath(root, name); !errors.Is(err, ErrPathEscapesRoot) {
			t.Errorf("ResolveLocalPath(%q) error = %v, want ErrPathEscapesRoot", name, err)
		}
	}
}

func TestResolvePathPreservesRemoteRoot(t *testing.T) {
	got, err := ResolvePath("remote:bucket/prefix", "/images/../photo.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if want := "remote:bucket/prefix/photo.jpg"; got != want {
		t.Fatalf("ResolvePath result = %q, want %q", got, want)
	}
	if _, err := ResolvePath("remote:bucket/prefix", "../../outside"); !errors.Is(err, ErrPathEscapesRoot) {
		t.Fatalf("remote escape error = %v, want ErrPathEscapesRoot", err)
	}
	if _, err := ResolvePath("", "file.txt"); err == nil {
		t.Fatal("empty storage root was accepted")
	}
	if _, err := ResolveLocalPath("", "file.txt"); err == nil {
		t.Fatal("empty local storage root was accepted")
	}
}

func TestResolveLocalPathAllowsConfiguredSymlinkRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	target := t.TempDir()
	parent := t.TempDir()
	root := filepath.Join(parent, "store")
	if err := os.Symlink(target, root); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveLocalPath(root, "file.txt")
	if err != nil {
		t.Fatalf("resolve beneath configured symlink root: %v", err)
	}
	if want := filepath.Join(root, "file.txt"); got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveLocalPathAllowsMissingStorageRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "not-created", "store")
	got, err := ResolveLocalPath(root, "nested/file.txt")
	if err != nil {
		t.Fatalf("resolve beneath missing root: %v", err)
	}
	if want := filepath.Join(root, "nested", "file.txt"); got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

func TestValidateLocalTreeDestinationRejectsDestinationSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional privileges on Windows")
	}
	root := t.TempDir()
	source := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "file.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "nested")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalTreeDestination(root, "", source); !errors.Is(err, ErrPathEscapesRoot) {
		t.Fatalf("tree destination error = %v, want ErrPathEscapesRoot", err)
	}
}
