package app

// panes lists the terminals of a tab in the order its splits read: the
// first branch, then the second.
func (t *termTab) panes() []*termPane {
	if t == nil {
		return nil
	}
	var out []*termPane
	t.root.each(func(p *termPane) { out = append(out, p) })
	return out
}

// each calls fn for every pane of a split tree, left or top first.
func (n *termNode) each(fn func(*termPane)) {
	if n == nil {
		return
	}
	if n.pane != nil {
		fn(n.pane)
		return
	}
	n.first.each(fn)
	n.second.each(fn)
}

// split turns the leaf of anchor into two branches: anchor and add,
// across when vertical is false and down when it is true.
func (n *termNode) split(anchor, add *termPane, vertical bool, id int) bool {
	if n == nil {
		return false
	}
	if n.pane == anchor {
		n.id = id
		n.first = &termNode{id: anchor.id, pane: anchor}
		n.second = &termNode{id: add.id, pane: add}
		n.pane = nil
		n.vertical = vertical
		n.ratio = 50
		return true
	}
	if n.pane != nil {
		return false
	}
	return n.first.split(anchor, add, vertical, id) || n.second.split(anchor, add, vertical, id)
}

// equalize puts every split of the tree at half, as Rex's equalize.
func (n *termNode) equalize() {
	if n == nil || n.pane != nil {
		return
	}
	n.ratio = 50
	n.first.equalize()
	n.second.equalize()
}

// remove takes p out of the tree, which returns the new root: the
// remaining branch where p's branch held only it.
func (n *termNode) remove(p *termPane) (*termNode, bool) {
	if n == nil {
		return nil, false
	}
	if n.pane != nil {
		if n.pane == p {
			return nil, true
		}
		return n, false
	}
	if n.first != nil {
		left, ok := n.first.remove(p)
		if ok {
			if left == nil {
				return n.second, true
			}
			n.first = left
			return n, true
		}
	}
	if n.second != nil {
		right, ok := n.second.remove(p)
		if ok {
			if right == nil {
				return n.first, true
			}
			n.second = right
			return n, true
		}
	}
	return n, false
}
