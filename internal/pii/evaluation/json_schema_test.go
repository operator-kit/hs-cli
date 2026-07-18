package evaluation

import "testing"

func TestJSONSchemaNumberBoundsAcceptFractionalMetrics(t *testing.T) {
	schema := []byte(`{"type":"number","minimum":0,"maximum":1}`)
	if err := ValidateJSONDocument(schema, []byte(`0.375`)); err != nil {
		t.Fatalf("fractional metric inside bounds was rejected: %v", err)
	}
	for _, value := range []string{`-0.001`, `1.001`} {
		if err := ValidateJSONDocument(schema, []byte(value)); err == nil {
			t.Fatalf("out-of-range metric %s was accepted", value)
		}
	}
}

func TestJSONSchemaIntegerStillRejectsFractionalValues(t *testing.T) {
	schema := []byte(`{"type":"integer","minimum":0}`)
	if err := ValidateJSONDocument(schema, []byte(`0.5`)); err == nil {
		t.Fatal("fractional value was accepted by an integer schema")
	}
}
