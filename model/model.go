package model

// Tag is tag for the bookmark
type Tag struct {
	ID         int64  `db:"id"          json:"id"`
	Name       string `db:"name"        json:"name"`
	NBookmarks int64  `db:"n_bookmarks" json:"nBookmarks"`
	Deleted    bool   `json:"-"`
}

// Bookmark is record of a specified URL
type Bookmark struct {
	ID          int64  `db:"id"            json:"id"`
	URL         string `db:"url"           json:"url"`
	Title       string `db:"title"         json:"title"`
	ImageURL    string `db:"image_url"     json:"imageURL"`
	Excerpt     string `db:"excerpt"       json:"excerpt"`
	Author      string `db:"author"        json:"author"`
	MinReadTime int    `db:"min_read_time" json:"minReadTime"`
	MaxReadTime int    `db:"max_read_time" json:"maxReadTime"`
	Modified    string `db:"modified"      json:"modified"`
	Content     string `db:"content"       json:"-"`
	HTML        string `db:"html"          json:"-"`
	Tags        []Tag  `json:"tags"`
    IsVideo     bool   `db:"isvideo"       json:"isvideo"`
    Downloaded  bool   `db:"downloaded"  json:"downloaded"`
}


type Video struct {
   ID       int64  `db:"id"     json:"id"`
   Downloaded bool `db:"downloaded" json:"downloaded"`
   Filename  string `db:"filename" json:"filename"`
}

// Account is account for accessing bookmarks from web interface
type Account struct {
	ID       int64  `db:"id"       json:"id"`
	Username string `db:"username" json:"username"`
	Password string `db:"password" json:"password"`
}

// LoginRequest is login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}
