package pkg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cretz/bine/tor"
	"github.com/eyedeekay/goSam"
	tdl "github.com/lmas/Damerau-Levenshtein"
	boom "github.com/tylertreat/BoomFilters"
	"golang.org/x/net/proxy"
)

const (
	maxItems    int = 50
	feedTimeout int = 2 // seconds
)

func date(t time.Time) string {
	return t.Format("2006-01-02")
}

func loadFilter(path string) (*boom.ScalableBloomFilter, error) {
	filter := boom.NewDefaultScalableBloomFilter(0.01)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return filter, nil
		}
		return nil, err
	}

	defer f.Close()
	if _, err := filter.ReadFrom(f); err != nil {
		return nil, err
	}
	return filter, nil
}

////////////////////////////////////////////////////////////////////////////////

func seenTitle(title string, list []string) bool {
	for _, s := range list {
		score := tdl.Distance(title, s)
		if score < 2 {
			return true
		}
	}
	return false
}

func (app *App) seenURL(url string) bool {
	return app.filter.TestAndAdd([]byte(url))
}

func (app *App) newItems(url string) ([]Item, error) {
	feed, err := app.feedParser.ParseURL(url)
	if err != nil {
		return nil, err
	}

	var titles []string
	var items []Item
	max := len(feed.Items)
	if max > maxItems {
		max = maxItems
	}

	for _, i := range feed.Items[:max] {
		if seenTitle(i.Title, titles) {
			continue
		}
		titles = append(titles, i.Title)

		if app.seenURL(i.Link) {
			continue
		}

		items = append(items, Item{
			Title: strings.TrimSpace(i.Title),
			URL:   i.Link,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Title < items[j].Title
	})
	return items, nil
}

////////////////////////////////////////////////////////////////////////////////

func (app *App) proxySet(u string) (io.Closer, error) {
	fmt.Println("Setting up proxy for URL:", u)
	app.feedParser.Client.Transport = nil
	Url, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	if Url.Hostname() == "localhost" || Url.Hostname() == "127.0.0.1" {
		return nil, nil
	}
	fmt.Println("Got hostname:", Url.Hostname())
	var sam *goSam.Client
	if strings.HasSuffix(Url.Hostname(), ".i2p") {
		fmt.Println("I2P Hostname, using SAM")
		sam, err = goSam.NewClient("127.0.0.1:7656")
		if err != nil {
			return nil, err
		}
		tr := &http.Transport{
			Dial: sam.Dial,
		}
		app.feedParser.Client.Transport = tr
		return sam, nil
	}
	if strings.HasSuffix(Url.Hostname(), ".onion") || app.Config.ListenAddr == "onion" || app.Config.ListenAddr == "i2p" {
		if tmp, torerr := net.Listen("tcp", "127.0.0.1:9050"); torerr != nil {
			fmt.Println("Onion hostname, using Tor")
			t, err := tor.Start(context.Background(), nil)
			if err != nil {
				if t == nil {
					return nil, err
				}
			}
			dialCtx, _ := context.WithTimeout(context.Background(), time.Second*time.Duration(app.Config.Timeout))
			dialer, err := t.Dialer(dialCtx, nil)
			if err != nil {
				return nil, err
			}
			tr := &http.Transport{DialContext: dialer.DialContext}
			app.feedParser.Client.Transport = tr
			return t, nil
		} else {
			tmp.Close()
			return nil, fmt.Errorf("Onion URL requested but Tor does not appear to be running.")
		}
	}
	envProxy, envProxyType := getEnvProxy()
	fmt.Println("Got env proxy:", envProxy)
	switch envProxyType {
	case -1:
		return nil, nil
	case 0:
		Url, err := url.Parse(envProxy)
		if err != nil {
			return nil, err
		}
		app.feedParser.Client.Transport = &http.Transport{
			Proxy: http.ProxyURL(Url),
		}
	case 1:
		Url, err := url.Parse(envProxy)
		if err != nil {
			return nil, err
		}
		dialSocksProxy, err := proxy.SOCKS5("tcp", Url.String(), nil, proxy.Direct)
		if err != nil {
			fmt.Println("Error connecting to proxy:", err)
		}
		tr := &http.Transport{Dial: dialSocksProxy.Dial}
		app.feedParser.Client.Transport = tr
	default:
		return nil, fmt.Errorf("Unknown proxy type: %d", envProxyType)
	}
	return nil, nil
}

func getEnvProxy() (string, int) {
	proxy := os.Getenv("HTTP_PROXY")
	if proxy == "" {
		proxy = os.Getenv("http_proxy")
	}
	if proxy != "" {
		return proxy, 0
	}
	proxy = os.Getenv("ALL_PROXY")
	if proxy == "" {
		proxy = os.Getenv("all_proxy")
	}
	if proxy != "" {
		return proxy, 1
	}
	return "", -1
}

func (app *App) updateAllFeeds(feeds map[string]string) []Feed {
	var updated []Feed
	sleep := time.Duration(feedTimeout) * time.Second
	for title, url := range feeds {
		closer, err := app.proxySet(url)
		if err != nil {
			if closer != nil {
				closer.Close()
			}
			fmt.Println("Error setting up proxy:", err)
			continue
		}
		app.Log("Updating %s (%s)", title, url)
		items, err := app.newItems(url)
		if err != nil {
			app.Log("%s", err)
		}

		if len(items) > 0 || err != nil {
			updated = append(updated, Feed{
				Title: title,
				URL:   url,
				Items: items,
				Error: err,
			})
		}
		time.Sleep(sleep)
		if closer != nil {
			closer.Close()
		}
	}
	sort.Slice(updated, func(i, j int) bool {
		return updated[i].Title < updated[j].Title
	})
	return updated
}

func (app *App) generatePage(feeds []Feed) (*bytes.Buffer, error) {
	app.Log("Generating page...")
	var buf bytes.Buffer
	err := app.tmpl.Execute(&buf, map[string]interface{}{
		"CurrentDate": date(app.time),
		"PrevDate":    date(app.time.AddDate(0, 0, -1)),
		"NextDate":    date(app.time.AddDate(0, 0, 1)),
		"Feeds":       feeds,
	})
	if err != nil {
		return nil, err
	}
	return &buf, nil
}

func (app *App) writePage(index, path string, buf *bytes.Buffer) error {
	app.Log("Writing page to %s...", path)
	err := os.MkdirAll(filepath.Dir(path), 0744)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := buf.WriteTo(f); err != nil {
		return err
	}

	err = os.Remove(index)
	if err != nil {
		// ignore error if the symlink doesn't exist already
		if !os.IsNotExist(err) {
			return err
		}
	}

	return os.Symlink(filepath.Base(path), index)
}

func (app *App) writeFilter(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = app.filter.WriteTo(f)
	return err
}

func (app *App) writeStyleFile(path string) error {
	// With these flags we try to avoid overwriting an existing file
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}

	defer f.Close()
	app.Log("Writing style file...")
	_, err = f.WriteString(tmplCSS)
	return err
}
