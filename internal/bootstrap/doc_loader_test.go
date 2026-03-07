package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocIngestion(t *testing.T) {
	t.Run("root_docs_always_loaded", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Project"), 0o644)
		os.WriteFile(filepath.Join(dir, "ARCHITECTURE.md"), []byte("# Architecture"), 0o644)
		os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
		os.WriteFile(filepath.Join(dir, "docs", "setup.md"), []byte("# Setup Guide"), 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.Load()
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		if len(docs) == 0 {
			t.Error("docs count = 0, want > 0")
		}

		// Verify specific files found
		found := make(map[string]bool)
		for _, d := range docs {
			found[d.Path] = true
		}
		for _, expected := range []string{"README.md", "ARCHITECTURE.md", filepath.Join("docs", "setup.md")} {
			if !found[expected] {
				t.Errorf("Missing expected doc: %q", expected)
			}
		}
	})

	t.Run("categories_assigned_correctly", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Readme"), 0o644)
		os.WriteFile(filepath.Join(dir, "CONTRIBUTING.md"), []byte("# Contributing"), 0o644)
		os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude"), 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.Load()
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		cats := make(map[string]string)
		for _, d := range docs {
			cats[d.Name] = d.Category
		}

		if cats["README.md"] != "readme" {
			t.Errorf("README.md category = %q, want readme", cats["README.md"])
		}
		if cats["CONTRIBUTING.md"] != "contributing" {
			t.Errorf("CONTRIBUTING.md category = %q, want contributing", cats["CONTRIBUTING.md"])
		}
		if cats["CLAUDE.md"] != "ai-instructions" {
			t.Errorf("CLAUDE.md category = %q, want ai-instructions", cats["CLAUDE.md"])
		}
	})

	t.Run("large_files_skipped", func(t *testing.T) {
		dir := t.TempDir()
		// Create a file larger than 512KB
		bigContent := make([]byte, 600*1024)
		os.WriteFile(filepath.Join(dir, "README.md"), bigContent, 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.Load()
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		if len(docs) != 0 {
			t.Errorf("docs count = %d, want 0 (oversized file should be skipped)", len(docs))
		}
	})

	t.Run("non_doc_files_ignored", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Root"), 0o644)
		os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
		os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("# Guide"), 0o644)
		os.WriteFile(filepath.Join(dir, "docs", "image.png"), []byte("binary"), 0o644)
		os.WriteFile(filepath.Join(dir, "docs", "data.json"), []byte("{}"), 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.Load()
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		for _, d := range docs {
			ext := filepath.Ext(d.Path)
			if ext == ".png" || ext == ".json" {
				t.Errorf("Non-doc file included: %q", d.Path)
			}
		}
	})
}

func TestSubrepoMetadataExtraction(t *testing.T) {
	t.Run("loads_docs_from_sub_repos", func(t *testing.T) {
		dir := t.TempDir()

		// Root has a README
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Workspace"), 0o644)

		// Sub-repo "api" has docs
		apiDir := filepath.Join(dir, "api")
		os.MkdirAll(filepath.Join(apiDir, "docs"), 0o755)
		os.WriteFile(filepath.Join(apiDir, "README.md"), []byte("# API Service"), 0o644)
		os.WriteFile(filepath.Join(apiDir, "ARCHITECTURE.md"), []byte("# API Arch"), 0o644)
		os.WriteFile(filepath.Join(apiDir, "docs", "guide.md"), []byte("# API Guide"), 0o644)

		// Sub-repo "web" has docs
		webDir := filepath.Join(dir, "web")
		os.MkdirAll(webDir, 0o755)
		os.WriteFile(filepath.Join(webDir, "README.md"), []byte("# Web App"), 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.LoadForServices([]string{"api", "web"})
		if err != nil {
			t.Fatalf("LoadForServices error: %v", err)
		}

		// Should find: root README + api/README + api/ARCHITECTURE + api/docs/guide + web/README
		if len(docs) < 4 {
			t.Errorf("LoadForServices found %d docs, want >= 4", len(docs))
		}

		// Verify sub-repo docs have prefixed paths
		foundAPIReadme := false
		foundWebReadme := false
		foundAPIGuide := false
		for _, doc := range docs {
			switch doc.Path {
			case filepath.Join("api", "README.md"):
				foundAPIReadme = true
			case filepath.Join("web", "README.md"):
				foundWebReadme = true
			case filepath.Join("api", "docs", "guide.md"):
				foundAPIGuide = true
			}
		}

		if !foundAPIReadme {
			t.Error("Missing api/README.md in loaded docs")
		}
		if !foundWebReadme {
			t.Error("Missing web/README.md in loaded docs")
		}
		if !foundAPIGuide {
			t.Error("Missing api/docs/guide.md in loaded docs")
		}
	})

	t.Run("root_only_when_no_services", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Root"), 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.LoadForServices(nil)
		if err != nil {
			t.Fatalf("LoadForServices error: %v", err)
		}

		if len(docs) != 1 {
			t.Errorf("LoadForServices found %d docs, want 1 (root README only)", len(docs))
		}
	})

	t.Run("missing_service_dir_skipped", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Root"), 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.LoadForServices([]string{"nonexistent"})
		if err != nil {
			t.Fatalf("LoadForServices error: %v", err)
		}

		if len(docs) != 1 {
			t.Errorf("LoadForServices found %d docs, want 1 (root only, missing service skipped)", len(docs))
		}
	})

	t.Run("no_duplicate_paths", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Root"), 0o644)

		// Create sub-dir that also has root-level patterns
		subDir := filepath.Join(dir, "svc")
		os.MkdirAll(subDir, 0o755)
		os.WriteFile(filepath.Join(subDir, "README.md"), []byte("# Svc"), 0o644)

		loader := NewDocLoader(dir)
		docs, err := loader.LoadForServices([]string{"svc"})
		if err != nil {
			t.Fatalf("LoadForServices error: %v", err)
		}

		// Check for duplicates
		seen := make(map[string]int)
		for _, doc := range docs {
			seen[doc.Path]++
			if seen[doc.Path] > 1 {
				t.Errorf("Duplicate doc path: %q", doc.Path)
			}
		}
	})
}
