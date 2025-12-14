"""Shared physical constants and convenience values used across the solver."""

from __future__ import annotations

# Molecular weights (g/mol)
MW_SUCR = 342.30
MW_GLU = 180.16
MW_FRU = 180.16
MW_LAC = 342.30
MW_GLYCEROL = 92.09
MW_SORBITOL = 182.17
MW_ERYTHRITOL = 122.12

# Cryoscopic constant of water
K_F_WATER = 1.86  # °C·kg/mol

# Volume helpers
PINT_L = 0.473  # US liquid pint in liters
US_CUP_L = 0.236588  # US customary cup in liters
MIX_DENSITY_KG_PER_L = 1.02  # typical ice cream mix density before overrun
SERVING_PORTION_L = US_CUP_L * (2.0 / 3.0)  # default scoop volume for nutrition facts

# Global tolerance for label-driven constraints (2 % relative slack)
LABEL_PERCENT_EPS = 0.02
