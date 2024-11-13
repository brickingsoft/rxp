package maxprocs

import (
	"github.com/brickingsoft/rxp/pkg/internal/maxprocs/cpu"
	"os"
	"runtime"
)

const maxProcsEnvKey = "GOMAXPROCS"

func currentMaxProcs() int {
	return runtime.GOMAXPROCS(0)
}

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

	prev := currentMaxProcs()
	undo = func() {
		runtime.GOMAXPROCS(prev)
	}

	runtime.GOMAXPROCS(maxProcs)
	return
}
