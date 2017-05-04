// Copyright 2017 David Nesting. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package venmoslack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

func init() {
	http.HandleFunc("/venmo-hook", hook)
}

type timestamp time.Time

func (ts *timestamp) UnmarshalJSON(b []byte) error {
	t, err := time.Parse("\"2006-01-02T15:04:05.999999999\"", string(b))
	*ts = timestamp(t)
	if string(b) == "null" {
		return nil
	}
	return err
}

// https://developer.venmo.com/docs/webhooks
type VenmoUser struct {
	DisplayName       string `json:"display_name"`
	ProfilePictureURL string `json:"profile_picture_url"`
	Username          string `json:"username"`
}
type VenmoWebhook struct {
	DateCreated timestamp `json:"date_created"`
	Type        string    `json:"type"` // payment.created or payment.updated
	Data        struct {
		Action        string    `json:"action"` // pay
		Actor         VenmoUser `json:"actor"`
		Amount        float32   `json:"amount"`
		DateCreated   timestamp `json:"date_created"`
		DateCompleted timestamp `json:"date_completed"`
		Note          string    `json:"note"`
		Status        string    `json:"status"` // settled, cancelled, expired, failed, pending
		Target        struct {
			Email string `json:"email"`
			Type  string `json:"type"` // user
			User  VenmoUser
		} `json:"target"`
	} `json:"data"`
}

func hook(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	conf, err := getConfig(ctx)
	if err != nil {
		log.Errorf(ctx, "%v", err)
	}
	if conf.SlackHook == "" {
		http.Error(w, "Unconfigured. Visit /config", http.StatusInternalServerError)
		log.Errorf(ctx, "%s", "Hook attempted without configuration")
	}

	// Respond to requests with venmo_challenge for initial callback setup
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusInternalServerError)
		log.Errorf(ctx, "form: %v", err)
	}
	if c := r.Form.Get("venmo_challenge"); c != "" {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, c)
		return
	}

	var data VenmoWebhook
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		log.Errorf(ctx, "json: %v", err)
		//http.Error(w, "Error decoding", http.StatusInternalServerError)
		//return
	}

	log.Errorf(ctx, "OK! %+v", data)

	tmpl := template.Must(template.ParseFiles("templates/slack-message.tmpl"))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Errorf(ctx, "template: %v", err)
		http.Error(w, "Error rendering message", http.StatusInternalServerError)
		return
	}

	if err := sendToSlack(ctx, buf.String()); err != nil {
		log.Errorf(ctx, "slack: %v", err)
		http.Error(w, "Error sending message", http.StatusInternalServerError)
		return
	}
}

func sendToSlack(ctx context.Context, msg string) error {
	config, err := getConfig(ctx)
	if err != nil {
		return err
	}

	var m struct {
		Text    string `json:"text"`
		Channel string `json:"channel",omitempty`
		//LinkNames int    `json:"link_names",omitempty`
		//Username  string `json:"username",omitempty`
		//IconEmoji string `json:"icon_emoji",omitempty`
	}

	m.Text = msg

	data, _ := json.Marshal(m)
	client := urlfetch.Client(ctx)
	resp, err := client.Post(config.SlackHook, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		log.Errorf(ctx, "Posting to slack: %+v\n%v", resp, data)
		return fmt.Errorf("Unexpected status: %v", resp)
	}
	return nil
}
