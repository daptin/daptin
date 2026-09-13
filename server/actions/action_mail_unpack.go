package actions

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type mailUnpackActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *mailUnpackActionPerformer) Name() string {
	return "mail.unpack"
}

func (d *mailUnpackActionPerformer) DoAction(_ actionresponse.Outcome, inFields map[string]interface{}, _ *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	mailValue, ok := inFields["mail"]
	if !ok || mailValue == nil {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("mail is required"), "mail is required", http.StatusBadRequest)}
	}
	limits, err := mailUnpackLimits(inFields)
	if err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, err.Error(), http.StatusBadRequest)}
	}
	mailBytes, err := d.cruds["mail"].MailColumnBytes("mail", "mail", mailValue)
	if err != nil {
		return nil, nil, []error{mailUnpackHTTPError(err)}
	}
	unpacked, err := normalizeMail(mailBytes, limits)
	if err != nil {
		return nil, nil, []error{mailUnpackHTTPError(err)}
	}

	parts := make([]map[string]interface{}, 0, len(unpacked.Parts))
	for _, part := range unpacked.Parts {
		name := strings.TrimSpace(part.Filename)
		if name == "" {
			name = fmt.Sprintf("part-%03d", part.Index)
		}
		parts = append(parts, map[string]interface{}{
			"index":               part.Index,
			"kind":                part.Kind,
			"media_type":          part.MediaType,
			"filename":            part.Filename,
			"content_id":          part.ContentID,
			"content_disposition": part.ContentDisposition,
			"decoded_size":        part.DecodedSize,
			"sha256":              part.SHA256,
			"content": []interface{}{map[string]interface{}{
				"name":     name,
				"path":     "",
				"type":     part.MediaType,
				"contents": base64.StdEncoding.EncodeToString(part.Content),
			}},
		})
	}

	attributes := map[string]interface{}{
		"preferred_text_body": unpacked.PreferredTextBody,
		"optional_html_body":  unpacked.HTMLBody,
		"parts":               parts,
	}
	model := api2go.NewApi2GoModelWithData("mail.unpack", nil, 0, nil, attributes)
	return resource.NewResponse(nil, model, http.StatusOK, nil), nil, nil
}

func mailUnpackLimits(inFields map[string]interface{}) (mailUnpackLimitsConfig, error) {
	limits := defaultMailUnpackLimits()
	var err error
	if limits.MaxRawBytes, err = mailUnpackInt64("max_raw_size", inFields["max_raw_size"], limits.MaxRawBytes); err != nil {
		return limits, err
	}
	if limits.MaxDecodedBytes, err = mailUnpackInt64("max_decoded_size", inFields["max_decoded_size"], limits.MaxDecodedBytes); err != nil {
		return limits, err
	}
	if limits.MaxPartBytes, err = mailUnpackInt64("max_part_size", inFields["max_part_size"], limits.MaxPartBytes); err != nil {
		return limits, err
	}
	if limits.MaxHeaderBytes, err = mailUnpackInt64("max_header_size", inFields["max_header_size"], limits.MaxHeaderBytes); err != nil {
		return limits, err
	}
	maxParts, err := mailUnpackInt64("max_parts", inFields["max_parts"], int64(limits.MaxParts))
	if err != nil {
		return limits, err
	}
	limits.MaxParts = int(maxParts)
	maxDepth, err := mailUnpackInt64("max_nesting_depth", inFields["max_nesting_depth"], int64(limits.MaxNestingDepth))
	if err != nil {
		return limits, err
	}
	limits.MaxNestingDepth = int(maxDepth)
	return limits, nil
}

func mailUnpackInt64(name string, value interface{}, maximum int64) (int64, error) {
	if value == nil {
		return maximum, nil
	}
	var parsed int64
	var err error
	switch typed := value.(type) {
	case int:
		parsed = int64(typed)
	case int64:
		parsed = typed
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("%s must be a positive integer no greater than %d", name, maximum)
		}
		parsed = int64(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		parsed, err = strconv.ParseInt(trimmed, 10, 64)
	default:
		err = errors.New("invalid limit")
	}
	if err != nil || parsed <= 0 || parsed > maximum {
		return 0, fmt.Errorf("%s must be a positive integer no greater than %d", name, maximum)
	}
	return parsed, nil
}

func mailUnpackHTTPError(err error) error {
	var unpackErr *mailUnpackErrorDetail
	if !errors.As(err, &unpackErr) {
		return api2go.NewHTTPError(errors.New("mail unavailable"), "mail unavailable", http.StatusUnprocessableEntity)
	}
	status := http.StatusUnprocessableEntity
	if strings.Contains(unpackErr.Code, "too_large") || unpackErr.Code == "mail_too_many_parts" || unpackErr.Code == "mail_nesting_too_deep" {
		status = http.StatusRequestEntityTooLarge
	}
	return api2go.NewHTTPError(errors.New(unpackErr.Code), unpackErr.Code, status)
}

func NewMailUnpackActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	return &mailUnpackActionPerformer{cruds: cruds}, nil
}
