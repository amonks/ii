package engine

import "strings"

var classNames = []string{
	"Bard",
	"Cleric",
	"Enchanter",
	"Fighter",
	"Friar",
	"Hunter",
	"Knight",
	"Magician",
	"Thief",
}

var kindredNames = []string{
	"Human",
	"Elf",
	"Grimalkin",
	"Mossling",
	"Woodgrue",
	"Breggle",
}

var classPrimes = map[string][]string{
	"bard":      {"cha", "dex"},
	"cleric":    {"wis"},
	"enchanter": {"cha", "int"},
	"fighter":   {"str"},
	"friar":     {"int", "wis"},
	"hunter":    {"con", "dex"},
	"knight":    {"str", "cha"},
	"magician":  {"int"},
	"thief":     {"dex"},
}

var classNameSet = makeNameSet(classNames)
var kindredNameSet = makeNameSet(kindredNames)

// ClassNames returns the list of class names in display order.
func ClassNames() []string {
	return append([]string(nil), classNames...)
}

// KindredNames returns the list of kindred names in display order.
func KindredNames() []string {
	return append([]string(nil), kindredNames...)
}

// IsValidClass reports whether the class is supported (case-insensitive).
func IsValidClass(class string) bool {
	_, ok := classNameSet[strings.ToLower(class)]
	return ok
}

// IsValidKindred reports whether the kindred is supported (case-insensitive).
func IsValidKindred(kindred string) bool {
	_, ok := kindredNameSet[strings.ToLower(kindred)]
	return ok
}

// ClassPrimes returns the prime ability score names for a class.
func ClassPrimes(class string) []string {
	primes, ok := classPrimes[strings.ToLower(class)]
	if !ok {
		return nil
	}
	return append([]string(nil), primes...)
}

func makeNameSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, name := range values {
		set[strings.ToLower(name)] = struct{}{}
	}
	return set
}
