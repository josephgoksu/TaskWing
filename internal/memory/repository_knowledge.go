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
	return r.db.ListNodes(filter)
}

// ListNodesFiltered returns nodes matching the given filter criteria.
// This is the preferred method for workspace-aware queries.
func (r *Repository) ListNodesFiltered(filter NodeFilter) ([]Node, error) {
	return r.db.ListNodesFiltered(filter)
}

func (r *Repository) GetNode(id string) (*Node, error) {
	return r.db.GetNode(id)
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

func (r *Repository) UpsertNodeBySummary(n Node) error {
	return r.db.UpsertNodeBySummary(n)
}

// ClearAllKnowledge removes all nodes, edges, features, decisions, and patterns.
func (r *Repository) ClearAllKnowledge() error {
	return r.db.ClearAllKnowledge()
}

// CreatePattern stores a new pattern in the DB.
func (r *Repository) CreatePattern(p Pattern) error {
	return r.db.CreatePattern(p)
}

// === FTS5 Hybrid Search ===

// ListNodesWithEmbeddings returns all nodes with embeddings in a single query.
func (r *Repository) ListNodesWithEmbeddings() ([]Node, error) {
	return r.db.ListNodesWithEmbeddings()
}

// ListNodesWithEmbeddingsFiltered returns nodes with embeddings matching the filter.
func (r *Repository) ListNodesWithEmbeddingsFiltered(filter NodeFilter) ([]Node, error) {
	return r.db.ListNodesWithEmbeddingsFiltered(filter)
}

// SearchFTS performs full-text search using FTS5 with BM25 ranking.
func (r *Repository) SearchFTS(query string, limit int) ([]FTSResult, error) {
	return r.db.SearchFTS(query, limit)
}

// SearchFTSFiltered performs full-text search with workspace filtering.
func (r *Repository) SearchFTSFiltered(query string, limit int, filter NodeFilter) ([]FTSResult, error) {
	return r.db.SearchFTSFiltered(query, limit, filter)
}

// RebuildFTS rebuilds the FTS5 index from existing nodes.
func (r *Repository) RebuildFTS() error {
	return r.db.RebuildFTS()
}

// GetEmbeddingStats returns statistics about embeddings in the database.
func (r *Repository) GetEmbeddingStats() (*EmbeddingStats, error) {
	return r.db.GetEmbeddingStats()
}
