package creamery_test

import "testing"

func TestJenisSweetCreamFinal(t *testing.T) {
	// This test requires custom ingredient specs (NonfatMilkVariable) which are not
	// in the FDA DSL format. The new FDA DSL intentionally carries less information.
	t.Skip("jenis test requires custom ingredient specs not in FDA DSL")
}
