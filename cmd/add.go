package cmd

import (
	"fmt"
	"github.com/RadhiFadlillah/go-readability"
	"github.com/RadhiFadlillah/shiori/model"
	"github.com/rylio/ytdl"
	"github.com/spf13/cobra"
	nurl "net/url"
	"os"
	"strings"
	"time"
)

var (
	addCmd = &cobra.Command{
		Use:   "add url",
		Short: "Bookmark the specified URL",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Read flag and arguments
			url := args[0]
			title, _ := cmd.Flags().GetString("title")
			excerpt, _ := cmd.Flags().GetString("excerpt")
			tags, _ := cmd.Flags().GetStringSlice("tags")
			offline, _ := cmd.Flags().GetBool("offline")

			// Create bookmark item
			bookmark := model.Bookmark{
				URL:     url,
				Title:   normalizeSpace(title),
				Excerpt: normalizeSpace(excerpt),
			}

			bookmark.Tags = make([]model.Tag, len(tags))
			for i, tag := range tags {
				bookmark.Tags[i].Name = strings.TrimSpace(tag)
			}

			// Save new bookmark
			result, err := addBookmark(bookmark, offline)
			if err != nil {
				cError.Println(err)
				return
			}

			printBookmark(result)
		},
	}
)

func init() {
	addCmd.Flags().StringP("title", "i", "", "Custom title for this bookmark.")
	addCmd.Flags().StringP("excerpt", "e", "", "Custom excerpt for this bookmark.")
	addCmd.Flags().StringSliceP("tags", "t", []string{}, "Comma-separated tags for this bookmark.")
	addCmd.Flags().BoolP("offline", "o", false, "Save bookmark without fetching data from internet.")
	rootCmd.AddCommand(addCmd)
}

func addBookmark(base model.Bookmark, offline bool) (book model.Bookmark, err error) {
	// Prepare initial result
	book = base

	// Make sure URL valid
	parsedURL, err := nurl.ParseRequestURI(book.URL)
	if err != nil || parsedURL.Host == "" {
		return book, fmt.Errorf("URL is not valid")
	}

	// Clear UTM parameters from URL
	book.URL, err = clearUTMParams(parsedURL)
	if err != nil {
		return book, err
	}

	// Fetch data from internet
	if !offline {
		article, err := readability.Parse(book.URL, 10*time.Second)
		if err != nil {
			cError.Println("Failed to fetch article from internet:", err)
			if book.Title == "" {
				book.Title = "Untitled"
			}
		} else {
			book.URL = article.URL
			book.ImageURL = article.Meta.Image
			book.Author = article.Meta.Author
			book.MinReadTime = article.Meta.MinReadTime
			book.MaxReadTime = article.Meta.MaxReadTime
			book.Content = article.Content
			book.HTML = article.RawContent

			if book.Title == "" {
				book.Title = article.Meta.Title
			}

			if book.Excerpt == "" {
				book.Excerpt = article.Meta.Excerpt
			}
		}
	}

	// Save to database
	book.ID, err = DB.CreateBookmark(book)

	if strings.Contains(book.URL, "youtube.com") {
		video := model.Video{}

		book.IsVideo = true
		filename, err := youtubedl(book.URL)

		if err != nil {
			return book, err
		}

		video.Filename = filename
		video.Downloaded = true
		video.ID, err = DB.CreateVideo(book.ID, video)
	}

	return book, err
}

func normalizeSpace(str string) string {
	return strings.Join(strings.Fields(str), " ")
}

func clearUTMParams(url *nurl.URL) (string, error) {
	newQuery := nurl.Values{}
	for key, value := range url.Query() {
		if strings.HasPrefix(key, "utm_") {
			continue
		}

		newQuery[key] = value
	}

	url.RawQuery = newQuery.Encode()
	return url.String(), nil
}

func youtubedl(url string) (filename string, err error) {

	vid, err := ytdl.GetVideoInfo(url)
	cIndex.Println("Link is video")
	cTitle.Println("Downloading " + vid.Title + "...")
	filename = vid.Title + ".mp4"

	formatsFound := vid.Formats.Best(ytdl.FormatResolutionKey)
	if len(formatsFound) > 0 {
		file, err := os.Create(filename)
		if err != nil {
			return "", err
		}
		defer file.Close()
		err = vid.Download(formatsFound[0], file)
		if err != nil {
			return "", err
		}
	}

	return filename, err
}
