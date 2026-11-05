package lookup

import "testing"

func TestTryAllEmpty(t *testing.T) {
	got := TryAll("")
	if got != nil {
		t.Errorf("TryAll empty serial should return nil, got %+v", got)
	}
}
