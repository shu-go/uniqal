package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/shu-go/gli"
	"github.com/shu-go/minredir"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/xerrors"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Version is app version
var Version string

func init() {
	if Version == "" {
		Version = "dev-" + time.Now().Format("20060102")
	}
}

type globalCmd struct {
	Start      gli.Date    `cli:"start,s=DATE"  help:"defaults to today"`
	Items      int64       `cli:"items,n=NUMBER"  default:"10"  help:"the number of events from --start"`
	Keys       gli.StrList `cli:"keys,k=LIST_OF_STRINGS"  default:"Description,Summary,Start,End"  help:"comman-separated keys to test uniquity of events"`
	CalendarID string      `cli:"calendar-id,id"  default:"primary"`

	Credential string `cli:"credentials,c=FILE_NAME"  default:"./credentials.json"  help:"your client configuration file from Google Developer Console"`
	Token      string `cli:"token,t=FILE_NAME"  default:"./token.json"  help:"file path to read/write retrieved token"`

	AuthPort uint16 `cli:"auth-port=NUMBER"  default:"7878"`

	DryRun bool `cli:"dry-run,dry"  help:"do not exec"`
}

var (
	ClientID, ClientSecret string
)

func UniqKey(e *calendar.Event, fields ...string) string {
	k := ""

	for _, f := range fields {
		switch strings.ToLower(f) {
		case "created":
			k += e.Created
		case "description":
			k += e.Description
		case "end":
			t := e.End.DateTime
			if t != "" {
				k += t
			} else {
				k += e.End.Date
			}
		case "etag":
			k += e.Etag
		case "hangoutLink":
			k += e.HangoutLink
		case "htmllink":
			k += e.HtmlLink
		case "icaluid":
			k += e.ICalUID
		case "id":
			k += e.Id
		case "location":
			k += e.Location
		case "start":
			t := e.Start.DateTime
			if t != "" {
				k += t
			} else {
				k += e.Start.Date
			}
		case "summary":
			k += e.Summary
		case "updated":
			k += e.Updated
		default:
		}
	}

	return k
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config, tokFile string, port uint16) (*http.Client, error) {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config, port)
		if err != nil {
			return nil, err
		}

		err := saveToken(tokFile, tok)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config, port uint16) (*oauth2.Token, error) {
	// setup parameters

	codeChan := make(chan string)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	err, errChan := minredir.ServeTLS(ctx, fmt.Sprintf(":%v", port), codeChan)
	if err != nil {
		return nil, xerrors.Errorf("failed to start local TLS server: %v", err)
	}

	// request authorization (and authentication)

	config.RedirectURL = fmt.Sprintf("https://localhost:%d/", port)
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	err = browser.OpenURL(authURL)
	if err != nil {
		return nil, err
	}

	var authCode string
	select {
	case authCode = <-codeChan:
		// nop
	case err := <-errChan:
		return nil, xerrors.Errorf("failed to retrieve token from web: %v", err)
	}
	cancel()

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, xerrors.Errorf("failed to retrieve token from web: %v", err)
	}
	return tok, nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return xerrors.Errorf("failed to cache oauth token: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func main() {
	app := gli.NewWith(&globalCmd{})
	app.Name = "uniqal"
	app.Version = Version
	app.Desc = "make each event be unique"
	app.Usage = `uniqal --credential=./my_credentials.json --items=100 --start=` + time.Now().AddDate(0, 0, 7).Format("2006-01-02") + `

------------
how to start
------------

1. go to https://console.cloud.google.com
2. make a new project
3. enable Google Calendar API from Library
4. download credential json
5. rename it as credentials.json and place it in current working dir

------
--keys
------

created
description
end
etag
hangoutLink
htmllink
icaluid
id
location
start
summary
updated

--keys=summary,start,end  may match for your needs.
And then, --dry is useful for testing.
`
	app.Copyright = "(C) 2019 Shuhei Kubota"

	err := app.Run(os.Args)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func (c globalCmd) Run() error {
	uniqs := make(map[string]struct{})

	var config *oauth2.Config
	var err error
	if _, err := os.Stat(c.Credential); err != nil {
		if ClientID == "" || ClientSecret == "" {
			return xerrors.New("ClientID or ClientSecret is empty")
		}

		c.Items = 10
		c.Start = gli.Date(time.Now())

		config = &oauth2.Config{
			ClientID:     ClientID,
			ClientSecret: ClientSecret,
			Scopes:       []string{calendar.CalendarScope},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://accounts.google.com/o/oauth2/token",
			},
		}
	} else {
		b, err := os.ReadFile(c.Credential)
		if err != nil {
			return xerrors.Errorf("failed to read the credential file: %v", err)
		}

		// If modifying these scopes, delete your previously saved token.json.
		config, err = google.ConfigFromJSON(b, calendar.CalendarScope /*CalendarReadonlyScope*/)
		if err != nil {
			return xerrors.Errorf("failed to parse the credentials: %v", err)
		}
	}
	client, err := getClient(config, c.Token, c.AuthPort)
	if err != nil {
		return xerrors.Errorf("failed to connect services: %v", err)
	}

	ctx := context.Background()
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return xerrors.Errorf("failed to retrieve Calendar client: %v", err)
	}

	t := c.Start.Time().Format(time.RFC3339)
	events, err := srv.Events.List(c.CalendarID).ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(c.Items).OrderBy("startTime").Do()
	if err != nil {
		return xerrors.Errorf("failed to retrieve events: %v", err)
	}

	if len(events.Items) == 0 {
		fmt.Println("no events")
		return nil
	}

	for _, item := range events.Items {
		date := item.Start.DateTime
		if date == "" {
			date = item.Start.Date
		}

		key := UniqKey(item, c.Keys...)
		if _, found := uniqs[key]; found {
			fmt.Printf("[DEL] %v (%v)\n", item.Summary, date)
			if !c.DryRun {
				delevent := srv.Events.Delete(c.CalendarID, item.Id)
				err = delevent.Do()
				if err != nil {
					fmt.Printf("  failed to delete: %v", err)
				}
			}
		} else {
			uniqs[key] = struct{}{}

			fmt.Printf("* %v (%v)\n", item.Summary, date)
		}
	}

	return nil
}

func (c *globalCmd) Init() {
	c.Start = gli.Date(time.Now())
}
