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

// BreggleHornDamage returns the horn attack damage for a breggle at a level.
func BreggleHornDamage(level int) string {
	switch {
	case level < 3:
		return "1d4"
	case level < 6:
		return "1d4+1"
	case level < 9:
		return "1d6"
	case level < 10:
		return "1d6+1"
	default:
		return "1d6+2"
	}
}
