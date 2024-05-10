package zencached

//go:generate enumer -json -text -sql -type ErrorType -transform snake -trimprefix ErrorType

type ErrorType uint8

const (
	ErrorTypeUndefined ErrorType = iota
	ErrorTypeMaxReconnectionsReached
	ErrorTypeMemcachedInvalidResponse
	ErrorTypeMemcachedNoResponse
	ErrorTypeTelnetConnectionIsClosed
	ErrorTypeNoAvailableNodes
	ErrorTypeNoAvailableConnections
)

var (
	ErrMaxReconnectionsReached  ZError = NewError("maximum number of reconnections reached", ErrorTypeMaxReconnectionsReached)
	ErrMemcachedInvalidResponse ZError = NewError("invalid memcached command response received", ErrorTypeMemcachedInvalidResponse)
	ErrMemcachedNoResponse      ZError = NewError("no response from memcached", ErrorTypeMemcachedNoResponse)
	ErrTelnetConnectionIsClosed ZError = NewError("telnet connection is closed", ErrorTypeTelnetConnectionIsClosed)
	ErrNoAvailableNodes         ZError = NewError("there are no nodes available", ErrorTypeNoAvailableNodes)
	ErrNoAvailableConnections   ZError = NewError("there are no available connections", ErrorTypeNoAvailableConnections)
)

// ZErrorData - a struc to store some metadata in the error to be an alternative to include zencached deps
type ZErrorData struct {
	msg       string
	errorType ErrorType
}

func (e ZErrorData) Error() string {

	return e.msg
}

func (e ZErrorData) Code() uint8 {

	return uint8(e.errorType)
}

func (e ZErrorData) String() string {

	return e.errorType.String()
}

// NewError - creates a new error
func NewError(msg string, et ErrorType) ZError {

	return ZErrorData{
		msg:       msg,
		errorType: et,
	}
}
