// Copyright 2017 David Nesting. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package venmoslack

import (
	"container/ring"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"time"

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

func isAuthorized(ctx context.Context) bool {
	u := user.Current(ctx)
	return u != nil && u.Email == os.Getenv("ADMIN")
}

const keyBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const keyLength = 30

func generateAccessKey() string {
	b := make([]byte, keyLength)
	for i := range b {
		b[i] = keyBytes[rand.Int63()%int64(len(keyBytes))]
	}
	return string(b)
}

var tpl = template.Must(template.ParseGlob("templates/*.tmpl"))

func init() {
	rand.Seed(time.Now().UnixNano())
	http.HandleFunc("/", handleIndex)
}

const historySize = 3

var history = ring.New(historySize)

func handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	conf, err := getConfig(ctx)

	// If no initial config, create one.
	if err != nil || conf.AccessKey == "" {
		conf.AccessKey = generateAccessKey()
		err = writeConfig(ctx, conf)
		if err != nil {
			log.Errorf(ctx, "%v", err)
			http.Error(w, "Config error", http.StatusInternalServerError)
			return
		}
	}

	var email, logout, login string
	if u := user.Current(ctx); u != nil {
		logout, _ = user.LogoutURL(ctx, "/")
		email = u.Email
	} else {
		login, _ = user.LoginURL(ctx, "/")
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
			message = "Saved Slack Incoming Webhook URL"
		} else if r.Form["action"][0] == "Regenerate" {
			conf.AccessKey = generateAccessKey()
			message = "Regenerated Venmo Webhook URL"
		}

		err := writeConfig(ctx, conf)
		if err != nil {
			log.Errorf(ctx, "%v", err)
			message = fmt.Sprintf("Failed to write config: %v", err)
		}
	}

	histSlice := make([]string, historySize)
	i := 0
	history.Do(func(v interface{}) {
		if v != nil {
			s := v.(string)
			if s != "" {
				histSlice[i] = v.(string)
				i++
			}
		}
	})
	histSlice = histSlice[:i]

	data := struct {
		Login, Logout, Email string
		Config               Config
		IsAdmin              bool
		Message              string
		Version              string
		History              []string
	}{
		Login:   login,
		Logout:  logout,
		Email:   email,
		Config:  conf,
		IsAdmin: isAuthorized(ctx),
		Message: message,
		Version: Release,
		History: histSlice,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "index.tmpl", data); err != nil {
		log.Errorf(ctx, "%v", err)
	}
}
