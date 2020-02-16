package server

import (
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"
	"log"
	"net/http"
	"strings"
	"time"
)

func CreateFeedHandler(cruds map[string]*resource.DbResource, streams []*resource.StreamProcessor) func(*gin.Context) {

	streamMap := make(map[string]*resource.StreamProcessor)

	for _, stream := range streams {
		streamMap[stream.GetName()] = stream
	}

	feedsInfo, err := cruds["feed"].GetAllRawObjects("feed")
	if err != nil {
		log.Printf("Failed to load stream")
	}
	streamInfos, err := cruds["feed"].GetAllRawObjects("stream")
	if err != nil {
		log.Printf("Failed to load feeds")
	}
	feedMap := make(map[string]map[string]interface{})
	streamInfoMap := make(map[string]map[string]interface{})
	for _, feed := range feedsInfo {
		feedMap[feed["feed_name"].(string)] = feed
	}
	for _, stream := range streamInfos {
		streamInfoMap[stream["id"].(string)] = stream
	}

	return func(c *gin.Context) {
		var feedName = c.Param("feedname")

		var parts = strings.Split(feedName, ".")
		feedName = parts[0]
		feedExtension := parts[1]

		feedInfo, ok := feedMap[feedName]
		if !ok || feedInfo == nil {
			c.AbortWithStatus(404)
			return
		}

		if !feedInfo["enabled"].(bool) {
			c.AbortWithStatus(404)
			return
		}
		streamId, ok := feedInfo["stream_id"].(string)
		if !ok {
			c.AbortWithStatus(404)
			return
		}

		streamInfo, ok := streamInfoMap[streamId]
		if !ok {
			c.AbortWithStatus(404)
			return
		}

		streamProcessor, ok := streamMap[streamInfo["stream_name"].(string)]
		if !ok {
			c.AbortWithStatus(404)
			return
		}

		pageSize := feedInfo["page_size"].(int64)

		pr := &http.Request{
			Method: "GET",
		}

		pr = pr.WithContext(c.Request.Context())

		req := api2go.Request{
			PlainRequest: pr,
			QueryParams: map[string][]string{
				"page[size]": []string{fmt.Sprintf("%v", pageSize)},
			},
		}

		_, rows, err := streamProcessor.PaginatedFindAll(req)

		if err != nil {
			c.AbortWithError(500, err)
			return
		}

		feed := &feeds.Feed{
			Title:       feedInfo["title"].(string),
			Link:        &feeds.Link{Href: feedInfo["link"].(string)},
			Description: feedInfo["description"].(string),
			Author:      &feeds.Author{Name: feedInfo["author_name"].(string), Email: feedInfo["author_email"].(string)},
			Created:     feedInfo["created_at"].(time.Time),
		}

		feedItems := make([]*feeds.Item, 0)

		for _, rowInterface := range rows.Result().([]interface{}) {

			row, ok := rowInterface.(map[string]interface{})
			if !ok {
				resource.CheckErr(err, "row was not of type map[string]interface{}")
				continue
			}
			feedItems = append(feedItems, &feeds.Item{
				Title:       row["title"].(string),
				Link:        &feeds.Link{Href: row["link"].(string)},
				Description: row["description"].(string),
				Author:      &feeds.Author{Name: row["author_name"].(string), Email: row["author_email"].(string)},
				Created:     row["created_at"].(time.Time),
			})

		}

		feed.Items = feedItems

		var output string
		switch strings.ToLower(feedExtension) {
		case "rss":
			output, err = feed.ToRss()
		case "atom":
			output, err = feed.ToAtom()
		case "json":
			output, err = feed.ToJSON()
		}

		resource.CheckErr(err, "Failed to generate feed [%v]", feedInfo)

		c.Writer.WriteString(output)
		c.AbortWithStatus(200)

	}
}
