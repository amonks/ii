package creamery

// ingredientBatch represents a concrete ingredient lot (per-kilogram basis).
type ingredientBatch struct {
	ID                 IngredientID
	Name               string
	Water              float64
	Fat                float64
	TransFat           float64
	SaturatedFat       float64
	SaturatedFatMin    float64
	SaturatedFatMax    float64
	Protein            float64
	Lactose            float64
	LactoseMin         float64
	LactoseMax         float64
	Sucrose            float64
	Glucose            float64
	Fructose           float64
	Maltodextrin       float64
	Polyols            float64
	Ash                float64
	OtherSolids        float64
	Cost               float64
	OsmoticCoeff       float64
	VHFactor           float64
	WaterBinding       float64
	EffectiveMW        float64
	MaltodextrinDP     float64
	PolyolMW           float64
	EmulsifierPower    float64
	Hydrocolloid       bool
	CholesterolMgPerKg float64
	AddedSugars        float64
	AddedSugarsMin     float64
	AddedSugarsMax     float64
}

func cloneBatch(base ingredientBatch, overrides func(*ingredientBatch)) ingredientBatch {
	copy := base
	overrides(&copy)
	if copy.Name == "" {
		copy.Name = base.Name
	}
	copy.ID = NewIngredientID(copy.Name)
	return copy
}

// ingredientBatchTable returns a map of rich ingredient definitions keyed by name.
func IngredientProfileTable() map[string]ConstituentProfile {
	batches := ingredientBatchTable()
	profiles := make(map[string]ConstituentProfile, len(batches))
	for key, batch := range batches {
		profiles[key] = batch.ToProfile(key)
	}
	return profiles
}

func ingredientBatchTable() map[string]ingredientBatch {
	table := map[string]ingredientBatch{
		"water": {
			ID:          NewIngredientID("water"),
			Name:        "water",
			Water:       1.0,
			EffectiveMW: mwSucrose,
		},
		"whole_milk": {
			ID:                 NewIngredientID("whole_milk"),
			Name:               "whole_milk",
			Water:              0.873,
			Fat:                0.0325,
			TransFat:           0.0325 * dairyTransFatShare,
			Protein:            0.032,
			Lactose:            0.049,
			Ash:                0.0085,
			Cost:               0,
			CholesterolMgPerKg: mgPerKgFrom100g(14.0),
		},
		"cream_fat": {
			ID:                 NewIngredientID("cream_fat"),
			Name:               "cream_fat",
			Water:              0.0,
			Fat:                1.0,
			TransFat:           1.0 * dairyTransFatShare,
			Cost:               0,
			CholesterolMgPerKg: mgPerKgFrom100g(260.0),
		},
		"cream_serum": {
			Name:    "cream_serum",
			Water:   0.907,
			Protein: 0.035,
			Lactose: 0.05,
			Ash:     0.008,
		},
		"milk": {
			Name:               "milk",
			Water:              0.875,
			Fat:                0.032,
			TransFat:           0.032 * dairyTransFatShare,
			Protein:            0.032,
			Lactose:            0.049,
			Ash:                0.0085,
			Cost:               0,
			CholesterolMgPerKg: mgPerKgFrom100g(14.0),
		},
		"skim_milk": {
			Name:               "skim_milk",
			Water:              0.905,
			Fat:                0.002,
			TransFat:           0.002 * dairyTransFatShare,
			Protein:            0.035,
			Lactose:            0.05,
			Ash:                0.008,
			Cost:               0,
			CholesterolMgPerKg: mgPerKgFrom100g(5.0),
		},
		"heavy_cream": {
			Name:               "heavy_cream",
			Water:              0.60,
			Fat:                0.36,
			TransFat:           0.36 * dairyTransFatShare,
			Protein:            0.02,
			Lactose:            0.015,
			Ash:                0.005,
			CholesterolMgPerKg: mgPerKgFrom100g(110.0),
		},
		"whipping_cream": {
			Name:               "whipping_cream",
			Water:              0.66,
			Fat:                0.30,
			TransFat:           0.30 * dairyTransFatShare,
			Protein:            0.02,
			Lactose:            0.015,
			Ash:                0.005,
			CholesterolMgPerKg: mgPerKgFrom100g(100.0),
		},
		"light_cream": {
			Name:               "light_cream",
			Water:              0.76,
			Fat:                0.18,
			TransFat:           0.18 * dairyTransFatShare,
			Protein:            0.03,
			Lactose:            0.02,
			Ash:                0.01,
			CholesterolMgPerKg: mgPerKgFrom100g(60.0),
		},
		"cream36": {
			Name:               "cream36",
			Water:              0.60,
			Fat:                0.36,
			TransFat:           0.36 * dairyTransFatShare,
			Protein:            0.02,
			Lactose:            0.03,
			Ash:                0.01,
			CholesterolMgPerKg: mgPerKgFrom100g(110.0),
		},
		"anhydrous_milk_fat": {
			Name:               "anhydrous_milk_fat",
			Water:              0.0,
			Fat:                0.999,
			TransFat:           0.999 * dairyTransFatShare,
			CholesterolMgPerKg: mgPerKgFrom100g(260.0),
		},
		"butter": {
			Name:               "butter",
			Water:              0.16,
			Fat:                0.80,
			TransFat:           0.80 * dairyTransFatShare,
			Protein:            0.01,
			Lactose:            0.02,
			Ash:                0.01,
			CholesterolMgPerKg: mgPerKgFrom100g(215.0),
		},
		"skim_milk_powder": {
			Name:               "skim_milk_powder",
			Water:              0.04,
			Fat:                0.01,
			TransFat:           0.01 * dairyTransFatShare,
			Protein:            0.35,
			Lactose:            0.49,
			Ash:                0.06,
			OtherSolids:        0.05,
			CholesterolMgPerKg: mgPerKgFrom100g(150.0),
		},
		"wpc80": {
			Name:               "wpc80",
			Water:              0.05,
			Fat:                0.01,
			TransFat:           0.01 * dairyTransFatShare,
			Protein:            0.80,
			Lactose:            0.08,
			Ash:                0.06,
			CholesterolMgPerKg: mgPerKgFrom100g(180.0),
		},
		"egg_yolk": {
			Name:               "egg_yolk",
			Water:              0.52,
			Fat:                0.32,
			TransFat:           0.32 * eggTransFatShare,
			Protein:            0.16,
			Ash:                0.01,
			CholesterolMgPerKg: mgPerKgFrom100g(1085.0),
		},
		"sucrose": {
			Name:        "sucrose",
			Water:       0.0,
			Sucrose:     0.999,
			EffectiveMW: mwSucrose,
		},
		"dextrose": {
			Name:        "dextrose",
			Water:       0.01,
			Glucose:     0.99,
			EffectiveMW: mwGlucose,
		},
		"fructose": {
			Name:        "fructose",
			Water:       0.01,
			Fructose:    0.99,
			EffectiveMW: mwFructose,
		},
		"corn_syrup_42": {
			Name:           "corn_syrup_42",
			Water:          0.20,
			Glucose:        0.15,
			Fructose:       0.05,
			Maltodextrin:   0.58,
			Ash:            0.02,
			MaltodextrinDP: 2.4,
		},
		"tapioca_syrup": {
			Name:           "tapioca_syrup",
			Water:          0.22,
			Glucose:        0.10,
			Fructose:       0.02,
			Maltodextrin:   0.62,
			Ash:            0.02,
			OtherSolids:    0.02,
			MaltodextrinDP: 4.5,
			WaterBinding:   1.0,
			Hydrocolloid:   true,
		},
		"maltodextrin10": {
			Name:           "maltodextrin10",
			Water:          0.05,
			Maltodextrin:   0.93,
			Ash:            0.02,
			MaltodextrinDP: 10.0,
		},
		"inulin": {
			Name:           "inulin",
			Water:          0.05,
			OtherSolids:    0.93,
			Ash:            0.02,
			EffectiveMW:    5000.0,
			WaterBinding:   3.0,
			MaltodextrinDP: 20.0,
		},
		"glycerol": {
			Name:     "glycerol",
			Polyols:  0.995,
			PolyolMW: mwGlycerol,
		},
		"sorbitol": {
			Name:     "sorbitol",
			Polyols:  0.995,
			PolyolMW: mwSorbitol,
		},
		"erythritol": {
			Name:     "erythritol",
			Polyols:  0.995,
			PolyolMW: mwErythritol,
		},
		"guar_gum": {
			Name:         "guar_gum",
			Water:        0.10,
			OtherSolids:  0.90,
			EffectiveMW:  80000.0,
			WaterBinding: 12.0,
			Hydrocolloid: true,
		},
		"tara_gum": {
			Name:         "tara_gum",
			Water:        0.12,
			OtherSolids:  0.88,
			EffectiveMW:  90000.0,
			WaterBinding: 10.0,
			Hydrocolloid: true,
		},
		"locust_bean_gum": {
			Name:         "locust_bean_gum",
			Water:        0.10,
			OtherSolids:  0.90,
			EffectiveMW:  120000.0,
			WaterBinding: 10.0,
			Hydrocolloid: true,
		},
		"carrageenan": {
			Name:         "carrageenan",
			Water:        0.10,
			OtherSolids:  0.90,
			EffectiveMW:  400000.0,
			WaterBinding: 8.0,
			Hydrocolloid: true,
		},
		"xanthan": {
			Name:         "xanthan",
			Water:        0.10,
			OtherSolids:  0.90,
			EffectiveMW:  2000000.0,
			WaterBinding: 15.0,
			Hydrocolloid: true,
		},
		"gelatin": {
			Name:         "gelatin",
			Water:        0.10,
			OtherSolids:  0.90,
			EffectiveMW:  50000.0,
			WaterBinding: 6.0,
			Hydrocolloid: true,
		},
		"mono_diglycerides": {
			Name:            "mono_diglycerides",
			Fat:             0.99,
			EmulsifierPower: 4.0,
		},
		"ps80": {
			Name:            "ps80",
			OtherSolids:     1.0,
			EmulsifierPower: 5.0,
		},
		"lecithin": {
			Name:            "lecithin",
			Water:           0.01,
			Fat:             0.95,
			OtherSolids:     0.04,
			EmulsifierPower: 3.0,
		},
		"vanilla_extract": {
			Name:        "vanilla_extract",
			Water:       0.55,
			OtherSolids: 0.45,
			EffectiveMW: 200.0,
		},
		"vanilla_beans": {
			Name:        "vanilla_beans",
			Water:       0.10,
			Fat:         0.05,
			Protein:     0.05,
			OtherSolids: 0.76,
			Ash:         0.04,
			EffectiveMW: 600.0,
		},
		"potassium_phosphate": {
			Name:  "potassium_phosphate",
			Water: 0.0,
			Ash:   1.0,
		},
		"salt": {
			Name:  "salt",
			Water: 0.0,
			Ash:   1.0,
		},
		"lemon_peel": {
			Name:        "lemon_peel",
			Water:       0.20,
			OtherSolids: 0.70,
			Ash:         0.10,
			EffectiveMW: 800.0,
		},
		"cocoa_powder": {
			Name:        "cocoa_powder",
			Water:       0.03,
			Fat:         0.22,
			Protein:     0.20,
			OtherSolids: 0.45,
			Ash:         0.10,
			EffectiveMW: 500.0,
		},
		"strawberry_puree": {
			Name:     "strawberry_puree",
			Water:    0.90,
			Fructose: 0.06,
			Glucose:  0.02,
			Sucrose:  0.02,
		},
	}

	additional := []struct {
		key       string
		display   string
		fractions ComponentFractions
		configure func(*ingredientBatch)
	}{
		{
			key:     "liquid_sugar",
			display: "Liquid Sugar",
			fractions: ComponentFractions{
				Sucrose: Point(0.67),
			},
		},
		{
			key:     "stabilizer",
			display: "Stabilizer",
			fractions: ComponentFractions{
				OtherSolids: Point(1.0),
			},
			configure: func(b *ingredientBatch) {
				b.Hydrocolloid = true
			},
		},
		{
			key:     "avacream",
			display: "Avacream",
			fractions: ComponentFractions{
				Water:       Point(0.02),
				Fat:         Point(0.08),
				OtherSolids: Point(0.90),
			},
			configure: func(b *ingredientBatch) {
				b.Hydrocolloid = true
				b.EmulsifierPower = 0.4
				b.WaterBinding = 4.0
			},
		},
		{
			key:     "sweetened_condensed_milk",
			display: "Sweetened Condensed Milk",
			fractions: ComponentFractions{
				Fat:     Point(0.085),
				MSNF:    Point(0.20),
				Sucrose: Point(0.445),
			},
			configure: func(b *ingredientBatch) {
				b.AddedSugars = b.Sucrose
				b.AddedSugarsMin = b.Sucrose * (1 - labelPercentEPS)
				b.AddedSugarsMax = b.Sucrose * (1 + labelPercentEPS)
			},
		},
	}

	for _, entry := range additional {
		if _, exists := table[entry.key]; exists {
			continue
		}
		batch := batchFromFractions(entry.key, entry.display, entry.fractions)
		if entry.configure != nil {
			entry.configure(&batch)
		}
		table[entry.key] = batch
	}

	expandSaturated := func(keys []string, lo, hi float64) {
		loAdj, hiAdj := expandFractionBounds(lo, hi)
		mid := 0.5 * (loAdj + hiAdj)
		for _, key := range keys {
			ing, ok := table[key]
			if !ok {
				continue
			}
			if ing.Fat == 0 && ing.SaturatedFat == 0 {
				continue
			}
			ing.SaturatedFat = ing.Fat * mid
			ing.SaturatedFatMin = ing.Fat * loAdj
			ing.SaturatedFatMax = ing.Fat * hiAdj
			table[key] = ing
		}
	}

	setLactoseRange := func(keys []string) {
		for _, key := range keys {
			ing, ok := table[key]
			if !ok {
				continue
			}
			lo := max(0, ing.Lactose*(1-labelPercentEPS))
			hi := ing.Lactose * (1 + labelPercentEPS)
			ing.LactoseMin = lo
			ing.LactoseMax = hi
			table[key] = ing
		}
	}

	added := func(name string, lo, hi float64) {
		ing, ok := table[name]
		if !ok {
			return
		}
		mid := 0.5 * (lo + hi)
		ing.AddedSugars = mid
		ing.AddedSugarsMin = lo
		ing.AddedSugarsMax = hi
		table[name] = ing
	}

	sugarFields := func(name string, fields []func(ingredientBatch) float64) {
		ing, ok := table[name]
		if !ok {
			return
		}
		var total float64
		for _, f := range fields {
			total += f(ing)
		}
		lo := max(0, total*(1-labelPercentEPS))
		hi := total * (1 + labelPercentEPS)
		added(name, lo, hi)
	}

	dairyKeys := []string{
		"whole_milk",
		"cream_fat",
		"skim_milk",
		"heavy_cream",
		"whipping_cream",
		"light_cream",
		"cream36",
		"anhydrous_milk_fat",
		"butter",
		"skim_milk_powder",
		"wpc80",
	}

	expandSaturated(dairyKeys, 0.60, 0.75)
	expandSaturated([]string{"egg_yolk"}, 0.30, 0.40)
	expandSaturated([]string{"vanilla_beans"}, 0.10, 0.20)
	expandSaturated([]string{"cocoa_powder"}, 0.50, 0.70)
	expandSaturated([]string{"mono_diglycerides"}, 0.40, 0.60)
	expandSaturated([]string{"lecithin"}, 0.15, 0.25)

	setLactoseRange(append(dairyKeys, "cream_serum", "milk", "nonfat_milk"))

	sugarFields("sucrose", []func(ingredientBatch) float64{func(i ingredientBatch) float64 { return i.Sucrose }})
	sugarFields("dextrose", []func(ingredientBatch) float64{func(i ingredientBatch) float64 { return i.Glucose }})
	sugarFields("fructose", []func(ingredientBatch) float64{func(i ingredientBatch) float64 { return i.Fructose }})
	sugarFields("corn_syrup_42", []func(ingredientBatch) float64{
		func(i ingredientBatch) float64 { return i.Glucose },
		func(i ingredientBatch) float64 { return i.Fructose },
		func(i ingredientBatch) float64 { return i.Maltodextrin },
	})
	sugarFields("tapioca_syrup", []func(ingredientBatch) float64{
		func(i ingredientBatch) float64 { return i.Glucose },
		func(i ingredientBatch) float64 { return i.Fructose },
		func(i ingredientBatch) float64 { return i.Maltodextrin },
	})
	sugarFields("maltodextrin10", []func(ingredientBatch) float64{func(i ingredientBatch) float64 { return i.Maltodextrin }})

	for name, ing := range table {
		if ing.SaturatedFat == 0 && ing.Fat > 0 {
			ing.SaturatedFat = ing.Fat
		}
		if ing.SaturatedFatMin == 0 {
			ing.SaturatedFatMin = ing.SaturatedFat
		}
		if ing.SaturatedFatMax == 0 {
			ing.SaturatedFatMax = ing.SaturatedFat
		}

		if ing.LactoseMin == 0 && ing.Lactose > 0 {
			ing.LactoseMin = ing.Lactose
		}
		if ing.LactoseMax == 0 && ing.Lactose > 0 {
			ing.LactoseMax = ing.Lactose
		}

		if ing.AddedSugarsMin == 0 {
			ing.AddedSugarsMin = ing.AddedSugars
		}
		if ing.AddedSugarsMax == 0 {
			ing.AddedSugarsMax = ing.AddedSugars
		}

		if ing.OsmoticCoeff == 0 {
			ing.OsmoticCoeff = 1.0
		}
		if ing.VHFactor == 0 {
			ing.VHFactor = 1.0
		}
		if ing.EffectiveMW == 0 {
			ing.EffectiveMW = mwSucrose
		}
		if ing.MaltodextrinDP == 0 {
			ing.MaltodextrinDP = 10.0
		}
		if ing.PolyolMW == 0 {
			ing.PolyolMW = mwSorbitol
		}

		table[name] = ing
	}

	for key, ing := range table {
		if ing.ID == "" {
			name := ing.Name
			if name == "" {
				name = key
			}
			ing.ID = NewIngredientID(name)
		}
		table[key] = ing
	}

	return table
}

func batchFromFractions(key, display string, fractions ComponentFractions) ingredientBatch {
	comps := EnsureWater(fractions)
	return ingredientBatch{
		ID:              NewIngredientID(display),
		Name:            key,
		Water:           comps.Water.Mid(),
		Fat:             comps.Fat.Mid(),
		Protein:         comps.Protein.Mid(),
		Lactose:         comps.Lactose.Mid(),
		Sucrose:         comps.Sucrose.Mid(),
		Glucose:         comps.Glucose.Mid(),
		Fructose:        comps.Fructose.Mid(),
		Maltodextrin:    comps.Maltodextrin.Mid(),
		Polyols:         comps.Polyols.Mid(),
		Ash:             comps.Ash.Mid(),
		OtherSolids:     comps.OtherSolids.Mid(),
		SaturatedFat:    comps.Fat.Mid(),
		SaturatedFatMin: comps.Fat.Lo,
		SaturatedFatMax: comps.Fat.Hi,
	}
}

func expandFractionBounds(lo, hi float64) (float64, float64) {
	return max(0, lo*(1-labelPercentEPS)), min(1, hi*(1+labelPercentEPS))
}
