package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	fieldtypes "github.com/daptin/daptin/server/columntypes"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func feedEnabled(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int64:
		if v == 0 || v == 1 {
			return v == 1, nil
		}
	case string:
		return strconv.ParseBool(v)
	}
	return false, fmt.Errorf("invalid feed boolean value %T", value)
}

func feedText(row map[string]interface{}, name string) (string, error) {
	value, ok := row[name].(string)
	if !ok {
		return "", fmt.Errorf("invalid feed field %s: %T", name, row[name])
	}
	return value, nil
}

func feedCreatedAt(value interface{}) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		if parsed, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return parsed, nil
		}
		parsed, _, err := fieldtypes.GetTime(v)
		if err == nil {
			return parsed, nil
		}
		// The stream's dataframe converts time.Time values to time.Time.String().
		// Its final zone label may itself be a numeric offset.
		if zone := strings.LastIndexByte(v, ' '); zone >= 0 {
			return time.Parse("2006-01-02 15:04:05.999999999 -0700", v[:zone])
		}
		return time.Time{}, err
	default:
		return time.Time{}, fmt.Errorf("invalid feed created_at: %T", value)
	}
}

func feedItem(row map[string]interface{}) (*feeds.Item, error) {
	title, err := feedText(row, "title")
	if err != nil {
		return nil, err
	}
	link, err := feedText(row, "link")
	if err != nil {
		return nil, err
	}
	description, err := feedText(row, "description")
	if err != nil {
		return nil, err
	}
	authorName, err := feedText(row, "author_name")
	if err != nil {
		return nil, err
	}
	authorEmail, err := feedText(row, "author_email")
	if err != nil {
		return nil, err
	}
	created, err := feedCreatedAt(row["created_at"])
	if err != nil {
		return nil, err
	}
	return &feeds.Item{
		Title: title, Link: &feeds.Link{Href: link}, Description: description,
		Author: &feeds.Author{Name: authorName, Email: authorEmail}, Created: created,
	}, nil
}

func CreateFeedHandler(cruds map[string]*resource.DbResource, streams []*resource.StreamProcessor, transaction *sqlx.Tx) func(*gin.Context) {

	streamMap := make(map[string]*resource.StreamProcessor)

	for _, stream := range streams {
		streamMap[stream.GetName()] = stream
	}

	feedsInfo, err := cruds["feed"].GetAllRawObjectsWithTransaction("feed", transaction)
	resource.CheckErr(err, "Failed to load feeds")
	streamInfos, err := cruds["stream"].GetAllRawObjectsWithTransaction("stream", transaction)
	resource.CheckErr(err, "Failed to load stream")

	feedMap := make(map[string]map[string]interface{})
	streamInfoMap := make(map[int64]map[string]interface{})
	for _, feed := range feedsInfo {
		if name, ok := feed["feed_name"].(string); ok {
			feedMap[name] = feed
		}
	}
	for _, stream := range streamInfos {
		if id, err := resource.ResourceRowInt64(stream["id"]); err == nil {
			streamInfoMap[id] = stream
		}
	}

	return func(c *gin.Context) {
		feedName, feedExtension, ok := strings.Cut(c.Param("feedname"), ".")
		if !ok || feedName == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid feed request"})
			return
		}
		feedExtension = strings.ToLower(feedExtension)
		if feedExtension != "rss" && feedExtension != "atom" && feedExtension != "json" {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		feedInfo, ok := feedMap[feedName]
		if !ok || feedInfo == nil {
			c.AbortWithStatus(404)
			return
		}

		enabled, err := feedEnabled(feedInfo["enable"])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed configuration"})
			return
		}
		if !enabled {
			c.AbortWithStatus(404)
			return
		}
		formatEnabled, err := feedEnabled(feedInfo["enable_"+feedExtension])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed configuration"})
			return
		}
		if !formatEnabled {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		streamID, err := resource.ResourceRowInt64(feedInfo["stream_id"])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed stream"})
			return
		}

		streamInfo, ok := streamInfoMap[streamID]
		if !ok {
			c.AbortWithStatus(404)
			return
		}

		streamName, err := feedText(streamInfo, "stream_name")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed stream"})
			return
		}
		streamProcessor, ok := streamMap[streamName]
		if !ok {
			c.AbortWithStatus(404)
			return
		}

		pageSize, err := resource.ResourceRowInt64(feedInfo["page_size"])
		if err != nil || pageSize < 1 {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed page_size"})
			return
		}

		req := api2go.Request{
			PlainRequest: c.Request,
			QueryParams: map[string][]string{
				"page[size]": {strconv.FormatInt(pageSize, 10)},
			},
		}

		_, rows, err := streamProcessor.PaginatedFindAll(req)

		if err != nil || rows == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to read feed stream"})
			return
		}

		metadata, err := feedItem(feedInfo)
		if err != nil {
			log.Errorf("invalid feed configuration for %s: %v", feedName, err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed configuration"})
			return
		}
		feed := &feeds.Feed{
			Title: metadata.Title, Link: metadata.Link,
			Description: metadata.Description, Author: metadata.Author,
			Created: metadata.Created,
		}

		result, ok := rows.Result().([]api2go.Api2GoModel)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed stream result"})
			return
		}
		for _, model := range result {
			item, err := feedItem(model.GetAttributes())
			if err != nil {
				log.Errorf("invalid feed stream item for %s: %v", feedName, err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid feed stream item"})
				return
			}
			feed.Items = append(feed.Items, item)
		}

		var output, contentType string
		switch feedExtension {
		case "rss":
			contentType = "application/xml"
			output, err = feed.ToRss()
		case "atom":
			contentType = "application/xml"
			output, err = feed.ToAtom()
		case "json":
			contentType = "application/json"
			output, err = feed.ToJSON()
		}

		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to render feed"})
			return
		}
		c.Data(http.StatusOK, contentType, []byte(output))

	}
}
