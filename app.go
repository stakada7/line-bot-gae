package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"

	"golang.org/x/net/context"

	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/linebot"
)

func init() {
	err := godotenv.Load("line.env")
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/callback", handleCallback)
	http.HandleFunc("/task", handleTask)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	c := newContext(r)
	bot, err := newLINEBot(c)
	if err != nil {
		errorf(c, "newLINEBot: %v", err)
		return
	}

	evs, err := bot.ParseRequest(r)
	if err != nil {
		errorf(c, "bot.ParseRequest: %v", err)
		if err == linebot.ErrInvalidSignature {
			http.Error(w, err.Error(), 400)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}

	ts := make([]*taskqueue.Task, len(evs))
	for i, e := range evs {
		j, err := json.Marshal(e)
		if err != nil {
			errorf(c, "json.Marshal: %v", err)
			return
		}
		data := base64.StdEncoding.EncodeToString(j)
		t := taskqueue.NewPOSTTask("/task", url.Values{"data": {data}})
		ts[i] = t
	}
	taskqueue.AddMulti(c, ts, "")
	w.WriteHeader(204)
}

func handleTask(w http.ResponseWriter, r *http.Request) {
	c := newContext(r)
	data := r.FormValue("data")
	if data == "" {
		errorf(c, "No data")
		return
	}

	j, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		errorf(c, "base64 DecodeString: %v", err)
		return
	}

	e := new(linebot.Event)
	err = json.Unmarshal(j, e)
	if err != nil {
		errorf(c, "json.Unmarshal: %v", err)
		return
	}

	bot, err := newLINEBot(c)
	if err != nil {
		errorf(c, "newLINEBot: %v", err)
		return
	}

	logf(c, "EventType: %s", e.Type)

	src := e.Source
	if src.Type == linebot.EventSourceTypeUser {
		m := linebot.NewTextMessage("drink!!!")
		if _, err = bot.ReplyMessage(e.ReplyToken, m).WithContext(c).Do(); err != nil {
			errorf(c, "ReplayMessage: %v", err)
			return
		}
	}

	w.WriteHeader(200)
}

func logf(c context.Context, format string, args ...interface{}) {
	log.Infof(c, format, args...)
}

func errorf(c context.Context, format string, args ...interface{}) {
	log.Errorf(c, format, args...)
}

func newContext(r *http.Request) context.Context {
	return appengine.NewContext(r)
}

func newLINEBot(c context.Context) (*linebot.Client, error) {
	return linebot.New(
		os.Getenv("LINE_BOT_CHANNEL_SECRET"),
		os.Getenv("LINE_BOT_CHANNEL_TOKEN"),
		linebot.WithHTTPClient(urlfetch.Client(c)))
}

func isDevServer() bool {
	return os.Getenv("RUN_WITH_DEVAPPSERVER") != ""
}
