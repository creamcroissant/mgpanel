package tools

import "fmt"

// parseID extracts an int64 "id" from a params map.
func parseID(params any) (int64, error) {
	m, ok := params.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("params must be object")
	}
	id, ok := m["id"].(float64)
	if !ok {
		return 0, fmt.Errorf("id is required")
	}
	return int64(id), nil
}
