package project

import (
	"testing"

	"github.com/spf13/afero"
)

// setupFS creates an in-memory filesystem with the given paths.
// Paths ending with "/" are created as directories, others as files.
// Paths with content use the format "path=content".
func setupFS(paths []string) afero.Fs {
	fs := afero.NewMemMapFs()
	for _, p := range paths {
		if p[len(p)-1] == '/' {
			_ = fs.MkdirAll(p, 0755)
		} else {
			_ = afero.WriteFile(fs, p, []byte(""), 0644)
		}
	}
	return fs
}


func TestDetect_MonorepoRootWithGitOnly(t *testing.T) {
	// Polyglot monorepo: .git at root, multiple subdirs with manifests, no root manifest
	// This is the markwise-app scenario.
	fs := setupFS([]string{
		"/monorepo/.git/",
		"/monorepo/web/package.json",
		"/monorepo/backend/go.mod",
		"/monorepo/admin/package.json",
		"/monorepo/docs/",
	})

	d := NewDetector(fs)
	ctx, err := d.Detect("/monorepo")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.RootPath != "/monorepo" {
		t.Errorf("RootPath = %q, want /monorepo", ctx.RootPath)
	}
	if ctx.GitRoot != "/monorepo" {
		t.Errorf("GitRoot = %q, want /monorepo", ctx.GitRoot)
	}
	if ctx.MarkerType != MarkerGit {
		t.Errorf("MarkerType = %v, want MarkerGit", ctx.MarkerType)
	}
	if !ctx.IsMonorepo {
		t.Error("IsMonorepo = false, want true (multiple nested projects detected)")
	}
}

func TestDetect_MonorepoRootWithTaskWing(t *testing.T) {
	// After bootstrap: .taskwing + .git at root, multiple subdirs with manifests
	fs := setupFS([]string{
		"/monorepo/.git/",
		"/monorepo/.taskwing/",
		"/monorepo/web/package.json",
		"/monorepo/backend/go.mod",
		"/monorepo/chrome-ext/package.json",
	})

	d := NewDetector(fs)
	ctx, err := d.Detect("/monorepo")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.MarkerType != MarkerTaskWing {
		t.Errorf("MarkerType = %v, want MarkerTaskWing", ctx.MarkerType)
	}
	if !ctx.IsMonorepo {
		t.Error("IsMonorepo = false, want true (.taskwing at root with nested projects)")
	}
}

func TestDetect_MonorepoSubdirectory(t *testing.T) {
	// Running from a subdirectory of a monorepo (existing behavior)
	fs := setupFS([]string{
		"/monorepo/.git/",
		"/monorepo/backend/go.mod",
		"/monorepo/web/package.json",
	})

	d := NewDetector(fs)
	ctx, err := d.Detect("/monorepo/backend")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.RootPath != "/monorepo/backend" {
		t.Errorf("RootPath = %q, want /monorepo/backend", ctx.RootPath)
	}
	if ctx.GitRoot != "/monorepo" {
		t.Errorf("GitRoot = %q, want /monorepo", ctx.GitRoot)
	}
	if !ctx.IsMonorepo {
		t.Error("IsMonorepo = false, want true (RootPath != GitRoot)")
	}
}

func TestDetect_SingleRepoWithGitOnly(t *testing.T) {
	// Single project: .git at root, no nested project directories
	fs := setupFS([]string{
		"/myproject/.git/",
		"/myproject/src/",
		"/myproject/README.md",
	})

	d := NewDetector(fs)
	ctx, err := d.Detect("/myproject")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.IsMonorepo {
		t.Error("IsMonorepo = true, want false (no nested projects)")
	}
}

func TestDetect_SingleRepoWithManifest(t *testing.T) {
	// Standard single project: .git + package.json at root
	fs := setupFS([]string{
		"/myapp/.git/",
		"/myapp/package.json",
		"/myapp/src/",
	})

	d := NewDetector(fs)
	ctx, err := d.Detect("/myapp")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.MarkerType != MarkerPackageJSON {
		t.Errorf("MarkerType = %v, want MarkerPackageJSON", ctx.MarkerType)
	}
	if ctx.IsMonorepo {
		t.Error("IsMonorepo = true, want false (single repo with root manifest)")
	}
}

func TestDetect_SingleNestedProjectNotMonorepo(t *testing.T) {
	// Git root with only ONE nested project dir - should NOT be detected as monorepo.
	// Threshold is 2+ nested projects.
	fs := setupFS([]string{
		"/repo/.git/",
		"/repo/backend/go.mod",
		"/repo/docs/",
	})

	d := NewDetector(fs)
	ctx, err := d.Detect("/repo")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.IsMonorepo {
		t.Error("IsMonorepo = true, want false (only 1 nested project, threshold is 2)")
	}
}

func TestDetect_SkipsNodeModulesAndVendor(t *testing.T) {
	// Ensure node_modules and vendor are always skipped (safety net)
	fs := setupFS([]string{
		"/repo/.git/",
		"/repo/node_modules/some-pkg/package.json",
		"/repo/vendor/dep/go.mod",
		"/repo/web/package.json",
	})

	d := NewDetector(fs)
	ctx, err := d.Detect("/repo")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.IsMonorepo {
		t.Error("IsMonorepo = true, want false (node_modules and vendor should be skipped)")
	}
}

func TestDetect_GitignoreSkipsDirs(t *testing.T) {
	// .gitignore lists "dist" and "build" — those should not count as projects
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("/repo/.git", 0755)
	_ = afero.WriteFile(fs, "/repo/.gitignore", []byte("dist\nbuild/\n"), 0644)
	// dist and build have manifests but are gitignored
	_ = afero.WriteFile(fs, "/repo/dist/package.json", []byte(""), 0644)
	_ = afero.WriteFile(fs, "/repo/build/package.json", []byte(""), 0644)
	// Only one real project dir
	_ = afero.WriteFile(fs, "/repo/web/package.json", []byte(""), 0644)

	d := NewDetector(fs)
	ctx, err := d.Detect("/repo")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.IsMonorepo {
		t.Error("IsMonorepo = true, want false (dist and build are gitignored, only 1 real project)")
	}
}

func TestDetect_GitignoreDoesNotAffectRealProjects(t *testing.T) {
	// .gitignore lists build artifacts, but real project dirs are not ignored
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("/repo/.git", 0755)
	_ = afero.WriteFile(fs, "/repo/.gitignore", []byte("dist\ncoverage\n.next\n"), 0644)
	_ = afero.WriteFile(fs, "/repo/web/package.json", []byte(""), 0644)
	_ = afero.WriteFile(fs, "/repo/api/go.mod", []byte(""), 0644)
	_ = afero.WriteFile(fs, "/repo/admin/package.json", []byte(""), 0644)

	d := NewDetector(fs)
	ctx, err := d.Detect("/repo")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if !ctx.IsMonorepo {
		t.Error("IsMonorepo = false, want true (3 real project dirs, gitignore only affects build artifacts)")
	}
}

func TestHasNestedProjects(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		dir   string
		want  bool
	}{
		{
			name: "two nested projects",
			paths: []string{
				"/root/web/package.json",
				"/root/api/go.mod",
			},
			dir:  "/root",
			want: true,
		},
		{
			name: "three nested projects (polyglot)",
			paths: []string{
				"/root/frontend/package.json",
				"/root/backend/go.mod",
				"/root/ml/pyproject.toml",
			},
			dir:  "/root",
			want: true,
		},
		{
			name: "one nested project",
			paths: []string{
				"/root/app/package.json",
				"/root/docs/",
			},
			dir:  "/root",
			want: false,
		},
		{
			name: "no nested projects",
			paths: []string{
				"/root/src/",
				"/root/README.md",
			},
			dir:  "/root",
			want: false,
		},
		{
			name: "always-skip dirs not counted",
			paths: []string{
				"/root/node_modules/pkg/package.json",
				"/root/vendor/lib/go.mod",
				"/root/web/package.json",
			},
			dir:  "/root",
			want: false,
		},
		{
			name: "hidden dirs not counted",
			paths: []string{
				"/root/.cache/tool/package.json",
				"/root/.internal/svc/go.mod",
			},
			dir:  "/root",
			want: false,
		},
		{
			name: "dockerfile counts as project marker",
			paths: []string{
				"/root/svc-a/Dockerfile",
				"/root/svc-b/Dockerfile",
			},
			dir:  "/root",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := setupFS(tt.paths)
			det := &detector{fs: fs}
			got := det.hasNestedProjects(tt.dir)
			if got != tt.want {
				t.Errorf("hasNestedProjects(%q) = %v, want %v", tt.dir, got, tt.want)
			}
		})
	}
}

func TestLoadGitignore(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    map[string]bool
	}{
		{
			name:    "simple names",
			content: "dist\nbuild\ncoverage\n",
			want:    map[string]bool{"dist": true, "build": true, "coverage": true},
		},
		{
			name:    "directory markers stripped",
			content: "dist/\nbuild/\ntarget/\n",
			want:    map[string]bool{"dist": true, "build": true, "target": true},
		},
		{
			name:    "root-anchored patterns stripped",
			content: "/dist\n/build/\n/out\n",
			want:    map[string]bool{"dist": true, "build": true, "out": true},
		},
		{
			name:    "comments and blank lines skipped",
			content: "# Build output\ndist\n\n# Coverage\ncoverage\n\n",
			want:    map[string]bool{"dist": true, "coverage": true},
		},
		{
			name:    "negation patterns skipped",
			content: "dist\n!dist/important\nbuild\n",
			want:    map[string]bool{"dist": true, "build": true},
		},
		{
			name:    "glob patterns skipped",
			content: "*.log\ndist\nlogs/*.txt\nbuild\n",
			want:    map[string]bool{"dist": true, "build": true},
		},
		{
			name:    "path patterns with separators skipped",
			content: "src/generated\ndist\nfoo/bar/baz\n",
			want:    map[string]bool{"dist": true},
		},
		{
			name:    "whitespace trimmed",
			content: "  dist  \n  build  \n",
			want:    map[string]bool{"dist": true, "build": true},
		},
		{
			name:    "empty file",
			content: "",
			want:    map[string]bool{},
		},
		{
			name:    "realistic gitignore",
			content: "# Dependencies\nnode_modules\n\n# Build\ndist/\nbuild/\n.next/\n\n# Coverage\ncoverage/\n\n# Env\n.env\n.env.local\n\n# Logs\n*.log\n",
			want:    map[string]bool{"node_modules": true, "dist": true, "build": true, ".next": true, "coverage": true, ".env": true, ".env.local": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			_ = afero.WriteFile(fs, "/project/.gitignore", []byte(tt.content), 0644)

			det := &detector{fs: fs}
			got := det.loadGitignore("/project")

			if len(got) != len(tt.want) {
				t.Errorf("loadGitignore() returned %d entries, want %d\n  got:  %v\n  want: %v", len(got), len(tt.want), got, tt.want)
				return
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("loadGitignore() missing key %q", k)
				}
			}
		})
	}
}

func TestLoadGitignore_NoFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	det := &detector{fs: fs}
	got := det.loadGitignore("/nonexistent")

	if len(got) != 0 {
		t.Errorf("loadGitignore() returned %d entries for missing file, want 0", len(got))
	}
}

func TestDetect_GitignoreWithAnchoredPattern(t *testing.T) {
	// /dist in gitignore should match top-level "dist" dir
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll("/repo/.git", 0755)
	_ = afero.WriteFile(fs, "/repo/.gitignore", []byte("/dist\n/out\n"), 0644)
	_ = afero.WriteFile(fs, "/repo/dist/package.json", []byte(""), 0644)
	_ = afero.WriteFile(fs, "/repo/out/package.json", []byte(""), 0644)
	_ = afero.WriteFile(fs, "/repo/web/package.json", []byte(""), 0644)

	d := NewDetector(fs)
	ctx, err := d.Detect("/repo")
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}

	if ctx.IsMonorepo {
		t.Error("IsMonorepo = true, want false (dist and out are gitignored via /dist and /out)")
	}
}
