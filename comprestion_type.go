package zencached

//go:generate enumer -json -text -sql -type CompressionType -transform snake -trimprefix CompressionType

type CompressionType uint8

const (
	CompressionTypeNone CompressionType = iota
	CompressionTypeBase64
	CompressionTypeZstandard
)
