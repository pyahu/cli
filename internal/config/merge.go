package config

import "gopkg.in/yaml.v3"

func mergeDocuments(base *yaml.Node, overlay *yaml.Node) *yaml.Node {
	if documentEmpty(base) {
		return cloneNode(overlay)
	}
	if documentEmpty(overlay) {
		return cloneNode(base)
	}

	merged := cloneNode(base)
	merged.Content[0] = mergeNode(merged.Content[0], overlay.Content[0])
	return merged
}

func mergeNode(base *yaml.Node, overlay *yaml.Node) *yaml.Node {
	if base == nil || base.Kind == 0 {
		return cloneNode(overlay)
	}
	if overlay == nil || overlay.Kind == 0 {
		return cloneNode(base)
	}
	if base.Kind == yaml.MappingNode && overlay.Kind == yaml.MappingNode {
		return mergeMapping(base, overlay)
	}
	return cloneNode(overlay)
}

func mergeMapping(base *yaml.Node, overlay *yaml.Node) *yaml.Node {
	merged := cloneNode(base)
	for i := 0; i < len(overlay.Content); i += 2 {
		overlayKey := overlay.Content[i]
		overlayValue := overlay.Content[i+1]
		baseIndex := mappingValueIndex(merged, overlayKey.Value)
		if baseIndex == -1 {
			merged.Content = append(merged.Content, cloneNode(overlayKey), cloneNode(overlayValue))
			continue
		}
		merged.Content[baseIndex] = mergeNode(merged.Content[baseIndex], overlayValue)
	}
	return merged
}

func mappingValueIndex(node *yaml.Node, key string) int {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i + 1
		}
	}
	return -1
}

func documentEmpty(node *yaml.Node) bool {
	return node == nil || node.Kind == 0 || len(node.Content) == 0 || node.Content[0].Kind == 0
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			clone.Content[i] = cloneNode(child)
		}
	}
	return &clone
}
