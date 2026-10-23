package source

// DomainMap assigns every category its topic domain: the first branch —
// a non-hidden child of the wiki's root content category — that reaches it
// walking the category tree breadth-first. Hidden categories are
// maintenance plumbing and take no part in the walk; a category no branch
// reaches (orphaned, or under an admin tree) gets no domain. Breadth-first
// with the branches seeded together means a category claimed by two
// branches goes to the one fewer steps away, ties broken by seeding order
// (the API's page order, stable for a given wiki).
func DomainMap(pages []CategoryPage, root string) map[string]string {
	hidden := make(map[string]bool, len(pages))
	for _, p := range pages {
		if p.Hidden {
			hidden[p.Name] = true
		}
	}
	// The pages carry child→parents edges; the walk needs parent→children.
	children := make(map[string][]string, len(pages))
	var queue []string
	domain := make(map[string]string)
	for _, p := range pages {
		if p.Hidden {
			continue
		}
		for _, parent := range p.Parents {
			if parent == root && domain[p.Name] == "" {
				domain[p.Name] = p.Name
				queue = append(queue, p.Name)
			}
			children[parent] = append(children[parent], p.Name)
		}
	}
	for len(queue) > 0 {
		cat := queue[0]
		queue = queue[1:]
		for _, child := range children[cat] {
			if domain[child] == "" && !hidden[child] {
				domain[child] = domain[cat]
				queue = append(queue, child)
			}
		}
	}
	return domain
}
