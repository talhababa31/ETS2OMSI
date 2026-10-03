package graph

import (
	"path/filepath"
	"sort"
	"strings"
)

type NodeType string

const (
	NodeDefinition NodeType = "definition"
	NodeInclude    NodeType = "include"
	NodeModel      NodeType = "model"
	NodeGeometry   NodeType = "geometry"
	NodeCollision  NodeType = "collision"
	NodeMaterial   NodeType = "material"
	NodeTextureObj NodeType = "texture_object"
	NodeTexture    NodeType = "texture"
	NodeAnimation  NodeType = "animation"
	NodeUnknown    NodeType = "unknown"
)

type Node struct {
	Path   string   `json:"path"`
	Type   NodeType `json:"type"`
	Exists bool     `json:"exists"`
	Shared bool     `json:"shared,omitempty"`
}

type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
	Basis  string `json:"basis,omitempty"` // explicit field/include/documented convention
}

type Graph struct {
	Nodes map[string]Node `json:"-"`
	Edges []Edge          `json:"edges"`
}

func New() *Graph { return &Graph{Nodes: map[string]Node{}} }

func (g *Graph) AddNode(path string, exists bool) {
	p := Normalize(path)
	if p == "" {
		return
	}
	old, ok := g.Nodes[p]
	n := Node{Path: p, Type: TypeForPath(p), Exists: exists}
	if ok {
		n.Shared = old.Shared
		n.Exists = old.Exists || exists
	}
	g.Nodes[p] = n
}

func (g *Graph) AddEdge(from, to, reason, basis string, exists bool) {
	f, t := Normalize(from), Normalize(to)
	if f == "" || t == "" {
		return
	}
	g.AddNode(f, true)
	g.AddNode(t, exists)
	for _, e := range g.Edges {
		if e.From == f && e.To == t && e.Reason == reason {
			return
		}
	}
	g.Edges = append(g.Edges, Edge{From: f, To: t, Reason: reason, Basis: basis})
}

func (g *Graph) SortedNodes() []Node {
	out := make([]Node, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func TypeForPath(p string) NodeType {
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".sii", ".sui":
		return NodeDefinition
	case ".pmd":
		return NodeModel
	case ".pmg", ".pim":
		return NodeGeometry
	case ".pmc", ".pic":
		return NodeCollision
	case ".pma":
		return NodeAnimation
	case ".mat", ".pit":
		return NodeMaterial
	case ".tobj":
		return NodeTextureObj
	case ".dds", ".png", ".tga", ".jpg", ".jpeg":
		return NodeTexture
	default:
		return NodeUnknown
	}
}

func Normalize(p string) string {
	p = strings.TrimSpace(strings.Trim(p, "\"'"))
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	// path.Clean on slash paths without importing path to keep behavior explicit.
	parts := strings.Split(p, "/")
	stack := make([]string, 0, len(parts))
	for _, s := range parts {
		if s == "" || s == "." {
			continue
		}
		if s == ".." {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			continue
		}
		stack = append(stack, s)
	}
	return "/" + strings.Join(stack, "/")
}
