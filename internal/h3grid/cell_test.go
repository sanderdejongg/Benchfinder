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

func TestCellBBox(t *testing.T) {
	const lat, lng = 52.3731, 4.8926

	cell, err := h3grid.CellForPoint(lat, lng)
	if err != nil {
		t.Fatalf("CellForPoint returned error: %v", err)
	}

	bbox, err := h3grid.CellBBox(cell)
	if err != nil {
		t.Fatalf("CellBBox returned error: %v", err)
	}

	if bbox.South >= bbox.North {
		t.Fatalf("bbox.South (%v) should be less than bbox.North (%v)", bbox.South, bbox.North)
	}
	if bbox.West >= bbox.East {
		t.Fatalf("bbox.West (%v) should be less than bbox.East (%v)", bbox.West, bbox.East)
	}
	if lat < bbox.South || lat > bbox.North || lng < bbox.West || lng > bbox.East {
		t.Fatalf("bbox %+v does not contain the point it was derived from (%v, %v)", bbox, lat, lng)
	}

	if _, err := h3grid.CellBBox("not-a-real-cell"); err == nil {
		t.Fatal("CellBBox with an invalid index returned no error")
	}
}
