package maxprocs_test

import (
	"github.com/brickingsoft/rxp/pkg/internal/maxprocs"
	"testing"
)

func TestEnable(t *testing.T) {
	undo, err := maxprocs.Enable(maxprocs.Options{})
	if err != nil {
		t.Fatal(err)
		return
	}
	defer undo()
}
