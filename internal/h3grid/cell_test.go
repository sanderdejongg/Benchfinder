package h3grid_test

import (
	"testing"

	"github.com/sanderdejongg/Benchfinder/internal/h3grid"
)

func TestCellForPoint(t *testing.T) {
	// Amsterdam Dam Square and a point in Rotterdam city centre, roughly
	// 55km apart — comfortably distinct at res 8 (edge length ~461m).
	amsterdam, err := h3grid.CellForPoint(52.3731, 4.8926)
	if err != nil {
		t.Fatalf("CellForPoint(amsterdam) returned error: %v", err)
	}
	if amsterdam == "" {
		t.Fatal("CellForPoint(amsterdam) returned an empty index")
	}

	rotterdam, err := h3grid.CellForPoint(51.9244, 4.4777)
	if err != nil {
		t.Fatalf("CellForPoint(rotterdam) returned error: %v", err)
	}

	if amsterdam == rotterdam {
		t.Fatalf("expected distinct cells for distant points, got %q for both", amsterdam)
	}

	again, err := h3grid.CellForPoint(52.3731, 4.8926)
	if err != nil {
		t.Fatalf("CellForPoint(amsterdam) second call returned error: %v", err)
	}
	if again != amsterdam {
		t.Fatalf("CellForPoint is not deterministic: got %q then %q", amsterdam, again)
	}
}
