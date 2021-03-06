# Venmo → Slack Integration

This implements a Venmo web hook that sends notifications of Venmo
transactions to Slack.

## Quick Start

1. Set up the [Appengine SDK](https://cloud.google.com/appengine/docs/standard/go/download).

2. Modify `app.yaml` and change the `ADMIN` environment variable
   to point to a Google account you plan to use to configure the
   instance.

3. Start a copy locally to verify it runs for you:

   ```console
   $ dev_appserver.py app.yaml
   ```

4. Navigate to [http://localhost:8080/](http://localhost:8080/) to
   access the locally-running instance.  If you like, follow the
   configuration steps to get a sense for how you'll configure it
   for real.

5. If needed, create the AppEngine app:

   ```console
   $ gcloud app create
   ```

6. Deploy to AppEngine

   ```console
   $ gcloud app deploy
   ```

7. Visit the AppEngine URL for your new app and configure it.

## Configuration

Within `app.yaml` you will want to set the `ADMIN` environment
variable to point to a Google account that you plan to use to
administer the instance.  Once it's running, visiting the app's URL
will guide you through the rest of the configuration.

### Setting up a Slack Incoming Webhook

This is how messages get delivered to Slack.  [Read more about
Incoming Webhooks](https://api.slack.com/incoming-webhooks).  [Set
up an Incoming Webhook for your
team](https://my.slack.com/services/new/incoming-webhook/).

If desired, change the username and user icon in the Slack UI for
the Incoming Webhook.

### Setting up a Venmo Webhook

The UI will give you the Webhook URL that you'll give to Venmo.
[Sign in to your Venmo account](https://venmo.com/) and visit the
[Developer Settings](https://venmo.com/account/settings/developer)
page.  Paste the Venmo Webhook URL into this field and Save.

This URL has a random component to it, to prevent people from
discovering and abusing it.  You can regenerate this URL at any
time in this config UI, but will need to update Venmo to use the
new URL when you do so.

### Fin

At this point, you should be completely configured and notifications
should be arriving in Slack.  Send a test payment request to see
them in action.
