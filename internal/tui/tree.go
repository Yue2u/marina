package tui

import "github.com/Yue2u/marina/internal/core"

type nodeKind int

const (
	kindFolder nodeKind = iota
	kindHost
)

type treeNode struct {
	kind  nodeKind
	id    string
	label string
	depth int
}

func buildTree(folders []core.Folder, hosts []core.Host, parentID *string, depth int, expanded map[string]bool) []treeNode {
	var nodes []treeNode
	for _, f := range folders {
		if ptrEq(f.ParentID, parentID) {
			nodes = append(nodes, treeNode{kindFolder, f.ID, f.Name, depth})
			if expanded[f.ID] {
				nodes = append(nodes, buildTree(folders, hosts, &f.ID, depth+1, expanded)...)
			}
		}
	}
	for _, h := range hosts {
		if ptrEq(h.FolderID, parentID) {
			nodes = append(nodes, treeNode{kindHost, h.ID, h.Label, depth})
		}
	}
	return nodes
}

func ptrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
