package maxprocs

import (
	"github.com/brickingsoft/rxp/pkg/maxprocs/cpu"
	"os"
	"runtime"
)

const maxProcsEnvKey = "GOMAXPROCS"

type ProcsFunc func(minValue int, round func(v float64) int) (maxProcs int, status cpu.QuotaStatus, err error)

type RoundQuotaFunc func(v float64) int

type Undo func()

func Enable(options Options) (undo Undo, err error) {
	minGOMAXPROCS := options.MinGOMAXPROCS
	if minGOMAXPROCS < 0 {
		minGOMAXPROCS = 0
	}
	procs := options.Procs
	if procs == nil {
		procs = cpu.QuotaToGOMAXPROCS
	}
	roundQuotaFunc := options.RoundQuotaFunc
	if roundQuotaFunc == nil {
		roundQuotaFunc = cpu.DefaultRoundFunc
	}

	undo = func() {}

	if _, exists := os.LookupEnv(maxProcsEnvKey); exists {
		return
	}

	maxProcs, status, procsErr := procs(minGOMAXPROCS, roundQuotaFunc)
	if procsErr != nil {
		err = procsErr
		return
	}

	if status == cpu.QuotaUndefined {
		return
	}

	prev := runtime.GOMAXPROCS(0)
	undo = func() {
		runtime.GOMAXPROCS(prev)
	}

	runtime.GOMAXPROCS(maxProcs)
	return
}
