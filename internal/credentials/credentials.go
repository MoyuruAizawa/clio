package credentials

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Store struct{ Dir string }

func Default() (Store, error) { d, e := os.UserConfigDir(); return Store{filepath.Join(d, "clio")}, e }
func secureInfo(s os.FileInfo, dir bool) error {
	if s.Mode()&os.ModeSymlink != 0 || s.IsDir() != dir || (!dir && !s.Mode().IsRegular()) {
		return errors.New("unsafe credentials path")
	}
	limit := os.FileMode(0600)
	if dir {
		limit = 0700
	}
	if s.Mode().Perm() & ^limit != 0 || !owned(s) {
		return errors.New("unsafe credentials ownership or permissions")
	}
	return nil
}

// Pin the checked directory so renames cannot redirect credential operations.
func (s Store) openRoot() (*os.Root, error) {
	before, e := os.Lstat(s.Dir)
	if e != nil || secureInfo(before, true) != nil {
		return nil, errors.New("credentials directory unavailable or unsafe")
	}
	r, e := os.OpenRoot(s.Dir)
	if e != nil {
		return nil, errors.New("cannot open credentials directory")
	}
	after, e := r.Stat(".")
	if e != nil || !os.SameFile(before, after) || secureInfo(after, true) != nil {
		r.Close()
		return nil, errors.New("credentials directory changed")
	}
	return r, nil
}
func (s Store) Read() (string, error) {
	root, e := s.openRoot()
	if e != nil {
		return "", errors.New("credentials unavailable; run clio auth login")
	}
	defer root.Close()
	p := "credentials.json"
	before, e := root.Lstat(p)
	if e != nil || secureInfo(before, false) != nil {
		return "", errors.New("credentials unavailable or unsafe")
	}
	f, e := root.Open(p)
	if e != nil {
		return "", errors.New("cannot read credentials")
	}
	defer f.Close()
	after, e := f.Stat()
	current, ce := root.Lstat(p)
	if e != nil || ce != nil || !os.SameFile(before, after) || !os.SameFile(after, current) || secureInfo(after, false) != nil || secureInfo(current, false) != nil {
		return "", errors.New("credentials changed")
	}
	var v struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(io.LimitReader(f, 65536)).Decode(&v) != nil || strings.TrimSpace(v.Token) == "" || strings.ContainsAny(v.Token, "\r\n") {
		return "", errors.New("invalid credentials")
	}
	return v.Token, nil
}
func (s Store) Save(token string) error {
	if strings.TrimSpace(token) == "" || strings.ContainsAny(token, "\r\n") {
		return errors.New("invalid token")
	}
	if e := os.MkdirAll(filepath.Dir(s.Dir), 0700); e != nil {
		return errors.New("cannot create config directory")
	}
	if e := os.Mkdir(s.Dir, 0700); e != nil && !os.IsExist(e) {
		return errors.New("cannot create credentials directory")
	}
	root, e := s.openRoot()
	if e != nil {
		return e
	}
	defer root.Close()
	p := "credentials.json"
	if info, e := root.Lstat(p); e == nil {
		if e = secureInfo(info, false); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return errors.New("cannot inspect credentials")
	}
	temp := ".credentials-" + rand.Text()
	f, e := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return errors.New("cannot save credentials")
	}
	defer root.Remove(temp)
	defer f.Close()
	if e = f.Chmod(0600); e != nil {
		return e
	}
	if e = json.NewEncoder(f).Encode(struct {
		Token string `json:"token"`
	}{token}); e != nil {
		return errors.New("cannot save credentials")
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if info, e := root.Lstat(p); e == nil {
		if e = secureInfo(info, false); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return errors.New("cannot inspect credentials")
	}
	if e = root.Rename(temp, p); e != nil {
		return errors.New("cannot replace credentials")
	}
	return nil
}
func Login(s Store, terminal *os.File, w io.Writer) error {
	if !term.IsTerminal(int(terminal.Fd())) {
		return errors.New("token input requires a terminal")
	}
	fmt.Fprint(w, "TypeSafe API token: ")
	b, e := term.ReadPassword(int(terminal.Fd()))
	fmt.Fprintln(w)
	if e != nil {
		return errors.New("cannot read token")
	}
	defer clear(b)
	return s.Save(string(b))
}
