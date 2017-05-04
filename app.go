// Copyright 2017 David Nesting. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package venmoslack

import (
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"

	"golang.org/x/net/context"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"
)

type Config struct {
	SlackHook string
	AccessKey string
}

func configKey(ctx context.Context) *datastore.Key {
	return datastore.NewKey(ctx, "Config", "config", 0, nil)
}

func getConfig(ctx context.Context) (c Config, err error) {
	err = datastore.Get(ctx, configKey(ctx), &c)
	return
}

func writeConfig(ctx context.Context, c Config) (err error) {
	_, err = datastore.Put(ctx, configKey(ctx), &c)
	return
}

var tpl = template.Must(template.ParseGlob("templates/*.tmpl"))

func isAuthorized(ctx context.Context) bool {
	u := user.Current(ctx)
	return u != nil && u.Email == os.Getenv("ADMIN")
}

const keyBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateAccessKey() string {
	b := make([]byte, 20)
	for i := range b {
		b[i] = keyBytes[rand.Int63()%int64(len(keyBytes))]
	}
	return string(b)
}

func init() {
	http.HandleFunc("/", handleIndex)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	conf, err := getConfig(ctx)
	if err != nil {
		log.Errorf(ctx, "%v", err)
	}

	var email, logout, login string
	if u := user.Current(ctx); u != nil {
		logout, _ = user.LogoutURL(ctx, "/")
		email = u.Email
	} else {
		login, _ = user.LoginURL(ctx, "/")
	}

	if conf.AccessKey == "" {
		conf.AccessKey = generateAccessKey()
	}

	var message string
	if r.Method == "POST" {
		if !isAuthorized(ctx) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		r.ParseForm()

		if r.Form["action"][0] == "Save" {
			conf.SlackHook = r.Form["slackHook"][0]
		} else if r.Form["action"][0] == "Regenerate" {
			conf.AccessKey = generateAccessKey()
		}
		err := writeConfig(ctx, conf)
		if err != nil {
			log.Errorf(ctx, "%v", err)
			message = fmt.Sprintf("Failed to write config: %v", err)
		}
	}

	data := struct {
		Login, Logout, Email string
		Config               Config
		IsAdmin              bool
		Error                string
	}{
		Login:   login,
		Logout:  logout,
		Email:   email,
		Config:  conf,
		IsAdmin: isAuthorized(ctx),
		Error:   message,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "index.tmpl", data); err != nil {
		log.Errorf(ctx, "%v", err)
	}
}
