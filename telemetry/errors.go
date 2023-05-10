package telemetry

import "fmt"

// ErrMessageTooBig handles error when incoming payload is too large
var ErrMessageTooBig = fmt.Errorf("can't process message, size above 1mb")

// UnauthorizedSenderIDError is an error struct representing mismatch ID
type UnauthorizedSenderIDError struct {
	ExpectedSenderID string
	ReceivedSenderID string
}

// Error returns an error string implementing the error interface
func (e *UnauthorizedSenderIDError) Error() string {
	return fmt.Sprintf("Sender ID mismatch %s - %s", e.ExpectedSenderID, e.ReceivedSenderID)
}

// NonAnonymizedError is an error struct representing mismatch ID
type NonAnonymizedError struct {
}

// Error returns an error string implementing the error interface
func (e *NonAnonymizedError) Error() string {
	return "unexpected non-anonymized message"
}

// UnknownMessageType is an error struct representing a message that cannot be parsed
type UnknownMessageType struct {
	Bytes       []byte
	Txid        string
	GuessedType byte
}

// Error returns an error string implementing the error interface
func (e *UnknownMessageType) Error() string {
	return fmt.Sprintf("Unknown message Type for %s - %v", e.Txid, e.GuessedType)
}
