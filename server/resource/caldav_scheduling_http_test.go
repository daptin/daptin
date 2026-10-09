package resource

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/emersion/go-ical"
)

func TestSchedulingSupportedPrivilegesXML(t *testing.T) {
	type privilegeNode struct {
		Description string          `xml:"DAV: description"`
		Children    []privilegeNode `xml:"DAV: supported-privilege"`
	}
	for _, outbox := range []bool{false, true} {
		data := `<D:supported-privilege-set xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">` + schedulingSupportedPrivileges(outbox) + `</D:supported-privilege-set>`
		var tree struct {
			Roots []privilegeNode `xml:"DAV: supported-privilege"`
		}
		if err := xml.Unmarshal([]byte(data), &tree); err != nil {
			t.Fatalf("invalid supported privileges XML: %v", err)
		}
		var check func(privilegeNode)
		check = func(node privilegeNode) {
			if node.Description == "" {
				t.Fatal("supported privilege is missing its required description")
			}
			for _, child := range node.Children {
				check(child)
			}
		}
		if len(tree.Roots) != 1 {
			t.Fatalf("expected one aggregate privilege, got %d", len(tree.Roots))
		}
		check(tree.Roots[0])
	}
}

func TestSchedulingHTTPXMLContract(t *testing.T) {
	propfind := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:resourcetype/><C:schedule-inbox-URL/><C:schedule-outbox-URL/></D:prop></D:propfind>`
	if !schedulingPrincipalRequested([]byte(propfind)) || schedulingPrincipalRequested([]byte(`<D:propfind xmlns:D="DAV:"><D:propname/></D:propfind>`)) {
		t.Fatal("ordinary principal property names must remain with the DAV handler")
	}
	if schedulingPrincipalRequested([]byte(`<D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`)) {
		t.Fatal("ordinary allprop discovery must remain with the DAV handler")
	}
	if schedulingPrincipalRequested(nil) {
		t.Fatal("empty principal PROPFIND is allprop and must remain with the DAV handler")
	}
	request := httptest.NewRequest("PROPFIND", "/caldav/owner/", strings.NewReader(propfind))
	wanted, err := readSchedulingProperties(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range []xml.Name{
		{Space: davXMLNamespace, Local: "resourcetype"},
		{Space: caldavXMLNamespace, Local: "schedule-inbox-URL"},
		{Space: caldavXMLNamespace, Local: "schedule-outbox-URL"},
	} {
		if !wanted[property] {
			t.Fatalf("missing requested property %v", property)
		}
	}
	response := httptest.NewRecorder()
	writeSchedulingMultistatus(response, []schedulingXMLResource{{href: "/caldav/owner/", properties: map[xml.Name]string{
		{Space: davXMLNamespace, Local: "resourcetype"}:          "<D:collection/><D:principal/>",
		{Space: caldavXMLNamespace, Local: "schedule-inbox-URL"}: schedulingHref("/caldav/owner/schedule-inbox/"),
	}}}, wanted)
	if response.Code != http.StatusMultiStatus || !strings.Contains(response.Body.String(), "404 Not Found") {
		t.Fatalf("missing scheduling property did not get a 404 propstat: %d %s", response.Code, response.Body.String())
	}
	decoder := xml.NewDecoder(strings.NewReader(response.Body.String()))
	for {
		if _, err := decoder.Token(); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("invalid DAV multistatus XML: %v", err)
		}
	}
	missingResponse := httptest.NewRecorder()
	writeSchedulingMultistatus(missingResponse, []schedulingXMLResource{{href: "/caldav/owner/schedule-inbox/missing.ics", status: http.StatusNotFound}}, wanted)
	if missingResponse.Code != http.StatusMultiStatus || !strings.Contains(missingResponse.Body.String(), "<D:status>HTTP/1.1 404 Not Found</D:status>") {
		t.Fatalf("missing multiget item must have a per-resource 404: %d %s", missingResponse.Code, missingResponse.Body.String())
	}

	query := `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><C:calendar-data/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT"><C:time-range start="20300101T000000Z" end="20300102T000000Z"/></C:comp-filter></C:comp-filter></C:filter></C:calendar-query>`
	var report schedulingXMLReport
	if err := xml.Unmarshal([]byte(query), &report); err != nil {
		t.Fatal(err)
	}
	filter, err := schedulingCompFilter(report.Filter.Component)
	if err != nil || filter.Name != ical.CompCalendar || len(filter.Comps) != 1 || filter.Comps[0].Name != ical.CompEvent || filter.Comps[0].Start.IsZero() {
		t.Fatalf("calendar-query filter lost its component or range: %+v, %v", filter, err)
	}
	filter, err = schedulingCompFilter(schedulingXMLFilter{Name: ical.CompEvent, Properties: []schedulingXMLPropFilter{{
		Name: "UID", Text: &schedulingXMLTextMatch{Text: "meeting-1"},
	}}})
	if err != nil || len(filter.Props) != 1 || filter.Props[0].TextMatch == nil || filter.Props[0].TextMatch.Text != "meeting-1" {
		t.Fatalf("calendar-query lost the UID filter: %+v, %v", filter, err)
	}
}

func TestSchedulingHTTPPrincipalBoundary(t *testing.T) {
	owner := daptinid.InterfaceToDIR("11111111-1111-1111-1111-111111111111")
	other := daptinid.InterfaceToDIR("22222222-2222-2222-2222-222222222222")
	backend := NewCalDAVBackend(nil, &auth.SessionUser{UserReferenceId: owner}, nil)
	principalPath := "/caldav/" + owner.String() + "/"
	ordinary := httptest.NewRequest("PROPFIND", principalPath, strings.NewReader(`<D:propfind xmlns:D="DAV:"><D:propname/></D:propfind>`))
	if backend.ServeScheduling(httptest.NewRecorder(), ordinary) {
		t.Fatal("ordinary principal PROPFIND must remain with the DAV handler")
	}
	if body, _ := io.ReadAll(ordinary.Body); !strings.Contains(string(body), "propname") {
		t.Fatal("principal request body was consumed before DAV handling")
	}
	foreign := httptest.NewRequest("PROPFIND", "/caldav/"+other.String()+"/schedule-inbox/", nil)
	response := httptest.NewRecorder()
	if !backend.ServeScheduling(response, foreign) || response.Code != http.StatusForbidden {
		t.Fatalf("foreign scheduling principal must be denied: %d", response.Code)
	}
}
