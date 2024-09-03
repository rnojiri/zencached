package zencached

//go:generate enumer -json -text -sql -type ResultType -transform snake -trimprefix ResultType

type ResultType uint8

const (
	ResultTypeNone ResultType = iota
	ResultTypeError
	ResultTypeFound
	ResultTypeNotFound
	ResultTypeNotStored
	ResultTypeStored
	ResultTypeDeleted
	ResultTypeCompressionError
	ResultTypeDecompressionError
)
