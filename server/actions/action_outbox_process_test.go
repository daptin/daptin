package actions

import (
	"errors"
	"net"
	"testing"
)

func TestSendOutboxMailWithUsesConfiguredEHLOAndRecipientMX(t *testing.T) {
	var lookupDomain string
	var capturedMX string
	var capturedEHLO string
	var capturedFrom string
	var capturedTo []string

	err := sendOutboxMailWith(
		"mail.canaster.in",
		"login@canaster.in",
		[]string{"artpar@100x.bot"},
		[]byte("message"),
		func(domain string) ([]*net.MX, error) {
			lookupDomain = domain
			return []*net.MX{{Host: "mx.100x.bot."}}, nil
		},
		func(mxHost, ehloHostname, from string, to []string, message []byte) error {
			capturedMX = mxHost
			capturedEHLO = ehloHostname
			capturedFrom = from
			capturedTo = to
			return nil
		},
	)
	if err != nil {
		t.Fatalf("sendOutboxMailWith returned error: %v", err)
	}

	if lookupDomain != "100x.bot" {
		t.Fatalf("MX lookup domain = %q, want %q", lookupDomain, "100x.bot")
	}
	if capturedMX != "mx.100x.bot." {
		t.Fatalf("MX host = %q, want %q", capturedMX, "mx.100x.bot.")
	}
	if capturedEHLO != "mail.canaster.in" {
		t.Fatalf("EHLO hostname = %q, want %q", capturedEHLO, "mail.canaster.in")
	}
	if capturedFrom != "login@canaster.in" {
		t.Fatalf("from = %q, want %q", capturedFrom, "login@canaster.in")
	}
	if len(capturedTo) != 1 || capturedTo[0] != "artpar@100x.bot" {
		t.Fatalf("to = %#v, want one recipient artpar@100x.bot", capturedTo)
	}
}

func TestSendOutboxMailWithDoesNotUseRecipientDomainAsEHLO(t *testing.T) {
	err := sendOutboxMailWith(
		"mail.canaster.in",
		"login@canaster.in",
		[]string{"artpar@100x.bot"},
		[]byte("message"),
		func(domain string) ([]*net.MX, error) {
			return []*net.MX{{Host: domain}}, nil
		},
		func(mxHost, ehloHostname, from string, to []string, message []byte) error {
			if ehloHostname == "100x.bot" {
				return errors.New("used recipient domain as EHLO")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("sendOutboxMailWith returned error: %v", err)
	}
}

func TestSendOutboxMailWithStopsAfterUncertainDelivery(t *testing.T) {
	attempts := 0
	err := sendOutboxMailWith("mail.example.test", "sender@example.test", []string{"guest@example.test"},
		[]byte("message"), func(string) ([]*net.MX, error) {
			return []*net.MX{{Host: "primary.example.test"}, {Host: "backup.example.test"}}, nil
		}, func(string, string, string, []string, []byte) error {
			attempts++
			return &outboxUncertainError{err: errors.New("DATA response lost")}
		})
	var uncertain *outboxUncertainError
	if !errors.As(err, &uncertain) || attempts != 1 {
		t.Fatalf("uncertain delivery was retried at another MX: attempts=%d error=%v", attempts, err)
	}
}

func TestLookupOutboxMXUsesImplicitHostForLocalDelivery(t *testing.T) {
	mxs, err := lookupOutboxMX("localhost")
	if err != nil || len(mxs) == 0 || mxs[0].Host != "localhost" {
		t.Fatalf("localhost without MX was not resolved as an implicit MX: %v, %#v", err, mxs)
	}
}
