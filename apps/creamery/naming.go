package creamery

import "strings"

// FriendlyIngredientName converts catalog keys (snake_case) into display names.
func FriendlyIngredientName(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "Ingredient"
	}
	words := strings.FieldsFunc(slug, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	if len(words) == 0 {
		return "Ingredient"
	}
	for i, w := range words {
		if w == "" {
			continue
		}
		lower := strings.ToLower(w)
		words[i] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(words, " ")
}
