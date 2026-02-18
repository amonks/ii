package engine

// BreggleGazeUses returns how many times per day a breggle can use Gaze.
func BreggleGazeUses(level int) int {
	if level < 4 {
		return 0
	}
	if level < 6 {
		return 1
	}
	if level < 8 {
		return 2
	}
	if level < 10 {
		return 3
	}
	return 4
}
