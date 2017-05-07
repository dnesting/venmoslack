// Copyright 2017 David Nesting. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package venmoslack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

// timestamp is a time.Time that can UnmarshalJSON from a format
// of 2006-01-02T15:04:05.999999999.
type timestamp time.Time

func (ts *timestamp) UnmarshalJSON(b []byte) error {
	// Almost, but not quite time.RFC3339Nano (which expects a timezone)
	t, err := time.Parse("\"2006-01-02T15:04:05.999999999\"", string(b))
	*ts = timestamp(t)
	if string(b) == "null" {
		return nil
	}
	return err
}

// https://developer.venmo.com/docs/webhooks
type venmoUser struct {
	DisplayName       string `json:"display_name"`
	ProfilePictureURL string `json:"profile_picture_url"`
	Username          string `json:"username"`
}
type venmoWebhook struct {
	DateCreated timestamp `json:"date_created"`
	Type        string    `json:"type"` // payment.created or payment.updated
	Data        struct {
		Action        string    `json:"action"` // pay
		Actor         venmoUser `json:"actor"`
		Amount        float32   `json:"amount"`
		DateCreated   timestamp `json:"date_created"`
		DateCompleted timestamp `json:"date_completed"`
		Note          string    `json:"note"`
		Status        string    `json:"status"` // settled, cancelled, expired, failed, pending
		Target        struct {
			Email string `json:"email"`
			Type  string `json:"type"` // user
			User  venmoUser
		} `json:"target"`
	} `json:"data"`
}

func init() {
	http.HandleFunc("/venmo-hook/", hook)
}

func hook(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	conf, err := getConfig(ctx)
	if err != nil {
		log.Errorf(ctx, "config error: %v", err)
		http.Error(w, "Unconfigured", http.StatusInternalServerError)
		return
	}

	// Verify that the last path component matches our stored key
	if conf.AccessKey == "" {
		log.Errorf(ctx, "%s", "no access key defined; visit app URL to configure.")
		http.Error(w, "Unconfigured", http.StatusInternalServerError)
		return
	} else {
		_, key := path.Split(r.URL.Path)
		if conf.AccessKey != key {
			log.Infof(ctx, "key mismatch; ensure Venmo is using the right URL")
			http.Error(w, "Key mismatch", http.StatusForbidden)
			return
		}
	}

	// Verify that we have a Slack hook configured before proceeding.
	if conf.SlackHook == "" {
		log.Errorf(ctx, "%s", "Hook attempted without configuration; visit app URL to configure.")
		http.Error(w, "Unconfigured", http.StatusInternalServerError)
		return
	}

	// Respond to requests with venmo_challenge for initial callback setup. In reality, it doesn't
	// look like this is done, but the docs say we have to do it.
	if err := r.ParseForm(); err != nil {
		log.Errorf(ctx, "form: %v", err)
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}
	if c := r.Form.Get("venmo_challenge"); c != "" {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, c)
		return
	}

	// Decode the Venmo Webhook payload
	var data venmoWebhook
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		log.Errorf(ctx, "json: %v", err)
		http.Error(w, "Error decoding JSON body", http.StatusBadRequest)
		return
	}

	log.Debugf(ctx, "%+v", data)

	// Render the Slack message
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "slack-message.tmpl", data); err != nil {
		log.Errorf(ctx, "template: %v", err)
		http.Error(w, "Error rendering message", http.StatusInternalServerError)
		return
	}

	// Deliver it to Slack
	if err := sendToSlack(ctx, conf.SlackHook, buf.String()); err != nil {
		log.Errorf(ctx, "slack: %v", err)
		http.Error(w, "Error sending message", http.StatusInternalServerError)
		return
	}

	history.Value = buf.String()
	history = history.Next()
}

// sendToSlack delivers msg to the pre-configured Slack webhook URL.
func sendToSlack(ctx context.Context, url string, msg string) error {
	m := struct {
		Text string `json:"text"`
	}{
		Text: msg,
	}

	data, _ := json.Marshal(m)
	client := urlfetch.Client(ctx)
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		log.Errorf(ctx, "Posting to slack: %+v\n%v", resp, data)
		return fmt.Errorf("Unexpected status: %v", resp)
	}
	return nil
}
