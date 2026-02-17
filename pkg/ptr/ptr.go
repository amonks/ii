package ptr

//go:fix inline
func String(s string) *string {
	return new(s)
}
