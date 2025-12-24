package main

import (
    "fmt"

    "github.com/amonks/creamery"
)

func main() {
    def, ok := creamery.LabelScenarioByKey(creamery.LabelJenisSweetCream)
    if !ok {
        panic("missing scenario")
    }
    fmt.Printf("Lots (%d)\n", len(def.Lots))
    for _, lot := range def.Lots {
        if lot.Definition != nil {
            fmt.Printf("- %s (%s)\n", lot.Definition.ID, lot.Definition.Name)
        }
    }
    fmt.Printf("Presence: %v\n", def.Presence)
    fmt.Printf("Groups: %d\n", len(def.Groups))
    for _, g := range def.Groups {
        fmt.Printf("Group %s: %v\n", g.Name, g.Keys)
        if len(g.FractionBounds) > 0 {
            fmt.Printf("  Bounds:\n")
            for id, iv := range g.FractionBounds {
                fmt.Printf("    %s: [%f,%f]\n", id, iv.Lo, iv.Hi)
            }
        }
    }
}
