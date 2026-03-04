package main

func shouldUseEditor(hasFlags bool, editFlag bool, noEditFlag bool, interactive bool) bool {
	if editFlag {
		return true
	}
	if noEditFlag {
		return false
	}
	if hasFlags {
		return false
	}
	return interactive
}
