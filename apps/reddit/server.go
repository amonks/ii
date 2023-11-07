package main

import (
	"html/template"
	"net/http"
)

func (c *app) getPosts(limit, offset int) ([]*post, error) {
	var ids []string
	rows, err := c.db.Query("select name from posts where filetype is not null order by name desc limit ? offset ?", limit, offset)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	posts := make([]*post, len(ids))
	for i := 0; i < len(ids); i++ {
		post, err := c.getPost(ids[i])
		if err != nil {
			return nil, err
		}
		posts[i] = post
	}
	return posts, nil
}

func (c *app) servePage(limit, offset int, res http.ResponseWriter, req *http.Request) {
	tmpl := template.Must(template.ParseFiles("index.gohtml"))
	posts, err := c.getPosts(limit, offset)
	if err != nil {
		panic(err)
	}
	tmpl.Execute(res, struct {
		Posts []*post
		Next  int
	}{posts, offset + 1})
}
