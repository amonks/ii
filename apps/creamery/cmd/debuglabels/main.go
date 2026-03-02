package main

import (
	"fmt"

	"github.com/amonks/creamery"
)

func main() {
	label, ok := creamery.FDALabelByKey(creamery.LabelJenisSweetCream)
	if !ok {
		panic("missing scenario")
	}
	fmt.Printf("Label: %s\n", label.Name)
	fmt.Printf("Pint Mass: %.1fg\n", label.PintMassGrams)
	fmt.Printf("Ingredients (%d)\n", len(label.Ingredients))
	for _, ing := range label.Ingredients {
		fmt.Printf("- %s\n", ing.ID)
	}
	fmt.Printf("Groups: %d\n", len(label.Groups))
	for _, g := range label.Groups {
		fmt.Printf("Group %s: %v\n", g.Name, g.Members)
		if len(g.FractionBounds) > 0 {
			fmt.Printf("  Bounds:\n")
			for id, bound := range g.FractionBounds {
				fmt.Printf("    %s: [%f,%f]\n", id, bound.Lo, bound.Hi)
			}
		}
	}
}
