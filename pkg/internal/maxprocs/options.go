package maxprocs

import "github.com/brickingsoft/rxp/pkg/internal/maxprocs/cpu"

type Options struct {
	Procs          func(int, func(v float64) int) (int, cpu.QuotaStatus, error)
	MinGOMAXPROCS  int
	RoundQuotaFunc func(v float64) int
}
