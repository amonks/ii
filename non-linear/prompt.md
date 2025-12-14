Write me an industrial ice cream formulation solver.
- in python with casadi and ipopt
- use uv
- include type hints and docstrings; check types with `uvx ty check`
- include tests; unit tests of course but also some sanity checks against normal formulations
- just one module
- table of ingredients, no magic numbers
- include a wide variety of stabilizers, sugars and sugar alcohols, milk ingredients (including amf), etc
- include flavor ingredients: I’m not doing gelato, but in gelato terms we’re doing the “direct” method, not a single base
- include tests; make sure they pass

account for,

- Ingredient composition (water, dry solids, sugar types, fat, protein, ash).
- Water availability and water binding by stabilizers/hydrocolloids.
- True-molar solutes and effective-molar polymer solutes for freezing point depression.
- Nonideal freezing point (osmotic coefficient, ionic strength effects).
- Ice fraction as a function of temperature (freezing curve).
- Sweetness expressed as sucrose equivalents.
- Total solids and moisture balance.
- Hydrocolloid hydration kinetics (temperature-dependent).
- Rheology/viscosity as a function of hydrated polymer, solids, temperature, and shear rate.
- Fat destabilization and emulsifier–protein competition at fat–serum interfaces.
- Overrun prediction based on viscosity and fat/emulsifier behavior.
- Hardness/scoopability prediction based on ice fraction, solids, and polyol effects.
- Meltdown stability influenced by viscosity, overrun, and fat destabilization.
- Lactose crystallization (supersaturation, nucleation, growth; sandiness risk).
- Freezer residence-time crystallization kinetics (ice crystal size evolution).
- Freezer energy balance (heat load, draw temperature).
- Cost contribution of all ingredients.
