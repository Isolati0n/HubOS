// ft: TEST PROTOTYPE ONLY (not the production feature). One-time signed URLs for moving a file or a folder from one node
// to another, the bytes going straight from the sending node to the receiving node. Go standard library only.
//
//	ft genkey  PRIVFILE PUBFILE                          make an Ed25519 key pair (hex), stands in for the management key
//	ft sign    PRIVFILE SENDER RECEIVER PATH TTL_SECONDS  print a token  (what hubd would do: authorise one transfer)
//	ft serve   PUBFILE SENDER ROOT LISTEN                 what the sending node would run (stands in for the node helper)
//
// Token = base64url(JSON payload) "." base64url(signature over the payload bytes).
package main

import (
	"archive/tar"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type payload struct {
	V        int    `json:"v"`
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Path     string `json:"path"`
	Exp      int64  `json:"exp"`
	Nonce    string `json:"nonce"`
}

func die(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...); os.Exit(1) }

func main() {
	if len(os.Args) < 2 {
		die("usage: ft genkey|sign|serve ...")
	}
	switch os.Args[1] {
	case "genkey":
		pub, priv, _ := ed25519.GenerateKey(rand.Reader)
		os.WriteFile(os.Args[2], []byte(hex.EncodeToString(priv)), 0o600)
		os.WriteFile(os.Args[3], []byte(hex.EncodeToString(pub)), 0o644)
	case "sign":
		raw, _ := os.ReadFile(os.Args[2])
		pb, err := hex.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil || len(pb) != ed25519.PrivateKeySize {
			die("bad private key")
		}
		ttl, _ := strconv.Atoi(os.Args[6])
		n := make([]byte, 16)
		rand.Read(n)
		p := payload{1, os.Args[3], os.Args[4], os.Args[5], time.Now().Unix() + int64(ttl), hex.EncodeToString(n)}
		j, _ := json.Marshal(p)
		sig := ed25519.Sign(ed25519.PrivateKey(pb), j)
		fmt.Print(base64.RawURLEncoding.EncodeToString(j) + "." + base64.RawURLEncoding.EncodeToString(sig))
	case "serve":
		serve(os.Args[2], os.Args[3], os.Args[4], os.Args[5])
	default:
		die("unknown command")
	}
}

var (
	mu   sync.Mutex
	used = map[string]bool{}
)

func serve(pubFile, me, root, listen string) {
	raw, _ := os.ReadFile(pubFile)
	pub, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		die("bad public key")
	}
	rootAbs, _ := filepath.EvalSymlinks(root)
	http.HandleFunc("/t/", func(w http.ResponseWriter, r *http.Request) {
		deny := func(code int, why string) {
			fmt.Fprintf(os.Stderr, "deny %d: %s\n", code, why)
			http.Error(w, why, code)
		}
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/t/"), ".", 2)
		if len(parts) != 2 {
			deny(400, "malformed token")
			return
		}
		j, e1 := base64.RawURLEncoding.DecodeString(parts[0])
		sig, e2 := base64.RawURLEncoding.DecodeString(parts[1])
		if e1 != nil || e2 != nil || !ed25519.Verify(ed25519.PublicKey(pub), j, sig) {
			deny(403, "bad signature")
			return
		}
		var p payload
		if json.Unmarshal(j, &p) != nil || p.V != 1 {
			deny(400, "bad payload")
			return
		}
		if p.Sender != me {
			deny(403, "token is for another sender")
			return
		}
		if time.Now().Unix() > p.Exp {
			deny(403, "expired")
			return
		}
		real, err := filepath.EvalSymlinks(p.Path)
		if err != nil || (real != rootAbs && !strings.HasPrefix(real, rootAbs+"/")) {
			deny(403, "path outside the shared root")
			return
		}
		mu.Lock()
		if used[p.Nonce] {
			mu.Unlock()
			deny(403, "token already used")
			return
		}
		used[p.Nonce] = true
		mu.Unlock()
		st, err := os.Stat(real)
		if err != nil {
			deny(404, "no such file")
			return
		}
		if !st.IsDir() {
			f, err := os.Open(real)
			if err != nil {
				deny(500, err.Error())
				return
			}
			defer f.Close()
			w.Header().Set("X-Name", filepath.Base(p.Path))
			http.ServeContent(w, r, filepath.Base(p.Path), st.ModTime(), f)
			return
		}
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("X-Name", filepath.Base(p.Path))
		tw := tar.NewWriter(w)
		defer tw.Close()
		base := filepath.Dir(real)
		filepath.WalkDir(real, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			info, _ := d.Info()
			link := ""
			if d.Type()&fs.ModeSymlink != 0 {
				link, _ = os.Readlink(path)
			}
			h, _ := tar.FileInfoHeader(info, link)
			h.Name, _ = filepath.Rel(base, path)
			if d.IsDir() {
				h.Name += "/"
			}
			tw.WriteHeader(h)
			if d.Type().IsRegular() {
				f, err := os.Open(path)
				if err == nil {
					io.Copy(tw, f)
					f.Close()
				}
			}
			return nil
		})
	})
	fmt.Fprintln(os.Stderr, "ft serve: listening on", listen)
	die("%v", http.ListenAndServe(listen, nil))
}
