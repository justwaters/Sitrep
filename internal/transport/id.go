package transport

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewWorkerID generates a random, stable worker identifier. It is
// generated once by the worker itself before creating its enrollment CSR
// (not assigned by the manager), and embedded as the CSR's CommonName —
// so the manager's issued client certificate's CN, the WorkerID the
// worker claims in every Report, and the registry key the manager stores
// it under are all the same value, letting the report handler cross-check
// the verified cert's CN against the claimed WorkerID.
func NewWorkerID() (WorkerID, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate worker id: %w", err)
	}
	return WorkerID("wkr_" + hex.EncodeToString(buf)), nil
}
