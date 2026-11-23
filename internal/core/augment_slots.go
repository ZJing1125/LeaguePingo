package core

// Preserve selection slots, including gaps; zero denotes an unrecorded slot.
func augmentSlots(values ...*int) []int {
	result := make([]int, len(values))
	found := false
	for i, value := range values {
		if value != nil && *value > 0 {
			result[i] = *value
			found = true
		}
	}
	if !found {
		return nil
	}
	return result
}
