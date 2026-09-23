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
	requestBytes int
	mu           sync.Mutex
	reservations []MeteringPayloadReservation
}

// MeteringPayloadBodyLimit bounds the request and response bodies retained for
// api_usage while byte counters continue to describe the complete payload.
const MeteringPayloadBodyLimit = 64 * 1024

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
		}{Reader: io.TeeReader(request.Body, capture), Closer: request.Body}
	}
	return request.WithContext(context.WithValue(request.Context(), meteringPayloadCaptureKey{}, capture)), capture
}

func (capture *MeteringPayloadCapture) Write(value []byte) (int, error) {
	capture.requestBytes += len(value)
	if remaining := MeteringPayloadBodyLimit - capture.requestBody.Len(); remaining > 0 {
		capture.requestBody.Write(value[:min(len(value), remaining)])
	}
	return len(value), nil
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

func (capture *MeteringPayloadCapture) RequestBytes() int {
	return capture.requestBytes
}

func (capture *MeteringPayloadCapture) RequestBodyTruncated() bool {
	return capture.requestBytes > capture.requestBody.Len()
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
