package pkg

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"time"

	sam "github.com/eyedeekay/sam3/helper"
	"github.com/mmcdole/gofeed"
	boom "github.com/tylertreat/BoomFilters"
)

type Item struct {
	Title string
	URL   string
}

type Feed struct {
	Title string
	URL   string
	Items []Item
	Error error
}

type App struct {
	Config *Config

	time       time.Time
	tmpl       *template.Template
	filter     *boom.ScalableBloomFilter
	feedParser *gofeed.Parser
}

type UserAgentTransport struct {
	http.RoundTripper
}

func (c *UserAgentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Note: having some issues with reddit not liking bots, so have to
	// really show them some love with this verbose user agent.
	// No other way to set it currently, with this rss lib.
	// See https://github.com/mmcdole/gofeed/issues/74
	r.Header.Set("User-Agent", "linux:feedloggr:3.0 (by /u/go255)")
	return c.RoundTripper.RoundTrip(r)
}

func New(config *Config) (*App, error) {
	tmpl, err := template.New("page").Parse(tmplPage)
	if err != nil {
		return nil, err
	}

	feedParser := gofeed.NewParser()
	feedParser.Client = &http.Client{
		Timeout:   time.Duration(config.Timeout) * time.Second,
		Transport: &UserAgentTransport{http.DefaultTransport},
	}

	path := filepath.Join(config.OutputPath, ".filter.dat")
	filter, err := loadFilter(path)
	if err != nil {
		return nil, err
	}

	app := &App{
		Config:     config,
		time:       time.Now(),
		tmpl:       tmpl,
		filter:     filter,
		feedParser: feedParser,
	}
	return app, nil
}

func (app *App) Log(msg string, args ...interface{}) {
	if app.Config.Verbose {
		log.Printf(msg+"\n", args...)
	}
}

func (app *App) Update() error {
	feeds := app.updateAllFeeds(app.Config.Feeds)
	buf, err := app.generatePage(feeds)
	if err != nil {
		return err
	}

	index := filepath.Join(app.Config.OutputPath, "index.html")
	path := filepath.Join(app.Config.OutputPath, date(app.time)+".html")
	if err := app.writePage(index, path, buf); err != nil {
		return err
	}

	path = filepath.Join(app.Config.OutputPath, ".filter.dat")
	if err := app.writeFilter(path); err != nil {
		return err
	}

	path = filepath.Join(app.Config.OutputPath, "style.css")
	if err := app.writeStyleFile(path); err != nil {
		return err
	}
	return nil
}

func (app *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(app.Config.OutputPath, r.URL.Path)
	http.ServeFile(w, r, path)
}

func (app *App) Serve(l net.Listener) error {
	app.Log("Serving on %s", l.Addr())
	return http.Serve(l, app)
}

func (app *App) ListenAndServe() error {
	switch app.Config.ListenAddr {
	case "onion":
		return fmt.Errorf("Onion not implemented yet, check back later.")
	case "i2p":
		l, err := sam.I2PListener("feedloggr", "127.0.0.1:7656", "feedloggr")
		if err != nil {
			return err
		}
		return app.Serve(l)
	default:
		l, err := net.Listen("tcp", app.Config.ListenAddr)
		if err != nil {
			return err
		}
		return app.Serve(l)
	}
	return fmt.Errorf("unsupported listen address", app.Config.ListenAddr)
}
