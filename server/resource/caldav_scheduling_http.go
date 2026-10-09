package resource

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/go-webdav/caldav"
	"github.com/emersion/go-ical"
)

const (
	schedulingXMLLimit       = 1 << 20
	schedulingMaxMultiget    = 100
	schedulingMaxFilterDepth = 8
)

const (
	davXMLNamespace    = "DAV:"
	caldavXMLNamespace = "urn:ietf:params:xml:ns:caldav"
)

type schedulingXMLFilter struct {
	Name       string                    `xml:"name,attr"`
	NotDefined *struct{}                 `xml:"is-not-defined"`
	TimeRange  *schedulingXMLTimeRange   `xml:"time-range"`
	Properties []schedulingXMLPropFilter `xml:"prop-filter"`
	Components []schedulingXMLFilter     `xml:"comp-filter"`
}

type schedulingXMLPropFilter struct {
	Name       string                     `xml:"name,attr"`
	NotDefined *struct{}                  `xml:"is-not-defined"`
	TimeRange  *schedulingXMLTimeRange    `xml:"time-range"`
	Text       *schedulingXMLTextMatch    `xml:"text-match"`
	Parameters []schedulingXMLParamFilter `xml:"param-filter"`
}

type schedulingXMLParamFilter struct {
	Name       string                  `xml:"name,attr"`
	NotDefined *struct{}               `xml:"is-not-defined"`
	Text       *schedulingXMLTextMatch `xml:"text-match"`
}

type schedulingXMLTextMatch struct {
	Text   string `xml:",chardata"`
	Negate string `xml:"negate-condition,attr"`
}

type schedulingXMLTimeRange struct {
	Start string `xml:"start,attr"`
	End   string `xml:"end,attr"`
}

type schedulingXMLReport struct {
	XMLName xml.Name
	Hrefs   []string `xml:"DAV: href"`
	Filter  struct {
		Component schedulingXMLFilter `xml:"comp-filter"`
	} `xml:"filter"`
}

// ServeScheduling owns only scheduling URLs and scheduling principal discovery.
// The DAV handler continues to own ordinary CalDAV resources.
func (b *DaptinDAVBackend) ServeScheduling(w http.ResponseWriter, r *http.Request) bool {
	if b.sessionUser == nil {
		return false
	}
	principalPath := "/caldav/" + b.sessionUser.UserReferenceId.String() + "/"
	if r.Method == http.MethodOptions {
		parts := strings.Split(strings.Trim(path.Clean(r.URL.Path), "/"), "/")
		if len(parts) >= 2 && len(parts) <= 4 && parts[0] == "caldav" && parts[1] == b.sessionUser.UserReferenceId.String() {
			allow := ""
			switch {
			case len(parts) == 2:
				allow = "OPTIONS, PROPFIND"
			case len(parts) == 3 && parts[2] == "calendars":
				allow = "OPTIONS, PROPFIND, MKCOL"
			case len(parts) == 4 && parts[2] == "calendars":
				if _, err := b.GetCalendar(r.Context(), r.URL.Path); err == nil {
					allow = "OPTIONS, PROPFIND, REPORT, DELETE"
				}
			}
			if allow != "" {
				active, err := b.autoSchedulingEnabled(r.Context(), principalPath)
				if err != nil {
					writeSchedulingError(w, err, http.StatusInternalServerError)
					return true
				}
				if active {
					writeSchedulingOptions(w, allow, true)
					return true
				}
			}
		}
	}
	if r.Method == "PROPFIND" && path.Clean(r.URL.Path) == path.Clean(principalPath) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, schedulingXMLLimit))
		if err != nil {
			writeSchedulingError(w, err, http.StatusBadRequest)
			return true
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if !schedulingPrincipalRequested(body) {
			return false
		}
		b.serveSchedulingPrincipal(w, r, principalPath)
		return true
	}
	parts := strings.Split(strings.Trim(path.Clean(r.URL.Path), "/"), "/")
	if len(parts) < 3 || len(parts) > 4 || parts[0] != "caldav" ||
		(parts[2] != "schedule-inbox" && parts[2] != "schedule-outbox") {
		return false
	}
	if parts[2] == "schedule-outbox" && len(parts) != 3 ||
		parts[2] == "schedule-inbox" && len(parts) == 4 && !strings.HasSuffix(parts[3], ".ics") {
		http.Error(w, "invalid scheduling path", http.StatusNotFound)
		return true
	}
	principal, err := b.schedulingPrincipal(r.Context(), "/caldav/"+parts[1]+"/")
	if err != nil || principal == nil {
		writeSchedulingError(w, err, http.StatusNotFound)
		return true
	}
	if parts[2] == "schedule-outbox" && len(principal.Addresses) == 0 {
		http.Error(w, "scheduling outbox unavailable", http.StatusNotFound)
		return true
	}
	if parts[2] == "schedule-inbox" {
		b.serveSchedulingInbox(w, r, principal, len(parts) == 4)
	} else {
		b.serveSchedulingOutbox(w, r, principal)
	}
	return true
}

func schedulingPrincipalRequested(body []byte) bool {
	if len(bytes.TrimSpace(body)) == 0 {
		return false
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		if element, ok := token.(xml.StartElement); ok {
			if element.Name.Space == caldavXMLNamespace {
				switch element.Name.Local {
				case "calendar-user-address-set", "schedule-inbox-URL", "schedule-outbox-URL":
					return true
				}
			}
		}
	}
}

func (b *DaptinDAVBackend) serveSchedulingPrincipal(w http.ResponseWriter, r *http.Request, principalPath string) {
	requested, err := readSchedulingProperties(r)
	if err != nil {
		writeSchedulingError(w, err, http.StatusBadRequest)
		return
	}
	principal, err := b.schedulingPrincipal(r.Context(), principalPath)
	if err != nil {
		writeSchedulingError(w, err, http.StatusInternalServerError)
		return
	}
	home, err := b.CalendarHomeSetPath(r.Context(), principalPath)
	if err != nil {
		writeSchedulingError(w, err, http.StatusInternalServerError)
		return
	}
	properties := map[xml.Name]string{
		{Space: davXMLNamespace, Local: "current-user-principal"}: schedulingHref(principalPath),
		{Space: davXMLNamespace, Local: "resourcetype"}:           "<D:collection/><D:principal/>",
		{Space: caldavXMLNamespace, Local: "calendar-home-set"}:   schedulingHref(home),
	}
	if principal != nil && len(principal.Addresses) != 0 {
		var addresses strings.Builder
		for _, address := range principal.Addresses {
			addresses.WriteString(schedulingHref(address))
		}
		properties[xml.Name{Space: caldavXMLNamespace, Local: "calendar-user-address-set"}] = addresses.String()
		properties[xml.Name{Space: caldavXMLNamespace, Local: "schedule-inbox-URL"}] = schedulingHref(principal.InboxPath)
		properties[xml.Name{Space: caldavXMLNamespace, Local: "schedule-outbox-URL"}] = schedulingHref(principal.OutboxPath)
	}
	writeSchedulingMultistatus(w, []schedulingXMLResource{{href: principalPath, properties: properties}}, requested)
}

func (b *DaptinDAVBackend) serveSchedulingInbox(w http.ResponseWriter, r *http.Request, principal *schedulingPrincipal, objectPath bool) {
	if r.Method == http.MethodOptions {
		allow := "OPTIONS, PROPFIND, REPORT"
		if objectPath {
			allow = "OPTIONS, HEAD, GET, DELETE, PROPFIND"
		}
		active, err := b.autoSchedulingEnabled(r.Context(), "/caldav/"+b.sessionUser.UserReferenceId.String()+"/")
		if err != nil {
			writeSchedulingError(w, err, http.StatusInternalServerError)
			return
		}
		writeSchedulingOptions(w, allow, active)
		return
	}
	if objectPath {
		object, err := b.getSchedulingInboxObject(r.Context(), r.URL.Path)
		if err != nil {
			writeSchedulingError(w, err, http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
			w.Header().Set("ETag", strconv.Quote(object.ETag))
			if r.Method == http.MethodGet {
				if err := ical.NewEncoder(w).Encode(object.Data); err != nil {
					return
				}
			}
		case "PROPFIND":
			requested, err := readSchedulingProperties(r)
			if err != nil {
				writeSchedulingError(w, err, http.StatusBadRequest)
				return
			}
			resource, err := schedulingObjectResource(object)
			if err != nil {
				writeSchedulingError(w, err, http.StatusInternalServerError)
				return
			}
			writeSchedulingMultistatus(w, []schedulingXMLResource{resource}, requested)
		case http.MethodDelete:
			if err := b.deleteSchedulingInboxObject(r.Context(), r.URL.Path); err != nil {
				writeSchedulingError(w, err, http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "OPTIONS, HEAD, GET, DELETE, PROPFIND")
			http.Error(w, "method unavailable", http.StatusMethodNotAllowed)
		}
		return
	}
	switch r.Method {
	case "PROPFIND":
		requested, err := readSchedulingProperties(r)
		if err != nil {
			writeSchedulingError(w, err, http.StatusBadRequest)
			return
		}
		resources := []schedulingXMLResource{{href: principal.InboxPath, properties: map[xml.Name]string{
			{Space: davXMLNamespace, Local: "resourcetype"}:               "<D:collection/><C:schedule-inbox/>",
			{Space: davXMLNamespace, Local: "current-user-privilege-set"}: "<D:privilege><D:read/></D:privilege><D:privilege><D:unbind/></D:privilege>",
		}}}
		if r.Header.Get("Depth") != "0" {
			objects, err := b.listSchedulingInbox(r.Context(), r.URL.Path)
			if err != nil {
				writeSchedulingError(w, err, http.StatusInternalServerError)
				return
			}
			for i := range objects {
				resource, err := schedulingObjectResource(&objects[i])
				if err != nil {
					writeSchedulingError(w, err, http.StatusInternalServerError)
					return
				}
				resources = append(resources, resource)
			}
		}
		writeSchedulingMultistatus(w, resources, requested)
	case "REPORT":
		b.serveSchedulingReport(w, r)
	default:
		w.Header().Set("Allow", "OPTIONS, PROPFIND, REPORT")
		http.Error(w, "method unavailable", http.StatusMethodNotAllowed)
	}
}

func (b *DaptinDAVBackend) serveSchedulingOutbox(w http.ResponseWriter, r *http.Request, principal *schedulingPrincipal) {
	switch r.Method {
	case http.MethodOptions:
		active, err := b.autoSchedulingEnabled(r.Context(), "/caldav/"+b.sessionUser.UserReferenceId.String()+"/")
		if err != nil {
			writeSchedulingError(w, err, http.StatusInternalServerError)
			return
		}
		writeSchedulingOptions(w, "OPTIONS, PROPFIND, POST", active)
	case "PROPFIND":
		requested, err := readSchedulingProperties(r)
		if err != nil {
			writeSchedulingError(w, err, http.StatusBadRequest)
			return
		}
		writeSchedulingMultistatus(w, []schedulingXMLResource{{href: principal.OutboxPath, properties: map[xml.Name]string{
			{Space: davXMLNamespace, Local: "resourcetype"}:               "<D:collection/><C:schedule-outbox/>",
			{Space: davXMLNamespace, Local: "current-user-privilege-set"}: "<D:privilege><C:schedule-send-freebusy/></D:privilege>",
		}}}, requested)
	case http.MethodPost:
		if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "text/calendar") {
			http.Error(w, "expected text/calendar", http.StatusBadRequest)
			return
		}
		calendar, err := ical.NewDecoder(http.MaxBytesReader(w, r.Body, schedulingXMLLimit)).Decode()
		if err != nil {
			writeSchedulingError(w, err, http.StatusBadRequest)
			return
		}
		responses, err := b.schedulingFreeBusy(r.Context(), r.URL.Path, calendar)
		if err != nil {
			writeSchedulingError(w, err, http.StatusBadRequest)
			return
		}
		var response strings.Builder
		response.WriteString(`<C:schedule-response xmlns:C="` + caldavXMLNamespace + `" xmlns:D="DAV:">`)
		for _, item := range responses {
			response.WriteString("<C:response><C:recipient>" + schedulingHref(item.Recipient) + "</C:recipient><C:request-status>" + schedulingEscape(item.RequestStatus) + "</C:request-status>")
			if item.Calendar != nil {
				var encoded bytes.Buffer
				if err := ical.NewEncoder(&encoded).Encode(item.Calendar); err != nil {
					writeSchedulingError(w, err, http.StatusInternalServerError)
					return
				}
				response.WriteString("<C:calendar-data>" + schedulingEscape(encoded.String()) + "</C:calendar-data>")
			}
			response.WriteString("</C:response>")
		}
		response.WriteString("</C:schedule-response>")
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, _ = io.WriteString(w, response.String())
	default:
		w.Header().Set("Allow", "OPTIONS, PROPFIND, POST")
		http.Error(w, "method unavailable", http.StatusMethodNotAllowed)
	}
}

func (b *DaptinDAVBackend) serveSchedulingReport(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, schedulingXMLLimit))
	if err != nil {
		writeSchedulingError(w, err, http.StatusBadRequest)
		return
	}
	var report schedulingXMLReport
	if err := xml.Unmarshal(body, &report); err != nil {
		writeSchedulingError(w, err, http.StatusBadRequest)
		return
	}
	var objects []caldav.CalendarObject
	var missingHrefs []string
	switch report.XMLName {
	case xml.Name{Space: caldavXMLNamespace, Local: "calendar-multiget"}:
		if len(report.Hrefs) > schedulingMaxMultiget {
			http.Error(w, "too many inbox references", http.StatusBadRequest)
			return
		}
		for _, href := range report.Hrefs {
			location, err := url.Parse(href)
			if err != nil || location.RawQuery != "" || location.Fragment != "" || path.Dir(location.Path) != path.Clean(r.URL.Path) {
				http.Error(w, "invalid inbox reference", http.StatusBadRequest)
				return
			}
			object, err := b.getSchedulingInboxObject(r.Context(), location.Path)
			if err != nil {
				var missing *schedulingHTTPError
				if errors.As(err, &missing) && missing.status == http.StatusNotFound {
					missingHrefs = append(missingHrefs, location.Path)
					continue
				}
				writeSchedulingError(w, err, http.StatusInternalServerError)
				return
			}
			objects = append(objects, *object)
		}
	case xml.Name{Space: caldavXMLNamespace, Local: "calendar-query"}:
		filter, err := schedulingCompFilter(report.Filter.Component)
		if err != nil {
			writeSchedulingError(w, err, http.StatusBadRequest)
			return
		}
		objects, err = b.querySchedulingInbox(r.Context(), r.URL.Path, &caldav.CalendarQuery{CompFilter: filter})
		if err != nil {
			writeSchedulingError(w, err, http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, "unsupported scheduling report", http.StatusBadRequest)
		return
	}
	resources := make([]schedulingXMLResource, 0, len(objects)+len(missingHrefs))
	for _, href := range missingHrefs {
		resources = append(resources, schedulingXMLResource{href: href, status: http.StatusNotFound})
	}
	for i := range objects {
		resource, err := schedulingObjectResource(&objects[i])
		if err != nil {
			writeSchedulingError(w, err, http.StatusInternalServerError)
			return
		}
		resources = append(resources, resource)
	}
	requested, err := schedulingRequestedProperties(body)
	if err != nil {
		writeSchedulingError(w, err, http.StatusBadRequest)
		return
	}
	writeSchedulingMultistatus(w, resources, requested)
}

func schedulingCompFilter(input schedulingXMLFilter) (caldav.CompFilter, error) {
	return schedulingCompFilterAtDepth(input, 0)
}

func schedulingCompFilterAtDepth(input schedulingXMLFilter, depth int) (caldav.CompFilter, error) {
	if depth > schedulingMaxFilterDepth {
		return caldav.CompFilter{}, errors.New("calendar-query filter is too deep")
	}
	if input.Name == "" {
		return caldav.CompFilter{}, errors.New("calendar-query requires a component filter")
	}
	result := caldav.CompFilter{Name: input.Name, IsNotDefined: input.NotDefined != nil}
	if result.IsNotDefined && (input.TimeRange != nil || len(input.Properties) != 0 || len(input.Components) != 0) {
		return caldav.CompFilter{}, errors.New("is-not-defined cannot have component filters")
	}
	if input.TimeRange != nil {
		var err error
		result.Start, result.End, err = schedulingTimeRange(input.TimeRange)
		if err != nil {
			return caldav.CompFilter{}, err
		}
	}
	for _, property := range input.Properties {
		if property.Name == "" || property.NotDefined != nil && (property.TimeRange != nil || property.Text != nil || len(property.Parameters) != 0) {
			return caldav.CompFilter{}, errors.New("invalid calendar-query property filter")
		}
		decoded := caldav.PropFilter{Name: property.Name, IsNotDefined: property.NotDefined != nil}
		if property.TimeRange != nil {
			var err error
			decoded.Start, decoded.End, err = schedulingTimeRange(property.TimeRange)
			if err != nil {
				return caldav.CompFilter{}, err
			}
		}
		if property.Text != nil {
			decoded.TextMatch = &caldav.TextMatch{Text: property.Text.Text, NegateCondition: strings.EqualFold(property.Text.Negate, "yes")}
		}
		for _, parameter := range property.Parameters {
			if parameter.Name == "" || parameter.NotDefined != nil && parameter.Text != nil {
				return caldav.CompFilter{}, errors.New("invalid calendar-query parameter filter")
			}
			param := caldav.ParamFilter{Name: parameter.Name, IsNotDefined: parameter.NotDefined != nil}
			if parameter.Text != nil {
				param.TextMatch = &caldav.TextMatch{Text: parameter.Text.Text, NegateCondition: strings.EqualFold(parameter.Text.Negate, "yes")}
			}
			decoded.ParamFilter = append(decoded.ParamFilter, param)
		}
		result.Props = append(result.Props, decoded)
	}
	for _, component := range input.Components {
		child, err := schedulingCompFilterAtDepth(component, depth+1)
		if err != nil {
			return caldav.CompFilter{}, err
		}
		result.Comps = append(result.Comps, child)
	}
	return result, nil
}

func schedulingTimeRange(input *schedulingXMLTimeRange) (time.Time, time.Time, error) {
	if input.Start == "" && input.End == "" {
		return time.Time{}, time.Time{}, errors.New("empty calendar-query time range")
	}
	var start, end time.Time
	var err error
	if input.Start != "" {
		start, err = time.Parse("20060102T150405Z", input.Start)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if input.End != "" {
		end, err = time.Parse("20060102T150405Z", input.End)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !start.IsZero() && !end.IsZero() && !start.Before(end) {
		return time.Time{}, time.Time{}, errors.New("invalid calendar-query time range")
	}
	return start, end, nil
}

type schedulingXMLResource struct {
	href       string
	status     int
	properties map[xml.Name]string
}

type schedulingHTTPError struct {
	status int
	cause  error
}

func (e *schedulingHTTPError) Error() string { return e.cause.Error() }
func (e *schedulingHTTPError) Unwrap() error { return e.cause }
func (e *schedulingHTTPError) Status() int   { return e.status }

func newSchedulingHTTPError(status int, cause error) error {
	return &schedulingHTTPError{status: status, cause: cause}
}

func schedulingObjectResource(object *caldav.CalendarObject) (schedulingXMLResource, error) {
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(object.Data); err != nil {
		return schedulingXMLResource{}, err
	}
	return schedulingXMLResource{href: object.Path, properties: map[xml.Name]string{
		{Space: davXMLNamespace, Local: "resourcetype"}:     "",
		{Space: davXMLNamespace, Local: "getetag"}:          schedulingEscape(strconv.Quote(object.ETag)),
		{Space: davXMLNamespace, Local: "getcontenttype"}:   "text/calendar; charset=utf-8",
		{Space: davXMLNamespace, Local: "getcontentlength"}: strconv.Itoa(encoded.Len()),
		{Space: caldavXMLNamespace, Local: "calendar-data"}: schedulingEscape(encoded.String()),
	}}, nil
}

func readSchedulingProperties(r *http.Request) (map[xml.Name]bool, error) {
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, schedulingXMLLimit))
	if err != nil {
		return nil, err
	}
	return schedulingRequestedProperties(body)
}

func schedulingRequestedProperties(body []byte) (map[xml.Name]bool, error) {
	requested := make(map[xml.Name]bool)
	if len(bytes.TrimSpace(body)) == 0 {
		return requested, nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	insideProp := false
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch node := token.(type) {
		case xml.StartElement:
			if insideProp {
				depth++
				if depth == 1 {
					requested[node.Name] = true
				}
			} else if node.Name == (xml.Name{Space: davXMLNamespace, Local: "prop"}) {
				insideProp = true
			}
		case xml.EndElement:
			if insideProp {
				if depth == 0 {
					insideProp = false
				} else {
					depth--
				}
			}
		}
	}
	return requested, nil
}

func writeSchedulingMultistatus(w http.ResponseWriter, resources []schedulingXMLResource, requested map[xml.Name]bool) {
	var response strings.Builder
	response.WriteString(`<D:multistatus xmlns:D="DAV:" xmlns:C="` + caldavXMLNamespace + `">`)
	for _, resource := range resources {
		response.WriteString("<D:response>" + schedulingHref(resource.href))
		if resource.status != 0 {
			response.WriteString("<D:status>HTTP/1.1 " + strconv.Itoa(resource.status) + " " + http.StatusText(resource.status) + "</D:status></D:response>")
			continue
		}
		var found, missing strings.Builder
		if len(requested) == 0 {
			for name, value := range resource.properties {
				writeSchedulingProperty(&found, name, value)
			}
		} else {
			for name := range requested {
				if value, ok := resource.properties[name]; ok {
					writeSchedulingProperty(&found, name, value)
				} else {
					writeSchedulingProperty(&missing, name, "")
				}
			}
		}
		if found.Len() != 0 {
			response.WriteString("<D:propstat><D:prop>" + found.String() + "</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>")
		}
		if missing.Len() != 0 {
			response.WriteString("<D:propstat><D:prop>" + missing.String() + "</D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>")
		}
		response.WriteString("</D:response>")
	}
	response.WriteString("</D:multistatus>")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, response.String())
}

func writeSchedulingProperty(out *strings.Builder, name xml.Name, value string) {
	prefix := "D:"
	if name.Space == caldavXMLNamespace {
		prefix = "C:"
	} else if name.Space != davXMLNamespace {
		if name.Space != "" {
			out.WriteString(`<X:` + name.Local + ` xmlns:X="` + html.EscapeString(name.Space) + `">` + value + `</X:` + name.Local + `>`)
		}
		return
	}
	tag := prefix + name.Local
	out.WriteString("<" + tag + ">" + value + "</" + tag + ">")
}

func schedulingHref(value string) string { return "<D:href>" + schedulingEscape(value) + "</D:href>" }

func schedulingEscape(value string) string {
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(value))
	return escaped.String()
}

func writeSchedulingOptions(w http.ResponseWriter, allow string, auto bool) {
	dav := "1, 3, calendar-access"
	if auto {
		dav += ", calendar-auto-schedule"
	}
	w.Header().Set("DAV", dav)
	w.Header().Set("Allow", allow)
	w.WriteHeader(http.StatusNoContent)
}

func writeSchedulingError(w http.ResponseWriter, err error, fallback int) {
	status := fallback
	if err != nil {
		var apiError api2go.HTTPError
		var schedulingError *schedulingHTTPError
		if errors.As(err, &apiError) {
			status = apiError.Status()
		} else if errors.As(err, &schedulingError) {
			status = schedulingError.Status()
		}
	}
	if err == nil {
		err = fmt.Errorf("scheduling resource unavailable")
	}
	if status >= http.StatusInternalServerError {
		http.Error(w, "scheduling resource unavailable", status)
		return
	}
	http.Error(w, err.Error(), status)
}
