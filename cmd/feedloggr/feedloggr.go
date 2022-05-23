package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/lmas/feedloggr/pkg"
)

var (
	verbose = flag.Bool("verbose", false, "run in verbose mode")
	config  = flag.String("config", ".feedloggr3.conf", "path to config file")

	version = flag.Bool("version", false, "print version and exit")
	example = flag.Bool("example", false, "print example config and exit")
	test    = flag.Bool("test", false, "test config file and exit")
)

var app *pkg.App
var cfg *pkg.Config

func main() {
	flag.Parse()

	if *version {
		fmt.Println("Feedloggr 3.0")
		fmt.Println("Aggregate news from RSS/Atom feeds and output static HTML pages.")
		return
	}

	if *example {
		cfg := pkg.NewConfig()
		fmt.Println(cfg)
		return // simple exit(0)
	}

	if err := updateRoutine(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if cfg.ListenAddr != "" {
		go updateLoop()
		if err := app.ListenAndServe(); err != nil {
			exiting = true
			fmt.Println(err)
			os.Exit(1)
		}
	}
	exiting = true

}

var exiting = false

func updateLoop() {
	for {
		if exiting {
			break
		}
		// get a random number between 24 and 48
		// sleep for that number of hours
		// then repeat updateRoutine()
		hours := 24 + rand.Intn(24)
		time.Sleep(time.Duration(hours) * time.Hour)
		if err := updateRoutine(); err != nil {
			fmt.Println(err)
			break
		}
	}
}

func updateRoutine() (err error) {
	cfg, err = pkg.LoadConfig(*config)
	if err != nil {
		fmt.Println(err)
		return
	}

	if *test {
		fmt.Println(cfg)
		fmt.Println("No errors while loading config file.")
		return
	}

	// cmd flags override config file
	if *verbose {
		cfg.Verbose = true
	}

	app, err = pkg.New(cfg)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = app.Update()
	if err != nil {
		fmt.Println(err)
		return
	}
	return nil
}
