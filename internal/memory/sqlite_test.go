package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "taskwing-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	store, err := NewSQLiteStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, cleanup
}

func TestCreateFeature(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	f := Feature{
		Name:     "Auth",
		OneLiner: "User authentication system",
		Tags:     []string{"core", "security"},
	}

	err := store.CreateFeature(f)
	if err != nil {
		t.Fatalf("create feature: %v", err)
	}

	// Verify feature was created
	features, err := store.ListFeatures()
	if err != nil {
		t.Fatalf("list features: %v", err)
	}

	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}

	if features[0].Name != "Auth" {
		t.Errorf("expected name 'Auth', got '%s'", features[0].Name)
	}

	// Verify markdown file was created
	mdPath := filepath.Join(store.basePath, "features", "auth.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("markdown file not created")
	}
}

func TestAddDecision(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a feature first
	f := Feature{
		ID:       "f-test123",
		Name:     "Payments",
		OneLiner: "Payment processing",
	}
	if err := store.CreateFeature(f); err != nil {
		t.Fatalf("create feature: %v", err)
	}

	// Add a decision
	d := Decision{
		Title:     "Use Stripe",
		Summary:   "Stripe for payment processing",
		Reasoning: "Best API, good documentation",
		Tradeoffs: "Higher fees than alternatives",
	}
	if err := store.AddDecision("f-test123", d); err != nil {
		t.Fatalf("add decision: %v", err)
	}

	// Verify decision was added
	decisions, err := store.GetDecisions("f-test123")
	if err != nil {
		t.Fatalf("get decisions: %v", err)
	}

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	if decisions[0].Title != "Use Stripe" {
		t.Errorf("expected title 'Use Stripe', got '%s'", decisions[0].Title)
	}

	// Verify decision count was updated
	feature, err := store.GetFeature("f-test123")
	if err != nil {
		t.Fatalf("get feature: %v", err)
	}

	if feature.DecisionCount != 1 {
		t.Errorf("expected decision count 1, got %d", feature.DecisionCount)
	}
}

func TestLinkFeatures(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create features
	features := []Feature{
		{ID: "f-auth", Name: "Auth", OneLiner: "Authentication"},
		{ID: "f-users", Name: "Users", OneLiner: "User management"},
		{ID: "f-payments", Name: "Payments", OneLiner: "Payment processing"},
	}

	for _, f := range features {
		if err := store.CreateFeature(f); err != nil {
			t.Fatalf("create feature %s: %v", f.Name, err)
		}
	}

	// Create relationships
	// Users depends on Auth
	if err := store.Link("f-users", "f-auth", EdgeTypeDependsOn); err != nil {
		t.Fatalf("link users->auth: %v", err)
	}

	// Payments depends on Users
	if err := store.Link("f-payments", "f-users", EdgeTypeDependsOn); err != nil {
		t.Fatalf("link payments->users: %v", err)
	}

	// Test GetDependencies (recursive)
	deps, err := store.GetDependencies("f-payments")
	if err != nil {
		t.Fatalf("get dependencies: %v", err)
	}

	if len(deps) != 2 {
		t.Errorf("expected 2 dependencies (users, auth), got %d: %v", len(deps), deps)
	}

	// Test GetDependents
	dependents, err := store.GetDependents("f-auth")
	if err != nil {
		t.Fatalf("get dependents: %v", err)
	}

	if len(dependents) != 2 {
		t.Errorf("expected 2 dependents (users, payments), got %d: %v", len(dependents), dependents)
	}
}

func TestCircularDependencyPrevention(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create features
	features := []Feature{
		{ID: "f-a", Name: "A", OneLiner: "Feature A"},
		{ID: "f-b", Name: "B", OneLiner: "Feature B"},
		{ID: "f-c", Name: "C", OneLiner: "Feature C"},
	}

	for _, f := range features {
		if err := store.CreateFeature(f); err != nil {
			t.Fatalf("create feature: %v", err)
		}
	}

	// A -> B -> C
	store.Link("f-a", "f-b", EdgeTypeDependsOn)
	store.Link("f-b", "f-c", EdgeTypeDependsOn)

	// Try to create circular: C -> A (should fail)
	err := store.Link("f-c", "f-a", EdgeTypeDependsOn)
	if err == nil {
		t.Error("expected circular dependency error, got nil")
	}
}

func TestDeleteFeatureWithDependents(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create features
	store.CreateFeature(Feature{ID: "f-base", Name: "Base", OneLiner: "Base feature"})
	store.CreateFeature(Feature{ID: "f-derived", Name: "Derived", OneLiner: "Derived feature"})

	// Derived depends on Base
	store.Link("f-derived", "f-base", EdgeTypeDependsOn)

	// Try to delete Base (should fail)
	err := store.DeleteFeature("f-base")
	if err == nil {
		t.Error("expected error when deleting feature with dependents")
	}

	// Delete Derived first, then Base should work
	if err := store.DeleteFeature("f-derived"); err != nil {
		t.Fatalf("delete derived: %v", err)
	}

	if err := store.DeleteFeature("f-base"); err != nil {
		t.Fatalf("delete base: %v", err)
	}
}

func TestIntegrityCheck(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a feature
	store.CreateFeature(Feature{ID: "f-test", Name: "Test", OneLiner: "Test feature"})

	// Manually delete the markdown file to create an integrity issue
	mdPath := filepath.Join(store.basePath, "features", "test.md")
	os.Remove(mdPath)

	// Run integrity check
	issues, err := store.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	if issues[0].Type != "missing_file" {
		t.Errorf("expected issue type 'missing_file', got '%s'", issues[0].Type)
	}

	// Repair should regenerate the file
	if err := store.Repair(); err != nil {
		t.Fatalf("repair: %v", err)
	}

	// File should exist now
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("markdown file not regenerated after repair")
	}
}

func TestIndexCache(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create features
	store.CreateFeature(Feature{Name: "Auth", OneLiner: "Authentication"})
	store.CreateFeature(Feature{Name: "Users", OneLiner: "User management"})

	// Get index (should build cache)
	index, err := store.GetIndex()
	if err != nil {
		t.Fatalf("get index: %v", err)
	}

	if len(index.Features) != 2 {
		t.Errorf("expected 2 features in index, got %d", len(index.Features))
	}

	// Verify index file was created
	indexPath := filepath.Join(store.basePath, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index.json not created")
	}

	// Verify LastUpdated is recent
	if time.Since(index.LastUpdated) > time.Minute {
		t.Error("index LastUpdated is not recent")
	}
}

// === QA Remediation Tests ===

func TestUpdateFeature(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create initial feature
	f := Feature{
		ID:       "f-update",
		Name:     "Original",
		OneLiner: "Original description",
		Status:   FeatureStatusActive,
		Tags:     []string{"v1"},
	}
	if err := store.CreateFeature(f); err != nil {
		t.Fatalf("create feature: %v", err)
	}

	// Update the feature
	f.Name = "Updated"
	f.OneLiner = "Updated description"
	f.Status = FeatureStatusDeprecated
	f.Tags = []string{"v2", "deprecated"}

	if err := store.UpdateFeature(f); err != nil {
		t.Fatalf("update feature: %v", err)
	}

	// Verify update
	updated, err := store.GetFeature("f-update")
	if err != nil {
		t.Fatalf("get feature: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got '%s'", updated.Name)
	}
	if updated.OneLiner != "Updated description" {
		t.Errorf("expected updated description, got '%s'", updated.OneLiner)
	}
	if updated.Status != FeatureStatusDeprecated {
		t.Errorf("expected status 'deprecated', got '%s'", updated.Status)
	}
}

func TestUpdateDecision(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create feature and initial decision
	store.CreateFeature(Feature{ID: "f-dec", Name: "DecisionTest", OneLiner: "Test"})
	d := Decision{
		ID:        "d-update",
		Title:     "Original Title",
		Summary:   "Original summary",
		Reasoning: "Original reason",
	}
	store.AddDecision("f-dec", d)

	// Update the decision
	d.Title = "Updated Title"
	d.Summary = "Updated summary"
	d.Reasoning = "Updated reason"
	d.Tradeoffs = "New tradeoffs"

	if err := store.UpdateDecision(d); err != nil {
		t.Fatalf("update decision: %v", err)
	}

	// Verify update
	decisions, _ := store.GetDecisions("f-dec")
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	if decisions[0].Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got '%s'", decisions[0].Title)
	}
	if decisions[0].Summary != "Updated summary" {
		t.Errorf("expected updated summary")
	}
}

func TestDeleteDecision(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create feature with decisions
	store.CreateFeature(Feature{ID: "f-deldec", Name: "DeleteDecTest", OneLiner: "Test"})
	store.AddDecision("f-deldec", Decision{ID: "d-del1", Title: "Decision 1", Summary: "First"})
	store.AddDecision("f-deldec", Decision{ID: "d-del2", Title: "Decision 2", Summary: "Second"})

	// Verify 2 decisions exist
	decisions, _ := store.GetDecisions("f-deldec")
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions, got %d", len(decisions))
	}

	// Delete one decision
	if err := store.DeleteDecision("d-del1"); err != nil {
		t.Fatalf("delete decision: %v", err)
	}

	// Verify only 1 remains
	decisions, _ = store.GetDecisions("f-deldec")
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision after delete, got %d", len(decisions))
	}
	if decisions[0].ID != "d-del2" {
		t.Errorf("wrong decision remaining")
	}

	// Verify decision count updated on feature
	feature, _ := store.GetFeature("f-deldec")
	if feature.DecisionCount != 1 {
		t.Errorf("expected decision count 1, got %d", feature.DecisionCount)
	}
}

func TestUnlink(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create features with relationship
	store.CreateFeature(Feature{ID: "f-unlink1", Name: "Unlink1", OneLiner: "Test"})
	store.CreateFeature(Feature{ID: "f-unlink2", Name: "Unlink2", OneLiner: "Test"})
	store.Link("f-unlink1", "f-unlink2", EdgeTypeDependsOn)

	// Verify link exists
	deps, _ := store.GetDependencies("f-unlink1")
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency before unlink, got %d", len(deps))
	}

	// Unlink
	if err := store.Unlink("f-unlink1", "f-unlink2", EdgeTypeDependsOn); err != nil {
		t.Fatalf("unlink: %v", err)
	}

	// Verify link removed
	deps, _ = store.GetDependencies("f-unlink1")
	if len(deps) != 0 {
		t.Errorf("expected 0 dependencies after unlink, got %d", len(deps))
	}
}

func TestGetRelated(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create a graph: A -> B -> C, A <-> D
	store.CreateFeature(Feature{ID: "f-a", Name: "A", OneLiner: "A"})
	store.CreateFeature(Feature{ID: "f-b", Name: "B", OneLiner: "B"})
	store.CreateFeature(Feature{ID: "f-c", Name: "C", OneLiner: "C"})
	store.CreateFeature(Feature{ID: "f-d", Name: "D", OneLiner: "D"})

	store.Link("f-a", "f-b", EdgeTypeDependsOn)
	store.Link("f-b", "f-c", EdgeTypeDependsOn)
	store.Link("f-a", "f-d", EdgeTypeRelated)

	// Get related with depth 1
	related, err := store.GetRelated("f-a", 1)
	if err != nil {
		t.Fatalf("get related: %v", err)
	}

	// Should include B and D (direct connections)
	if len(related) < 2 {
		t.Errorf("expected at least 2 related features, got %d: %v", len(related), related)
	}

	// Get related with depth 2
	related, _ = store.GetRelated("f-a", 2)
	// Should include B, C, D (2-hop reach)
	if len(related) < 3 {
		t.Errorf("expected at least 3 related features at depth 2, got %d", len(related))
	}
}

// === Negative Test Cases ===

func TestCreateFeatureDuplicateName(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create first feature
	store.CreateFeature(Feature{Name: "Duplicate", OneLiner: "First"})

	// Try to create with same name - should fail
	err := store.CreateFeature(Feature{Name: "Duplicate", OneLiner: "Second"})
	if err == nil {
		t.Error("expected error for duplicate feature name, got nil")
	}
}

func TestLinkNonExistentFeatures(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Try to link features that don't exist
	err := store.Link("f-ghost1", "f-ghost2", EdgeTypeDependsOn)
	if err == nil {
		t.Error("expected error for linking non-existent features, got nil")
	}
}

func TestDeleteDecisionInvalidID(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Try to delete non-existent decision
	err := store.DeleteDecision("d-nonexistent")
	if err == nil {
		t.Error("expected error for deleting non-existent decision, got nil")
	}
}

func TestUpdateFeatureNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Try to update non-existent feature
	err := store.UpdateFeature(Feature{ID: "f-ghost", Name: "Ghost", OneLiner: "Boo"})
	if err == nil {
		t.Error("expected error for updating non-existent feature, got nil")
	}
}

func TestUpdateDecisionNotFound(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Try to update non-existent decision
	err := store.UpdateDecision(Decision{ID: "d-ghost", Title: "Ghost", Summary: "Boo"})
	if err == nil {
		t.Error("expected error for updating non-existent decision, got nil")
	}
}

func TestUnlinkNonExistentRelationship(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.CreateFeature(Feature{ID: "f-orphan1", Name: "Orphan1", OneLiner: "Test"})
	store.CreateFeature(Feature{ID: "f-orphan2", Name: "Orphan2", OneLiner: "Test"})

	// Try to unlink relationship that doesn't exist
	err := store.Unlink("f-orphan1", "f-orphan2", EdgeTypeDependsOn)
	if err == nil {
		t.Error("expected error for unlinking non-existent relationship, got nil")
	}
}

func TestLinkInvalidRelationType(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	store.CreateFeature(Feature{ID: "f-type1", Name: "Type1", OneLiner: "Test"})
	store.CreateFeature(Feature{ID: "f-type2", Name: "Type2", OneLiner: "Test"})

	// Try to link with invalid relation type
	err := store.Link("f-type1", "f-type2", "invalid_type")
	if err == nil {
		t.Error("expected error for invalid relation type, got nil")
	}
}
