package value_test

import (
	"github.com/kdihalas/mosaic/pkg/value"
	"math/big"
	"testing"
)

func TestPrecisionCloneAndCanonicalOrdering(t *testing.T) {
	r, _ := new(big.Rat).SetString("123456789.125")
	v := value.Object(map[string]value.Value{"z": value.Decimal(r), "a": value.Int(big.NewInt(2))})
	b, e := v.CanonicalJSON()
	if e != nil || string(b) != `{"a":2,"z":123456789.125}` {
		t.Fatalf("%s %v", b, e)
	}
	if !v.Equal(v.Clone()) {
		t.Fatal("clone differs")
	}
}
func TestReferenceRoundTrip(t *testing.T) {
	v := value.Reference(value.ReferenceValue{Target: "application.a.workload.main", Field: []string{"name"}})
	b, _ := v.CanonicalJSON()
	var x value.Value
	if e := x.UnmarshalJSON(b); e != nil || !v.Equal(x) {
		t.Fatalf("%s %v", b, e)
	}
}
