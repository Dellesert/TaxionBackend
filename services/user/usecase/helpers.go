package usecase

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// stringValue returns the value of a string pointer, or empty string if nil
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
