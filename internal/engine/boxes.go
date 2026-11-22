package engine

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/josephgoksu/TaskWing/v2/internal/box"
)

func (r *runner) track(b *box.Box) {
	r.boxesMu.Lock()
	r.boxes = append(r.boxes, b)
	r.boxesMu.Unlock()
}

func (r *runner) untrack(b *box.Box) {
	r.boxesMu.Lock()
	defer r.boxesMu.Unlock()
	for i, x := range r.boxes {
		if x == b {
			r.boxes = append(r.boxes[:i], r.boxes[i+1:]...)
			return
		}
	}
}

// destroy scans a box for secrets, then deletes it (unless --keep).
func (r *runner) destroy(b *box.Box) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if r.o.Provider.Sandboxed() {
		var sb strings.Builder
		if _, err := r.o.Provider.Exec(ctx, b, secretScanScript, nil, &sb); err == nil {
			found, login := scanBox(sb.String())
			if login {
				r.boxesMu.Lock()
				r.agentLogin = true
				r.boxesMu.Unlock()
			}
			if len(found) > 0 {
				r.boxesMu.Lock()
				for _, f := range found {
					r.secrets = append(r.secrets, f+" in "+b.Name)
				}
				r.boxesMu.Unlock()
				r.ev.log("secret.found", map[string]any{"box": b.ID, "kinds": found})
			}
		}
	}
	if r.o.Keep {
		r.ev.log("box.kept", map[string]any{"box": b.ID})
		r.untrack(b)
		return
	}
	if err := r.o.Provider.Destroy(ctx, b); err != nil {
		r.ui.say("  %s could not delete box %s: %v (taskwing doctor finds it)", r.ui.amber("!"), b.ID, err)
	}
	r.ev.log("box.state", map[string]any{"box": b.ID, "state": "deleted"})
	r.untrack(b)
}

// githubCreds lists the GitHub credentials a fresh box already holds. Agents
// never start in such a box: they could push as the user. A scan that cannot
// run is an error, never "nothing found".
func (r *runner) githubCreds(ctx context.Context, b *box.Box) ([]string, error) {
	if !r.o.Provider.Sandboxed() {
		return nil, nil
	}
	var sb strings.Builder
	if _, err := r.o.Provider.Exec(ctx, b, secretScanScript, nil, &sb); err != nil {
		return nil, err
	}
	var gh []string
	found, _ := scanBox(sb.String())
	for _, f := range found {
		if strings.Contains(f, "GitHub") || strings.Contains(f, "gh ") {
			gh = append(gh, f)
		}
	}
	return gh, nil
}

// cleanup deletes any box still alive, for example after Ctrl-C or an error.
func (r *runner) cleanup() {
	r.boxesMu.Lock()
	left := append([]*box.Box(nil), r.boxes...)
	r.boxesMu.Unlock()
	var wg sync.WaitGroup
	for _, b := range left {
		wg.Add(1)
		go func(b *box.Box) { defer wg.Done(); r.destroy(b) }(b)
	}
	wg.Wait()
}

func shq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// upload copies a git bundle into the box and checks it out on a new branch.
func (r *runner) upload(ctx context.Context, b *box.Box, bundle, branch string) error {
	f, err := os.Open(bundle)
	if err != nil {
		return err
	}
	defer f.Close()
	// Base64 both ways, so no terminal layer in between can damage the bytes.
	pr, pw := io.Pipe()
	go func() {
		enc := base64.NewEncoder(base64.StdEncoding, pw)
		_, err := io.Copy(enc, f)
		if err == nil {
			err = enc.Close()
		}
		pw.CloseWithError(err)
	}()
	name, _ := git(ctx, r.root, "config", "user.name")
	email, _ := git(ctx, r.root, "config", "user.email")
	script := fmt.Sprintf(`set -e
rm -rf "$TW_REPO" "$TW_REPO.bundle"
mkdir -p "$(dirname "$TW_REPO")"
base64 -d > "$TW_REPO.bundle"
git clone -q "$TW_REPO.bundle" "$TW_REPO" </dev/null
cd "$TW_REPO"
git checkout -q -B %s </dev/null
git remote remove origin </dev/null || true
git config user.name %s
git config user.email %s
`, shq(branch), shq(name), shq(email))
	var out strings.Builder
	rc, err := r.o.Provider.Exec(ctx, b, script, pr, &out)
	pr.Close()
	if err != nil {
		return err
	}
	if rc != 0 {
		return fmt.Errorf("copying the repository into box %s failed (exit %d): %s", b.ID, rc, lastLines(out.String(), 3))
	}
	return nil
}
