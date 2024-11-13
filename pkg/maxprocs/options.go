package maxprocs

type Options struct {
	Procs          ProcsFunc
	MinGOMAXPROCS  int
	RoundQuotaFunc RoundQuotaFunc
}
