# Internal Validation

## Overview
The validation package provides shared helpers for formatting validation errors.

## Value Membership
- `IsValidValue` checks if a value appears in a list of valid options.

## Valid Value Formatting
- `FormatValidValues` joins string-like values for inclusion in error messages.
- `FormatInvalidValueError` wraps an invalid-value error with the valid values list.
