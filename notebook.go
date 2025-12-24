package creamery

import (
	"fmt"
	"strings"
)

const notebookIntervalEpsilon = 1e-4

// Notebook reproduces the exploratory workflow that used to live in workflow_test.go.
// It prints progress and returns an error if any of the solver constraints fail.
func Notebook() error {
	fmt.Println("=" + strings.Repeat("=", 60))
	fmt.Println("FULL WORKFLOW: Label → Formulation → Recipe")
	fmt.Println("=" + strings.Repeat("=", 60))

	fmt.Println("\n## STEP 1: Analyze the label")
	fmt.Println()

	labelDef, ok := LabelDefinitionByKey(LabelHaagenDazsVanilla)
	if !ok {
		return fmt.Errorf("label %q missing from catalog", LabelHaagenDazsVanilla)
	}
	label := labelDef.Label

	fmt.Printf("%s label:\n", labelDef.Name)
	if len(labelDef.DisplayNames) > 0 {
		fmt.Printf("  Ingredients: %s\n", strings.Join(labelDef.DisplayNames, ", "))
	}
	fmt.Printf("  Per %.0fg: %.0fg fat, %.0fg protein, %.0fg sugar\n",
		label.ServingSize, label.TotalFat, label.Protein, label.Sugars)

	target := label.ToTarget()
	target.POD = Interval{}
	target.PAC = Interval{}
	targetFractions := target.Components

	fmt.Println()
	fmt.Println("Derived formulation targets (with FDA rounding uncertainty):")
	fmt.Printf("  Fat:   %s\n", target.FatInterval())
	fmt.Printf("  MSNF:  %s\n", target.MSNFInterval())
	fmt.Printf("  Added sugar: %s\n", target.AddedSugarsInterval())
	fmt.Printf("  Water: %s (derived)\n", target.WaterInterval())

	fmt.Println()
	fmt.Println("### Interpreting the ingredient list")
	fmt.Println("Using the label's ingredient order to constrain possible formulations:")

	labelSpecs := labelDef.IngredientSpecs
	labelProblem := NewProblem(labelSpecs, target)
	labelProblem.OrderConstraints = true

	labelSolver, err := NewSolver(labelProblem)
	if err != nil {
		return fmt.Errorf("label solver creation failed: %w", err)
	}

	labelSamples, err := labelSolver.Sample(50, true, nil)
	if err != nil {
		return fmt.Errorf("label sampling failed: %w", err)
	}
	if len(labelSamples) == 0 {
		fmt.Println("  No formulation satisfies both the label macros and ingredient order.")
	} else {
		fmt.Println("  Candidate formulations:")
		limit := minInt(2, len(labelSamples))
		for i := 0; i < limit; i++ {
			s := labelSamples[i]
			fmt.Printf("    Option %d:\n", i+1)
			for _, spec := range labelProblem.Specs() {
				if w := s.Weights[spec.ID]; w > 0.005 {
					fmt.Printf("      %-18s %5.1f%%\n", spec.Name+":", w*100)
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("## STEP 2: Create recipe with ingredients on hand")
	fmt.Println()

	myCream := HeavyCream
	myCream.Name = "Cream (36%)"
	myCream.ID = NewIngredientID(myCream.Name)
	myCream.Profile.Name = myCream.Name
	myCream.Profile.ID = myCream.ID

	myMilk := WholeMilk
	myMilk.Name = "Whole Milk (3.25%)"
	myMilk.ID = NewIngredientID(myMilk.Name)
	myMilk.Profile.Name = myMilk.Name
	myMilk.Profile.ID = myMilk.ID

	myNFDM := NonfatDryMilk
	myNFDM.Name = "Skim Milk Powder"
	myNFDM.ID = NewIngredientID(myNFDM.Name)
	myNFDM.Profile.Name = myNFDM.Name
	myNFDM.Profile.ID = myNFDM.ID

	myEggYolks := EggYolks
	myEggYolks.Name = "Egg Yolks"
	myEggYolks.ID = NewIngredientID(myEggYolks.Name)
	myEggYolks.Profile.Name = myEggYolks.Name
	myEggYolks.Profile.ID = myEggYolks.ID

	catalog := DefaultIngredientCatalog()
	mySucrose, err := catalogSpec(catalog, "sucrose", "Sucrose")
	if err != nil {
		return err
	}
	myDextrose, err := catalogSpec(catalog, "dextrose", "Dextrose")
	if err != nil {
		return err
	}

	fmt.Println("Ingredients on hand:")
	fmt.Printf("  %-20s Fat: %.1f%%, MSNF: %.1f%%\n", myCream.Name, myCream.Profile.Components.Fat.Mid()*100, myCream.Profile.MSNFInterval().Mid()*100)
	fmt.Printf("  %-20s Fat: %.2f%%, MSNF: %.2f%%\n", myMilk.Name, myMilk.Profile.Components.Fat.Mid()*100, myMilk.Profile.MSNFInterval().Mid()*100)
	fmt.Printf("  %-20s Fat: %.1f%%, MSNF: %.0f%%\n", myNFDM.Name, myNFDM.Profile.Components.Fat.Mid()*100, myNFDM.Profile.MSNFInterval().Mid()*100)
	fmt.Printf("  %-20s Fat: %.1f%%, Protein: %.1f%%\n", myEggYolks.Name, myEggYolks.Profile.Components.Fat.Mid()*100, myEggYolks.Profile.Components.Protein.Mid()*100)
	printSugar := func(spec IngredientDefinition, suffix string) {
		fmt.Printf("  %-20s Sugar: %.0f%%, POD: %.0f, PAC: %.0f%s\n",
			spec.Name,
			spec.Profile.AddedSugarsInterval().Mid()*100,
			spec.Profile.PODInterval().Mid(),
			spec.Profile.PACInterval().Mid(),
			suffix,
		)
	}
	printSugar(mySucrose, "")
	printSugar(myDextrose, " (less sweet, more softening)")

	specs := []IngredientDefinition{
		myCream,
		myMilk,
		myNFDM,
		myEggYolks,
		mySucrose,
		myDextrose,
	}

	problem := NewProblem(specs, target)
	solver, err := NewSolver(problem)
	if err != nil {
		return fmt.Errorf("solver creation failed: %w", err)
	}

	bounds, err := solver.FindBounds()
	if err != nil {
		return fmt.Errorf("bounds failed: %w", err)
	}

	fmt.Println()
	if !bounds.Feasible {
		fmt.Println("WARNING: No feasible recipe with these ingredients!")
		fmt.Println("The target formulation cannot be achieved.")
		return nil
	}

	fmt.Println("Feasible ingredient ranges:")
	for _, spec := range specs {
		r := bounds.WeightRanges[spec.ID]
		fmt.Printf("  %-20s %5.1f%% - %5.1f%%\n", spec.Name, r.Lo*100, r.Hi*100)
	}

	recipePref := DefaultRecipePreference()
	bestRecipe, err := solver.OptimizeWithPreference(recipePref)
	if err != nil {
		return fmt.Errorf("failed to optimize recipe: %w", err)
	}

	fmt.Println()
	fmt.Println("## STEP 3: Best recipe (multi-curve preference)")
	fmt.Println()

	fmt.Println("Ingredients:")
	for _, spec := range specs {
		w := bestRecipe.Weights[spec.ID]
		if w > 0.001 {
			fmt.Printf("  %-20s %5.1f%%\n", spec.Name, w*100)
		}
	}

	fmt.Println()
	fmt.Println("Achieved composition:")
	fmt.Printf("  Fat:   %.2f%%  (target: %s)\n", bestRecipe.Achieved.Fat.Mid()*100, target.FatInterval())
	fmt.Printf("  MSNF:  %.2f%%  (target: %s)\n", bestRecipe.Achieved.MSNF.Mid()*100, target.MSNFInterval())
	fmt.Printf("  Added sugar: %.2f%%  (target: %s)\n", bestRecipe.Achieved.AddedSugarsInterval().Mid()*100, target.AddedSugarsInterval())
	fmt.Printf("  Water: %.2f%%\n", bestRecipe.Achieved.Water.Mid()*100)
	if err := ensureFractionsWithinTarget(targetFractions, bestRecipe.Achieved, "Best recipe"); err != nil {
		return err
	}

	sweetener := AnalyzeSweeteners(bestRecipe, specs)
	fmt.Println()
	fmt.Println("Sweetener analysis:")
	fmt.Printf("  POD: %.1f (equivalent to %.1f%% sucrose)\n", sweetener.TotalPOD, sweetener.EquivalentSucrose()*100)
	fmt.Printf("  PAC: %.1f → %s\n", sweetener.TotalPAC, sweetener.RelativeSoftness())
	if err := ensureSweetenersMatchTarget(target, sweetener, "Best recipe"); err != nil {
		return err
	}

	if snapshot, snapErr := bestRecipe.Snapshot(MixOptions{}); snapErr == nil {
		totalPref := recipePref.Score(snapshot)
		viscScore := recipePref.Viscosity.Score(snapshot.ViscosityAtServe)
		sweetPct := snapshot.SweetnessEq / snapshot.TotalMassKg
		sweetScore := recipePref.Sweetness.Score(sweetPct)
		iceScore := recipePref.IceFraction.Score(snapshot.IceFractionAtServe)
		fmt.Println()
		fmt.Println("Texture preview:")
		fmt.Printf("  Viscosity @ serve: %.4f Pa·s (score %.2f)\n", snapshot.ViscosityAtServe, viscScore)
		fmt.Printf("  Sweetness (sucrose eq): %.2f%% (score %.2f)\n", sweetPct*100, sweetScore)
		fmt.Printf("  Ice fraction @ serve : %.1f%% (score %.2f)\n", snapshot.IceFractionAtServe*100, iceScore)
		fmt.Printf("  Overall preference   : %.3f\n", totalPref)
		fmt.Printf("  Overrun estimate : %.1f%%\n", snapshot.OverrunEstimate*100)
		fmt.Printf("  Hardness index   : %.2f\n", snapshot.HardnessIndex)
	}

	batchGrams := 1000.0
	fmt.Println()
	fmt.Printf("For a %.0fg batch:\n", batchGrams)
	for _, spec := range specs {
		w := bestRecipe.Weights[spec.ID]
		if w > 0.001 {
			grams := w * batchGrams
			fmt.Printf("  %-20s %6.1fg\n", spec.Name, grams)
		}
	}

	fmt.Println()
	fmt.Println("## STEP 4: Target texture with PAC constraint")
	fmt.Println()
	fmt.Println("Problem: sucrose alone → PAC ~24 (hard)")
	fmt.Println("         dextrose alone → PAC ~41 (too soft)")
	fmt.Println("Solution: blend them to target PAC 28-32 (firm, scoopable)")
	fmt.Println()

	problem2 := NewProblem(specs, target)
	problem2.Target.PAC = Range(28, 32)

	solver2, err := NewSolver(problem2)
	if err != nil {
		return fmt.Errorf("solver2 creation failed: %w", err)
	}
	bounds2, err := solver2.FindBounds()
	if err != nil {
		return fmt.Errorf("bounds2 failed: %w", err)
	}

	if !bounds2.Feasible {
		fmt.Println("No feasible solution with PAC constraint!")
		return nil
	}

	fmt.Println("With PAC target [28-32]:")
	for _, spec := range specs {
		r := bounds2.WeightRanges[spec.ID]
		if r.Hi > 0.001 {
			fmt.Printf("  %-20s %5.1f%% - %5.1f%%\n", spec.Name, r.Lo*100, r.Hi*100)
		}
	}

	sample2, err := solver2.FindSolution()
	if err != nil {
		return fmt.Errorf("PAC solver failed: %w", err)
	}
	sweetener2 := AnalyzeSweeteners(sample2, specs)

	fmt.Println()
	fmt.Println("PAC-optimized recipe (1000g batch):")
	for _, spec := range specs {
		w := sample2.Weights[spec.ID]
		if w > 0.001 {
			fmt.Printf("  %-20s %6.1fg\n", spec.Name, w*1000)
		}
	}
	fmt.Println()
	fmt.Printf("Result: POD=%.1f, PAC=%.1f → %s\n",
		sweetener2.TotalPOD, sweetener2.TotalPAC, sweetener2.RelativeSoftness())
	if err := ensureSweetenersMatchTarget(problem2.Target, sweetener2, "PAC-optimized recipe"); err != nil {
		return err
	}
	if err := ensureIntervalContains(problem2.Target.PAC, sweetener2.TotalPAC, "PAC-optimized recipe PAC"); err != nil {
		return err
	}
	return nil
}

func ensureFractionsWithinTarget(target ComponentFractions, achieved ComponentFractions, context string) error {
	check := func(name string, interval Interval, got float64) error {
		return ensureIntervalContains(interval, got, fmt.Sprintf("%s %s", context, name))
	}
	if err := check("fat", target.Fat, achieved.Fat.Mid()); err != nil {
		return err
	}
	if err := check("msnf", target.EffectiveMSNF(), achieved.EffectiveMSNF().Mid()); err != nil {
		return err
	}
	if err := check("added sugar", target.AddedSugarsInterval(), achieved.AddedSugarsInterval().Mid()); err != nil {
		return err
	}
	if err := check("other solids", target.OtherSolids, achieved.OtherSolids.Mid()); err != nil {
		return err
	}
	return nil
}

func ensureSweetenersMatchTarget(target FormulationTarget, sweet SweetenerAnalysis, context string) error {
	if target.HasPOD() {
		if err := ensureIntervalContains(target.POD, sweet.TotalPOD, context+" POD"); err != nil {
			return err
		}
	}
	if target.HasPAC() {
		if err := ensureIntervalContains(target.PAC, sweet.TotalPAC, context+" PAC"); err != nil {
			return err
		}
	}
	return nil
}

func ensureIntervalContains(interval Interval, value float64, label string) error {
	if interval.Lo == 0 && interval.Hi == 0 {
		return nil
	}
	if value < interval.Lo-notebookIntervalEpsilon || value > interval.Hi+notebookIntervalEpsilon {
		return fmt.Errorf("%s = %.4f outside interval %s", label, value, interval.StringAbs())
	}
	return nil
}

func catalogSpec(catalog IngredientCatalog, key, name string) (IngredientDefinition, error) {
	inst, ok := catalog.InstanceByKey(key)
	if !ok || inst.Definition == nil {
		return IngredientDefinition{}, fmt.Errorf("catalog missing %s entry", key)
	}
	spec := *inst.Definition
	spec.Name = name
	spec.ID = NewIngredientID(spec.Name)
	spec.Profile.Name = spec.Name
	spec.Profile.ID = spec.ID
	return spec, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
