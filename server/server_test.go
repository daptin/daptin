package server

import "testing"
import (
	"time"
)

type Blog struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	Posts         []*Post   `json:"posts"`
	CurrentPost   *Post     `json:"current_post"`
	CurrentPostId int       `json:"current_post_id"`
	CreatedAt     time.Time `json:"created_at"`
	ViewCount     int       `json:"view_count"`
}

type Post struct {
	ID       int        `json:"id"`
	BlogID   int        `json:"blog_id"`
	Title    string     `json:"title"`
	Body     string     `json:"body"`
	Comments []*Comment `json:"comments"`
}

type Comment struct {
	Id     int    `json:"id"`
	PostID int    `json:"post_id"`
	Body   string `json:"body"`
	Likes  uint   `json:"likes_count,omitempty"`
}

func JsonApiTests(t *testing.T) {

}
