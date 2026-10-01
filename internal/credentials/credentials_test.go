package credentials

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveReadUpdate(t *testing.T) {
	s := Store{filepath.Join(t.TempDir(), "clio")}
	if _, e := s.Read(); e == nil {
		t.Fatal("missing accepted")
	}
	for _, token := range []string{"first-secret", "second-secret"} {
		if e := s.Save(token); e != nil {
			t.Fatal(e)
		}
		v, e := s.Read()
		if e != nil || v != token {
			t.Fatal(v, e)
		}
		for p, want := range map[string]os.FileMode{s.Dir: 0700, filepath.Join(s.Dir, "credentials.json"): 0600} {
			st, e := os.Stat(p)
			if e != nil || st.Mode().Perm() != want {
				t.Fatal(p, e)
			}
		}
	}
	entries, e := os.ReadDir(s.Dir)
	if e != nil || len(entries) != 1 {
		t.Fatal(entries, e)
	}
}
func TestUnsafePaths(t *testing.T) {
	for _, kind := range []string{"directory-mode", "file-mode", "directory-link", "file-link"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			s := Store{filepath.Join(root, "clio")}
			if e := s.Save("secret"); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "directory-mode":
				os.Chmod(s.Dir, 0755)
			case "file-mode":
				os.Chmod(filepath.Join(s.Dir, "credentials.json"), 0644)
			case "directory-link":
				os.Rename(s.Dir, filepath.Join(root, "other"))
				os.Symlink(filepath.Join(root, "other"), s.Dir)
			case "file-link":
				p := filepath.Join(s.Dir, "credentials.json")
				os.Rename(p, p+".real")
				os.Symlink(p+".real", p)
			}
			if _, e := s.Read(); e == nil {
				t.Fatal("read accepted")
			}
			if e := s.Save("replacement"); e == nil {
				t.Fatal("save accepted")
			}
		})
	}
}
func TestLoginNoEchoOnNonTerminal(t *testing.T) {
	f, e := os.CreateTemp(t.TempDir(), "input")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	f.WriteString("supersecret\n")
	f.Seek(0, 0)
	var out bytes.Buffer
	e = Login(Store{filepath.Join(t.TempDir(), "clio")}, f, &out)
	if e == nil || strings.Contains(out.String(), "supersecret") || strings.Contains(e.Error(), "supersecret") {
		t.Fatal(e, out.String())
	}
}

func TestInvalidTokensAndNestedConfig(t *testing.T) {
	s := Store{filepath.Join(t.TempDir(), "config", "clio")}
	for _, token := range []string{"", " ", "secret\nline"} {
		if e := s.Save(token); e == nil || strings.Contains(e.Error(), "secret") {
			t.Fatal("invalid token accepted or disclosed")
		}
	}
	if e := s.Save("valid"); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(s.Dir, "credentials.json")
	if e := os.WriteFile(p, []byte(`{"token":"secret\nline"}`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Read(); e == nil || strings.Contains(e.Error(), "secret") {
		t.Fatal("invalid stored token accepted or disclosed")
	}
}
