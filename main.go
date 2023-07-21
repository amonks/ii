package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"

	_ "github.com/mattn/go-sqlite3"

	"strings"
	"time"
)

const (
	archivePath = "/mypool/tank/mirror/reddit/"
	dbPath      = "/mypool/tank/mirror/reddit/.reddit.db"
	clientID    = "-RT9cp4AERMlAEhwR01isQ"
	secret      = "mgo2f7coeJj31sIZDsdIlLZfjBfSiA"
	redgifToken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJhdXRoLXNlcnZpY2UiLCJpYXQiOjE2ODYwOTA5NTcsImF6cCI6IjE4MjNjMzFmN2QzLTc0NWEtNjU4OS0wMDA1LWQ4ZThmZTBhNDRjMiIsImV4cCI6MTY4NjE3NzM1Nywic3ViIjoiY2xpZW50LzE4MjNjMzFmN2QzLTc0NWEtNjU4OS0wMDA1LWQ4ZThmZTBhNDRjMiIsInNjb3BlcyI6InJlYWQiLCJ2YWxpZF9hZGRyIjoiNjYuMTkwLjE1Ny4xMjYiLCJ2YWxpZF9hZ2VudCI6ImN1cmwvOC4wLjEiLCJyYXRlIjotMX0.UBQBeKnyH9tYD1BMhvwiHPA1EvTCdM-LUGcqC712h7yMNu7WvpA_m4aGyRMBTB0P3qiBGYFppX8UCgpCeSUpTX1Qtr_QEypINqAYkAMFqQ5iDtP2WIs8NHf3r3AblwwYdLzmnBZXnsBF1OHx1AC9rV49w7e6H4A9PC3DvbbX6C7Uz_f6lt3WqcNJYDB2Or6jzB9RH6Ta7iu4CNXbNIpd24ilUEXwyj_cOCD0IOH9cCAOI9c7H_69wWGQ5A7D6n46rF6f6e9HV45PEZQyhxMcFjFhBxPZyKBIjDxix4LuwW9uuXb9SkrA4hOjl7bQH0PT8ApAwNUSo1g1lRmI9le4gA"
)

type app struct {
	accessToken accessToken
	db          *sql.DB
}

type post struct {
	name      string
	title     string
	author    string
	subreddit string
	url       string
	permalink string

	json *[]byte

	status      string
	filetype    *string
	archivepath *string
}

func (p *post) Embed() template.HTML {
	if p.filetype == nil {
		return "no"
	}
	src := strings.Replace(*p.archivepath, "/mypool/tank/mirror/reddit/", "/media/", 1)
	switch *p.filetype {
	case ".gif":
		fallthrough
	case ".jpg":
		fallthrough
	case ".png":
		tmpl, _ := template.New("gif").Parse(`<img src="{{.Src}}" />`)
		w := strings.Builder{}
		tmpl.Execute(&w, struct{ Src string }{Src: src})
		return template.HTML(w.String())
	case ".mp4":
		tmpl, _ := template.New("mp4").Parse(`<video controls autoplay loop><source src="{{ .Src}}" /></video>`)
		w := strings.Builder{}
		tmpl.Execute(&w, struct{ Src string }{Src: src})
		return template.HTML(w.String())
	default:
		return template.HTML("unexpected filetype: " + *p.filetype)
	}
}

func (c *app) getPost(name string) (*post, error) {
	var p post
	p.name = name
	row := c.db.QueryRow("select title, author, subreddit, url, permalink, status, filetype, archivepath from posts where name = ?", name)
	if err := row.Scan(&p.title, &p.author, &p.subreddit, &p.url, &p.permalink, &p.status, &p.filetype, &p.archivepath); err != nil {
		return nil, fmt.Errorf("error getting %s: %w", name, err)
	}
	return &p, nil
}

func (c *app) loadPostJson(p *post) error {
	var bs []byte
	row := c.db.QueryRow("select json from posts where name = ?", p.name)
	if err := row.Scan(&bs); err != nil {
		return err
	}
	p.json = &bs
	return nil
}

func (c *app) updatePost(p *post) error {
	if _, err := c.db.Exec("update posts set title=?, author=?, subreddit=?, url=?, permalink=?, filetype=?, status=?, archivepath=? where name = ?",
		p.title, p.author, p.subreddit, p.url, p.permalink, p.filetype, p.status, p.archivepath, p.name); err != nil {
		return fmt.Errorf("error updating %s: %w", p.name, err)
	}
	return nil
}

var errCollision = errors.New("Collision")

func (c *app) insertPost(p *post) error {
	if _, err := c.db.Exec("insert into posts values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);",
		p.name, p.title, p.author, p.subreddit, p.url, p.permalink, p.json, p.status, p.filetype, p.archivepath,
	); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: posts.name") {
			return errCollision
		}
		return fmt.Errorf("error inserting %s: %w", p.name, err)
	}
	return nil
}

func (c *app) migrate() error {
	if _, err := c.db.Exec(`create table if not exists posts (
		name         text primary key not null,
		title        text not null,
		author       text not null,
		subreddit    text not null,
		url          text not null,
		permalink    text not null,

		json         text not null,

		status       text not null,
		filetype     text,
		archivepath  text
	);`); err != nil {
		return fmt.Errorf("migration error: %w", err)
	}
	return nil
}

type accessToken struct {
	token     string
	expiresAt time.Time
}

func (t accessToken) isValid() bool {
	return t.expiresAt.After(time.Now())
}

func (c *app) withAccessToken() {
	if !c.accessToken.isValid() {
		c.accessToken = c.getAccessToken()
	}
}

func (c *app) request(method, url string, body string, transformReq func(*http.Request)) ([]byte, error) {
	var req *http.Request
	if body == "" {
		req_, err := http.NewRequest(method, url, nil)
		if err != nil {
			return nil, err
		}
		req = req_
	} else {
		req_, err := http.NewRequest(method, url, strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req = req_
	}

	transformReq(req)

	res, err := httpWithBackoff(req, 256)
	if err != nil {
		return nil, err
	}
	bs, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("[%s]: %s", res.Status, bs)
	}

	return bs, nil
}

func (c *app) getAccessToken() accessToken {
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", "richbizzness")
	data.Set("password", "gYnzov-tosmyp-2tidxo")
	body := data.Encode()

	bs, err := c.request("POST", "https://www.reddit.com/api/v1/access_token", body, func(req *http.Request) {
		req.SetBasicAuth(clientID, secret)
		req.Header.Set("Content-type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-agent", "curl/8.0.1")
	})
	if err != nil {
		panic(err)
	}

	var responseBody struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	json.Unmarshal(bs, &responseBody)

	return accessToken{
		token:     responseBody.AccessToken,
		expiresAt: time.Now().Add(time.Duration(responseBody.ExpiresIn) * time.Second),
	}
}

func (c *app) getSaved(after string) ([]byte, error) {
	c.withAccessToken()
	return c.request("GET", "https://oauth.reddit.com/user/richbizzness/saved?after="+after, "", func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+c.accessToken.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-agent", "arch")
	})
}

func (c *app) refreshSaved() error {
	next := "BEGIN"
	for next != "" {
		if next == "BEGIN" {
			next = ""
		}

		fmt.Println("handle page " + next)

		bs, err := c.getSaved(next)
		if err != nil {
			return fmt.Errorf("fetch error: %w", err)
		}

		var response struct {
			Data struct {
				After    string
				Children []struct {
					Data json.RawMessage
				}
			}
		}
		if err := json.Unmarshal(bs, &response); err != nil {
			return fmt.Errorf("json error: %w", err)
		}

		for _, child := range response.Data.Children {
			var item struct {
				Name      string
				Title     string
				Author    string
				Subreddit string
				URL       string
				Permalink string
			}
			jsonBs := []byte(child.Data)
			if err := json.Unmarshal(jsonBs, &item); err != nil {
				return fmt.Errorf("json error: %w", err)
			}

			if err := c.insertPost(&post{
				name:      item.Name,
				title:     item.Title,
				author:    item.Author,
				subreddit: item.Subreddit,
				url:       item.URL,
				permalink: item.Permalink,
				json:      &jsonBs,
			}); err != nil && err != errCollision {
				return fmt.Errorf("insert error: %w", err)
			}
		}
		next = response.Data.After
	}
	return nil
}

var serverError = errors.New("server error")

func downloadFile(url string, path string) error {
	if err := exec.Command("curl", url, "-o", path).Run(); err != nil {
		fmt.Println("url", url, "path", path)
		return err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := httpWithBackoff(req, 2)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		bs, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: %s", serverError, bs)
	}
	defer resp.Body.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	if resp.StatusCode != http.StatusOK {
		bs, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w %s: %s", serverError, resp.Status, bs)
	}

	if n, err := io.Copy(out, resp.Body); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("error copying file")
	}

	return nil
}

func (c *app) getPostsToArchive() ([]*post, error) {
	rows, err := c.db.Query("select name from posts where status = '';")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		names = append(names, name)
	}

	var posts []*post
	for _, name := range names {
		p, err := c.getPost(name)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, nil
}

var errUnsupportedSource = errors.New("unsupported source")
var errDeleted = errors.New("deleted")

func (c *app) archivePost(p *post) error {
	if strings.HasPrefix(p.url, "https://v.redd.it/") {
		return c.archiveOneRedditVideo(p)
	} else if strings.HasPrefix(p.url, "https://i.redd.it/") {
		return c.archiveOneImage(p)
	} else if strings.Contains(p.url, "redgifs.com/watch/") {
		return c.archiveOneRedgif(p)
	} else if strings.Contains(p.url, "gfycat.com/") {
		return c.archiveOneRedgif(p)
	} else if strings.Contains(p.url, "imgur.com") {
		return c.archiveOneImgur(p)
	} else {
		u, _ := url.Parse(p.url)
		return fmt.Errorf("%w %s", errUnsupportedSource, u.Host)
	}
}

func (c *app) archiveOneRedditVideo(p *post) error {
	fmt.Println("archiving " + p.title)

	if err := c.loadPostJson(p); err != nil {
		return err
	}

	var data struct {
		Secure_Media struct {
			Reddit_Video struct {
				Fallback_URL string
			}
		}
	}
	json.Unmarshal(*p.json, &data)
	u := data.Secure_Media.Reddit_Video.Fallback_URL
	if u == "" {
		p.status = "failed reddit without fallback"
		if err := c.updatePost(p); err != nil {
			return err
		}
		fmt.Println("reddit no fallback")
		return nil
	}

	f := path.Join(archivePath, p.name+".mp4")
	if err := downloadFile(u, f); err != nil {
		return err
	}

	p.archivepath = ptrS(f)
	p.status = "archived"
	p.filetype = ptrS(".mp4")
	if err := c.updatePost(p); err != nil {
		return err
	}

	return nil
}

func (c *app) archiveOneImgur(p *post) error {
	fmt.Println("archiving " + p.title)

	if err := c.loadPostJson(p); err != nil {
		return err
	}

	var data struct {
		Preview struct {
			Reddit_Video_Preview struct {
				Fallback_URL string
			}
		}
	}
	json.Unmarshal(*p.json, &data)
	u := data.Preview.Reddit_Video_Preview.Fallback_URL
	filetype := ".mp4"
	if u == "" {
		u = strings.Replace(p.url, ".gifv", ".mp4", 1)
	}

	f := path.Join(archivePath, p.name+filetype)
	if err := downloadFile(u, f); err != nil {
		return err
	}

	p.archivepath = ptrS(f)
	p.status = "archived"
	p.filetype = ptrS(".mp4")
	if err := c.updatePost(p); err != nil {
		return err
	}

	return nil
}

func (c *app) archiveOneRedgif(p *post) error {
	fmt.Println("archiving redgif", p.title)

	parts := strings.Split(p.url, "/")
	lastPart := parts[len(parts)-1]
	idParts := strings.Split(lastPart, "-")
	id := idParts[0]
	res, err := c.request("GET", "https://api.redgifs.com/v2/gifs/"+id, "", func(req *http.Request) {
		req.Header.Add("Authorization", "Bearer "+redgifToken)
	})
	if err != nil {
		if strings.Contains(err.Error(), "[410 Gone]") || strings.Contains(err.Error(), "[404 Not Found]") {
			p.status = "deleted"
			if err := c.updatePost(p); err != nil {
				return err
			}
			return nil
		}
		if strings.Contains(err.Error(), "[401 Unauthorized]") {
			fmt.Println("401")
			return nil
		}
		return err
	}
	var responseData struct {
		Gif struct {
			URLs struct {
				HD string
			}
		}
	}
	json.Unmarshal(res, &responseData)
	if responseData.Gif.URLs.HD == "" {
		return fmt.Errorf("no video url")
	}

	f := path.Join(archivePath, p.name+".mp4")
	if err := downloadFile(responseData.Gif.URLs.HD, f); err != nil {
		return err
	}

	p.archivepath = ptrS(f)
	p.status = "archived"
	p.filetype = ptrS(".mp4")
	if err := c.updatePost(p); err != nil {
		return err
	}

	return nil
}

func (c *app) archiveOneImage(p *post) error {
	fmt.Println("archiving " + p.title)

	filetype := path.Ext(p.url)

	url := p.url
	if strings.HasPrefix(p.url, ".gif") {
		if err := c.loadPostJson(p); err != nil {
			return err
		}
		var d struct {
			Preview struct {
				Images []struct {
					Variants struct {
						MP4 struct {
							Source struct {
								URL string
							}
						}
					}
				}
			}
		}
		if err := json.Unmarshal(*p.json, &d); err != nil {
			return err
		}
		u := d.Preview.Images[0].Variants.MP4.Source.URL
		if u != "" {
			url = u
			filetype = ".mp4"
		}
	}

	jpgPath := path.Join(archivePath, p.name+filetype)
	if err := downloadFile(url, jpgPath); err != nil {
		if errors.Is(err, serverError) {
			p.status = err.Error()
			if err := c.updatePost(p); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	p.status = "archived"
	p.archivepath = &jpgPath
	p.filetype = ptrS(filetype)
	if err := c.updatePost(p); err != nil {
		return err
	}

	return nil
}

func main() {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Println("opening db failed:", err)
		panic(err)
	}
	defer db.Close()

	c := &app{db: db}
	if err := c.migrate(); err != nil {
		fmt.Println("migrate failed:", err)
		os.Exit(1)
	}

	http.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		n := r.URL.Query().Get("n")
		offset, _ := strconv.ParseInt(n, 10, 64)
		c.servePage(1, int(offset), w, r)
	})

	fs := http.FileServer(http.Dir("/mypool/tank/mirror/reddit/"))
	http.Handle("/media/", http.StripPrefix("/media/", fs))
	http.ListenAndServe(":3334", nil)

	return

	if err := c.refreshSaved(); err != nil {
		fmt.Println("refresh failed:", err)
		os.Exit(1)
	}

	posts, err := c.getPostsToArchive()
	if err != nil {
		fmt.Println("error getting posts to archive", err)
		os.Exit(1)
	}
	for _, p := range posts {
		if err := c.archivePost(p); errors.Is(err, errUnsupportedSource) {
			fmt.Println(err)
		} else if errors.Is(err, errDeleted) {
			fmt.Println(err)
		} else if err != nil {
			fmt.Println("error archiving post", err)
			os.Exit(0)
		}
	}
}

func ptrS(s string) *string {
	return &s
}

func httpWithBackoff(req *http.Request, max int) (*http.Response, error) {
	c := http.Client{}
	backoffCoef := 1
	for {
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 429 {
			fmt.Printf("backoff %d\n", backoffCoef)
			time.Sleep(time.Duration(backoffCoef) * time.Second)
			backoffCoef *= 2
			if backoffCoef > max {
				return resp, err
			}
			continue
		}
		return resp, nil
	}
}
