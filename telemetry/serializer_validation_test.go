package telemetry

import "testing"

func TestInvalidRecordErrorImplementsError(t *testing.T) {
    e := &InvalidRecordError{Reason: "missing txid"}
    if e.Error() == "" {
        t.Fatal("expected error string")
    }
}
