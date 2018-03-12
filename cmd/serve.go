package cmd

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	fp "path/filepath"
	"strings"
	"time"

	"github.com/s-frostick/shiori/assets"
	"github.com/s-frostick/shiori/model"
	"github.com/dgrijalva/jwt-go"
	"github.com/dgrijalva/jwt-go/request"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var (
	jwtKey   []byte
	tplCache *template.Template
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Serve web app for managing bookmarks",
		Long: "Run a simple annd performant web server which serves the site for managing bookmarks." +
			"If --port flag is not used, it will use port 8080 by default.",
		Run: func(cmd *cobra.Command, args []string) {
			// Create JWT key
			jwtKey = make([]byte, 32)
			_, err := rand.Read(jwtKey)
			if err != nil {
				cError.Println("Failed to generate key for token")
				return
			}

			// Prepare template
			funcMap := template.FuncMap{
				"html": func(s string) template.HTML {
					return template.HTML(s)
				},
			}

			tplFile, _ := assets.ReadFile("cache.html")
			tplCache, err = template.New("cache.html").Funcs(funcMap).Parse(string(tplFile))
			if err != nil {
				cError.Println("Failed to generate HTML template")
				return
			}

			// Create router
			router := httprouter.New()

			router.GET("/js/*filepath", serveFiles)
			router.GET("/res/*filepath", serveFiles)
			router.GET("/css/*filepath", serveFiles)
			router.GET("/webfonts/*filepath", serveFiles)

			router.GET("/", serveIndexPage)
			router.GET("/login", serveLoginPage)
			router.GET("/bookmark/:id", serveBookmarkCache)

			router.POST("/api/login", apiLogin)
			router.GET("/api/bookmarks", apiGetBookmarks)
			router.GET("/api/tags", apiGetTags)
			router.POST("/api/bookmarks", apiInsertBookmarks)
			router.PUT("/api/bookmarks", apiUpdateBookmarks)
			router.DELETE("/api/bookmarks", apiDeleteBookmarks)

			// Route for panic
			router.PanicHandler = func(w http.ResponseWriter, r *http.Request, arg interface{}) {
				http.Error(w, fmt.Sprint(arg), 500)
			}

			port, _ := cmd.Flags().GetInt("port")
			url := fmt.Sprintf(":%d", port)
			logrus.Infoln("Serve shiori in", url)
			svr := &http.Server{
				Addr:         url,
				Handler:      router,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 20 * time.Second,
			}
			logrus.Fatalln(svr.ListenAndServe())
		},
	}
)

func init() {
	serveCmd.Flags().IntP("port", "p", 8080, "Port that used by server")
	rootCmd.AddCommand(serveCmd)
}

func serveFiles(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Read asset path
	path := r.URL.Path
	if path[0:1] == "/" {
		path = path[1:]
	}

	// Load asset
	asset, err := assets.ReadFile(path)
	checkError(err)

	// Set response header content type
	ext := fp.Ext(path)
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}

	// Serve asset
	buffer := bytes.NewBuffer(asset)
	io.Copy(w, buffer)
}

func serveIndexPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Check token
	err := checkToken(r)
	if err != nil {
		redirectPage(w, r, "/login")
		return
	}

	asset, _ := assets.ReadFile("index.html")
	w.Header().Set("Content-Type", "text/html")
	buffer := bytes.NewBuffer(asset)
	io.Copy(w, buffer)

}

func serveLoginPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Check token
	err := checkToken(r)
	if err == nil {
		redirectPage(w, r, "/")
		return
	}

	asset, _ := assets.ReadFile("login.html")
	w.Header().Set("Content-Type", "text/html")
	buffer := bytes.NewBuffer(asset)
	io.Copy(w, buffer)
}

func serveBookmarkCache(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Read param in URL
	id := ps.ByName("id")

	// Read bookmarks
	bookmarks, err := DB.GetBookmarks(true, id)
	checkError(err)

	if len(bookmarks) == 0 {
		panic(fmt.Errorf("No bookmark with matching index"))
	}

	// Read template
	err = tplCache.Execute(w, &bookmarks[0])
	checkError(err)
}

func apiLogin(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Decode request
	var request model.LoginRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	checkError(err)

	// Get account data from database
	accounts, err := DB.GetAccounts(request.Username, true)
	if err != nil || len(accounts) == 0 {
		panic(fmt.Errorf("Account does not exist"))
	}

	// Compare password with database
	account := accounts[0]
	err = bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(request.Password))
	if err != nil {
		panic(fmt.Errorf("Username and password don't match"))
	}

	// Calculate expiration time
	nbf := time.Now()
	exp := time.Now().Add(12 * time.Hour)
	if request.Remember {
		exp = time.Now().Add(7 * 24 * time.Hour)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"nbf": nbf.Unix(),
		"exp": exp.Unix(),
		"sub": account.ID,
	})

	tokenString, err := token.SignedString(jwtKey)
	checkError(err)

	// Return token
	fmt.Fprint(w, tokenString)
}

func apiGetBookmarks(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Get query parameter
	keyword := r.URL.Query().Get("keyword")
	strTags := r.URL.Query().Get("tags")
	tags := strings.Fields(strTags)

	// Check token
	err := checkAPIToken(r)
	checkError(err)

	// Fetch all bookmarks
	bookmarks, err := DB.SearchBookmarks(true, keyword, tags...)
	checkError(err)

	err = json.NewEncoder(w).Encode(&bookmarks)
	checkError(err)
}

func apiGetTags(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

	// Check token
	err := checkAPIToken(r)
	checkError(err)

	// Fetch all tags
	tags, err := DB.GetTags()
	checkError(err)

	err = json.NewEncoder(w).Encode(&tags)
	checkError(err)
}

func apiInsertBookmarks(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Check token
	err := checkAPIToken(r)
	checkError(err)

	// Decode request
	request := model.Bookmark{}
	err = json.NewDecoder(r.Body).Decode(&request)
	checkError(err)

	// Save bookmark
	book, err := addBookmark(request, false)
	checkError(err)

	// Return new saved result
	err = json.NewEncoder(w).Encode(&book)
	checkError(err)
}

func apiUpdateBookmarks(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Get url parameter
	_, dontOverwrite := r.URL.Query()["dont-overwrite"]
	overwrite := !dontOverwrite

	// Check token
	err := checkAPIToken(r)
	checkError(err)

	// Decode request
	request := model.Bookmark{}
	err = json.NewDecoder(r.Body).Decode(&request)
	checkError(err)

	// Convert tags and ID
	id := []string{fmt.Sprintf("%d", request.ID)}

	// Update bookmark
	bookmarks, err := updateBookmarks(id, request, false, overwrite)
	checkError(err)

	// Return new saved result
	err = json.NewEncoder(w).Encode(&bookmarks[0])
	checkError(err)
}

func apiDeleteBookmarks(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// Check token
	err := checkAPIToken(r)
	checkError(err)

	// Decode request
	request := []string{}
	err = json.NewDecoder(r.Body).Decode(&request)
	checkError(err)

	// Delete bookmarks
	err = DB.DeleteBookmarks(request...)
	checkError(err)

	fmt.Fprint(w, request)
}

func checkToken(r *http.Request) error {
	tokenCookie, err := r.Cookie("token")
	if err != nil {
		return fmt.Errorf("Token does not exist")
	}

	token, err := jwt.Parse(tokenCookie.Value, jwtKeyFunc)
	if err != nil {
		return err
	}

	claims := token.Claims.(jwt.MapClaims)
	return claims.Valid()
}

func checkAPIToken(r *http.Request) error {
	token, err := request.ParseFromRequest(r, request.AuthorizationHeaderExtractor, jwtKeyFunc)
	if err != nil {
		return err
	}

	claims := token.Claims.(jwt.MapClaims)
	return claims.Valid()
}

func jwtKeyFunc(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("Unexpected signing method")
	}

	return jwtKey, nil
}

func redirectPage(w http.ResponseWriter, r *http.Request, url string) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	http.Redirect(w, r, url, 301)
}
