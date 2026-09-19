package resource

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/daptin/daptin/server/auth"
)

type meteringPayloadCaptureKey struct{}

// MeteringPayloadCapture carries the original HTTP body to metering and the
// admitted reservation back to the HTTP response boundary.
type MeteringPayloadCapture struct {
	requestBody  bytes.Buffer
	mu           sync.Mutex
	reservations []MeteringPayloadReservation
}

type MeteringPayloadReservation struct {
	Token string
	Owner *auth.SessionUser
}

func NewMeteringPayloadCapture(request *http.Request) (*http.Request, *MeteringPayloadCapture) {
	capture := &MeteringPayloadCapture{}
	if request.Body != nil {
		request.Body = struct {
			io.Reader
			io.Closer
		}{Reader: io.TeeReader(request.Body, &capture.requestBody), Closer: request.Body}
	}
	return request.WithContext(context.WithValue(request.Context(), meteringPayloadCaptureKey{}, capture)), capture
}

func meteringPayloadCapture(request *http.Request) *MeteringPayloadCapture {
	if request == nil {
		return nil
	}
	capture, _ := request.Context().Value(meteringPayloadCaptureKey{}).(*MeteringPayloadCapture)
	return capture
}

func (capture *MeteringPayloadCapture) RequestBody() []byte {
	return append([]byte{}, capture.requestBody.Bytes()...)
}

func (capture *MeteringPayloadCapture) Reservations() []MeteringPayloadReservation {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]MeteringPayloadReservation(nil), capture.reservations...)
}

func (capture *MeteringPayloadCapture) HasReservation() bool {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return len(capture.reservations) != 0
}

func (capture *MeteringPayloadCapture) register(decision *MeteringDecision, owner *auth.SessionUser) {
	if capture == nil || decision == nil || !decision.Enabled {
		return
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	for _, reservation := range capture.reservations {
		if reservation.Token == decision.ReservationToken {
			return
		}
	}
	capture.reservations = append(capture.reservations, MeteringPayloadReservation{Token: decision.ReservationToken, Owner: owner})
}
