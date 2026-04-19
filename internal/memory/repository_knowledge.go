package memory

// === Knowledge Graph & Search ===

// LinkNodes creates an edge between two nodes in the knowledge graph.
func (r *Repository) LinkNodes(from, to, relation string, confidence float64, properties map[string]any) error {
	return r.db.LinkNodes(from, to, relation, confidence, properties)
}

// GetAllNodeEdges returns all edges in the knowledge graph.
func (r *Repository) GetAllNodeEdges() ([]NodeEdge, error) {
	return r.db.GetAllNodeEdges()
}

// GetNodeEdges returns all edges connected to a specific node.
func (r *Repository) GetNodeEdges(nodeID string) ([]NodeEdge, error) {
	return r.db.GetNodeEdges(nodeID)
}

// === Node Access ===

func (r *Repository) ListNodes(filter string) ([]Node, error) {
	nodes, err := r.db.ListNodes(filter)
	if err != nil {
		return nil, err
	}
	if r.global != nil {
		globalNodes, err := r.global.db.ListNodes(filter)
		if err == nil {
			nodes = deduplicateNodes(nodes, globalNodes)
		}
	}
	return nodes, nil
}

// ListNodesFiltered returns nodes matching the given filter criteria.
// This is the preferred method for workspace-aware queries.
func (r *Repository) ListNodesFiltered(filter NodeFilter) ([]Node, error) {
	nodes, err := r.db.ListNodesFiltered(filter)
	if err != nil {
		return nil, err
	}
	if r.global != nil {
		globalNodes, err := r.global.db.ListNodesFiltered(filter)
		if err == nil {
			nodes = deduplicateNodes(nodes, globalNodes)
		}
	}
	return nodes, nil
}

func (r *Repository) GetNode(id string) (*Node, error) {
	node, err := r.db.GetNode(id)
	if err == nil && node != nil {
		return node, nil
	}
	if r.global != nil {
		return r.global.db.GetNode(id)
	}
	return node, err
}

func (r *Repository) CreateNode(n *Node) error {
	return r.db.CreateNode(n)
}

func (r *Repository) UpdateNode(id, content, nodeType, summary string) error {
	return r.db.UpdateNode(id, content, nodeType, summary)
}

func (r *Repository) UpdateNodeEmbedding(id string, embedding []float32) error {
	return r.db.UpdateNodeEmbedding(id, embedding)
}

func (r *Repository) UpdateNodeWorkspace(id, workspace string) error {
	return r.db.UpdateNodeWorkspace(id, workspace)
}

func (r *Repository) DeleteNode(id string) error {
	return r.db.DeleteNode(id)
}

func (r *Repository) DeleteNodesByType(nodeType string) (int64, error) {
	return r.db.DeleteNodesByType(nodeType)
}

func (r *Repository) DeleteNodesByAgent(agent string) error {
	return r.db.DeleteNodesByAgent(agent)
}

func (r *Repository) DeleteNodesByFiles(agent string, filePaths []string) error {
	return r.db.DeleteNodesByFiles(agent, filePaths)
}

// GetNodesByFiles returns nodes from a specific agent that reference any of the given files.
func (r *Repository) GetNodesByFiles(agent string, filePaths []string) ([]Node, error) {
	return r.db.GetNodesByFiles(agent, filePaths)
}

func (r *Repository) MarkNodesStaleByAgent(agent string, workspaces ...string) error {
	return r.db.MarkNodesStaleByAgent(agent, workspaces...)
}

func (r *Repository) ReconcileStaleNodes(agent string, workspaces ...string) (int, int, error) {
	return r.db.ReconcileStaleNodes(agent, workspaces...)
}

func (r *Repository) UpsertNodeBySummary(n Node) error {
	return r.db.UpsertNodeBySummary(n)
}

// ClearAllKnowledge removes all nodes, edges, features, decisions, and patterns.
func (r *Repository) ClearAllKnowledge() error {
	return r.db.ClearAllKnowledge()
}

// === FTS5 Hybrid Search ===

// ListNodesWithEmbeddings returns all nodes with embeddings in a single query.
func (r *Repository) ListNodesWithEmbeddings() ([]Node, error) {
	nodes, err := r.db.ListNodesWithEmbeddings()
	if err != nil {
		return nil, err
	}
	if r.global != nil {
		globalNodes, err := r.global.db.ListNodesWithEmbeddings()
		if err == nil {
			nodes = deduplicateNodes(nodes, globalNodes)
		}
	}
	return nodes, nil
}

// ListNodesWithEmbeddingsFiltered returns nodes with embeddings matching the filter.
func (r *Repository) ListNodesWithEmbeddingsFiltered(filter NodeFilter) ([]Node, error) {
	nodes, err := r.db.ListNodesWithEmbeddingsFiltered(filter)
	if err != nil {
		return nil, err
	}
	if r.global != nil {
		globalNodes, err := r.global.db.ListNodesWithEmbeddingsFiltered(filter)
		if err == nil {
			nodes = deduplicateNodes(nodes, globalNodes)
		}
	}
	return nodes, nil
}

// SearchFTS performs full-text search using FTS5 with BM25 ranking.
func (r *Repository) SearchFTS(query string, limit int) ([]FTSResult, error) {
	results, err := r.db.SearchFTS(query, limit)
	if err != nil {
		return nil, err
	}
	if r.global != nil {
		globalResults, err := r.global.db.SearchFTS(query, limit)
		if err == nil {
			results = deduplicateFTSResults(results, globalResults)
		}
	}
	return results, nil
}

// SearchFTSFiltered performs full-text search with workspace filtering.
func (r *Repository) SearchFTSFiltered(query string, limit int, filter NodeFilter) ([]FTSResult, error) {
	results, err := r.db.SearchFTSFiltered(query, limit, filter)
	if err != nil {
		return nil, err
	}
	if r.global != nil {
		globalResults, err := r.global.db.SearchFTSFiltered(query, limit, filter)
		if err == nil {
			results = deduplicateFTSResults(results, globalResults)
		}
	}
	return results, nil
}

// RebuildFTS rebuilds the FTS5 index from existing nodes.
func (r *Repository) RebuildFTS() error {
	return r.db.RebuildFTS()
}

// GetEmbeddingStats returns statistics about embeddings in the database.
func (r *Repository) GetEmbeddingStats() (*EmbeddingStats, error) {
	return r.db.GetEmbeddingStats()
}

// deduplicateNodes appends globalNodes to existing, skipping any with duplicate summaries.
func deduplicateNodes(existing, globalNodes []Node) []Node {
	seen := make(map[string]bool, len(existing))
	for _, n := range existing {
		if n.Summary != "" {
			seen[n.Summary] = true
		}
	}
	for _, n := range globalNodes {
		if n.Summary != "" && seen[n.Summary] {
			continue
		}
		existing = append(existing, n)
	}
	return existing
}

// deduplicateFTSResults appends global results to existing, skipping duplicates by node summary.
func deduplicateFTSResults(existing, globalResults []FTSResult) []FTSResult {
	seen := make(map[string]bool, len(existing))
	for _, r := range existing {
		if r.Node.Summary != "" {
			seen[r.Node.Summary] = true
		}
	}
	for _, r := range globalResults {
		if r.Node.Summary != "" && seen[r.Node.Summary] {
			continue
		}
		existing = append(existing, r)
	}
	return existing
}
