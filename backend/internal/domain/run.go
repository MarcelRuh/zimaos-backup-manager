package domain

type Result string

const (
	ResultSuccess        Result = "success"
	ResultFailed         Result = "failed"
	ResultRunning        Result = "running"
	ResultUnknown        Result = "unknown"
	ResultConnectionLost Result = "connection_lost"
	ResultAmbiguous      Result = "ambiguous"
)

type DurationQuality string

const (
	DurationMeasured  DurationQuality = "measured"
	DurationEstimated DurationQuality = "estimated"
)

func IsTerminal(r Result) bool {
	switch r {
	case ResultSuccess, ResultFailed, ResultUnknown, ResultAmbiguous:
		return true
	default:
		return false
	}
}

func IsNotifySuccess(r Result) bool {
	return r == ResultSuccess
}

func IsNotifyFailure(r Result) bool {
	return r == ResultFailed
}
