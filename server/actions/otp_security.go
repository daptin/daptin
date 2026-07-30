package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/resource"
)

const (
	otpPeriodSeconds     = uint(120)
	otpAccountAttemptMax = int64(5)
	otpSourceAttemptMax  = int64(50)
	otpAttemptWindow     = 15 * time.Minute
	otpUsedCodeTTL       = 3 * time.Minute
)

var errOTPProtectionUnavailable = errors.New("OTP verification is temporarily unavailable")
var errOTPTooManyAttempts = errors.New("too many OTP attempts; try again later")
var errOTPAlreadyUsed = errors.New("OTP has already been used")

func otpProtectionHTTPError(err error) error {
	switch {
	case errors.Is(err, errOTPTooManyAttempts):
		return api2go.NewHTTPError(err, "otp_rate_limited", http.StatusTooManyRequests)
	case errors.Is(err, errOTPAlreadyUsed):
		return api2go.NewHTTPError(err, "otp_replayed", http.StatusConflict)
	default:
		return api2go.NewHTTPError(errOTPProtectionUnavailable, "otp_protection_unavailable", http.StatusServiceUnavailable)
	}
}

func otpSource(req *http.Request) string {
	if req == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if req.RemoteAddr != "" {
		return req.RemoteAddr
	}
	return "unknown"
}

func otpKeyPart(value string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(sum[:])
}

func consumeOTPAttempt(accountID int64, source string) error {
	if resource.OlricCache == nil {
		return errOTPProtectionUnavailable
	}
	ctx := context.Background()
	keys := []struct {
		key string
		max int64
	}{
		{key: fmt.Sprintf("otp-attempt:account:%d", accountID), max: otpAccountAttemptMax},
		{key: "otp-attempt:source:" + otpKeyPart(source), max: otpSourceAttemptMax},
	}
	for _, item := range keys {
		err := resource.OlricCache.Put(ctx, item.key, 0, olric.EX(otpAttemptWindow), olric.NX())
		if err != nil && !errors.Is(err, olric.ErrKeyFound) {
			return errOTPProtectionUnavailable
		}
		count, err := resource.OlricCache.Incr(ctx, item.key, 1)
		if err != nil {
			return errOTPProtectionUnavailable
		}
		if int64(count) > item.max {
			return errOTPTooManyAttempts
		}
	}
	return nil
}

func clearOTPAttempts(accountID int64, source string) {
	if resource.OlricCache == nil {
		return
	}
	ctx := context.Background()
	_, _ = resource.OlricCache.Delete(ctx,
		fmt.Sprintf("otp-attempt:account:%d", accountID),
		"otp-attempt:source:"+otpKeyPart(source),
	)
}

func consumeOTPCode(accountID int64, at time.Time) error {
	if resource.OlricCache == nil {
		return errOTPProtectionUnavailable
	}
	key := fmt.Sprintf("otp-used:account:%d:counter:%d", accountID, at.Unix()/int64(otpPeriodSeconds))
	err := resource.OlricCache.Put(context.Background(), key, true, olric.EX(otpUsedCodeTTL), olric.NX())
	if errors.Is(err, olric.ErrKeyFound) {
		return errOTPAlreadyUsed
	}
	if err != nil {
		return errOTPProtectionUnavailable
	}
	return nil
}
