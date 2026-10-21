package main

import "fmt"

// landedBindingFor matches the panel-round encoding written at merge time.
// Metadata read failures propagate: an unavailable binding must not allow writes.
func landedBindingFor(readMeta func(string) (map[string]interface{}, error), beadID, slug string, round int) (bool, error) {
	if beadID == "" {
		return false, nil
	}
	meta, err := readMeta(beadID)
	if err != nil {
		return false, err
	}
	binding, _ := meta["mindspec_landed_review_panel"].(string)
	return binding == fmt.Sprintf("%s@r%d", slug, round), nil
}
