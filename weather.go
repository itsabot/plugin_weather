package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itsabot/abot/shared/datatypes"
	"github.com/itsabot/abot/shared/language"
	"github.com/itsabot/abot/shared/log"
	"github.com/itsabot/abot/shared/nlp"
	"github.com/itsabot/abot/shared/plugin"
	"github.com/jmoiron/sqlx"
)

type Weather string
type weatherJSON struct {
	Description []string
	Temp        float64
	Humidity    int
}

var p *plugin.Plugin
var db *sqlx.DB
var l *log.Logger

const pluginName = "weather"

func main() {
	var coreaddr string
	flag.StringVar(&coreaddr, "coreaddr", "",
		"Port used to communicate with Abot.")
	flag.Parse()
	l = log.New(pluginName)
	l.SetDebug(true)
	rand.Seed(time.Now().UnixNano())
	trigger := &nlp.StructuredInput{
		Commands: []string{"what", "show", "tell", "is"},
		Objects: []string{"weather", "temperature", "temp", "outside",
			"raining"},
	}
	var err error
	db, err = plugin.ConnectDB()
	if err != nil {
		l.Fatal(err)
	}
	p, err = plugin.New(pluginName, coreaddr, trigger)
	if err != nil {
		l.Fatal(err)
	}
	p.Vocab = dt.NewVocab(
		dt.VocabHandler{
			Fn: kwGetTemp,
			Trigger: &nlp.StructuredInput{
				Commands: []string{"what", "show", "tell"},
				Objects: []string{"weather", "temperature",
					"temp", "outside"},
			},
		},
		dt.VocabHandler{
			Fn: kwGetRaining,
			Trigger: &nlp.StructuredInput{
				Commands: []string{"tell", "is"},
				Objects:  []string{"rain"},
			},
		},
	)
	weather := new(Weather)
	if err = p.Register(weather); err != nil {
		l.Fatal(err)
	}
}

func (t *Weather) Run(in *dt.Msg, resp *string) error {
	return t.FollowUp(in, resp)
}

func (t *Weather) FollowUp(in *dt.Msg, resp *string) error {
	*resp = p.Vocab.HandleKeywords(in)
	if len(*resp) == 0 {
		sm := buildStateMachine()
		*resp = sm.Next(in)
	}
	return nil
}

func kwGetTemp(in *dt.Msg, _ int) (resp string) {
	cities, err := language.ExtractCities(db, in)
	if err != nil {
		l.Debug("getting temp")
		return e(err)
	}
	if len(cities) == 0 {
		return ""
	}
	return getWeather(&cities[0])
}

func kwGetRaining(in *dt.Msg, _ int) (resp string) {
	cities, err := language.ExtractCities(db, in)
	if err != nil {
		l.Debug("getting rain")
		return e(err)
	}
	if len(cities) == 0 {
		return ""
	}
	resp = getWeather(&cities[0])
	for _, w := range strings.Fields(resp) {
		if w == "rain" {
			return fmt.Sprintf("It's raining in %s right now.",
				cities[0].Name)
		}
	}
	return fmt.Sprintf("It's not raining in %s right now", cities[0].Name)
}

func getWeather(city *dt.City) string {
	l.Debug("getting weather for city", city.Name)
	req := weatherJSON{}
	n := url.QueryEscape(city.Name)
	resp, err := http.Get("https://www.itsabot.org/api/weather.json?city=" + n)
	if err != nil {
		return e(err)
	}
	l.Debug("decoding resp")
	if err = json.NewDecoder(resp.Body).Decode(&req); err != nil {
		return e(err)
	}
	l.Debug("closing resp.Body")
	if err = resp.Body.Close(); err != nil {
		return e(err)
	}
	l.Debug("got weather")
	var ret string
	if len(req.Description) == 0 {
		ret = fmt.Sprintf("It's %.f in %s right now.", req.Temp,
			city.Name)
	} else {
		if len(strings.Fields(req.Description[0])) > 1 {
			// 2 word description, e.g. "moderate rain"
			ret = fmt.Sprintf("It's %.0f with %s in %s.", req.Temp,
				req.Description[0], city.Name)
		} else {
			// 1 word description, e.g. "sunny"
			ret = fmt.Sprintf("It's %.0f and %s in %s.", req.Temp,
				req.Description[0], city.Name)
		}
	}
	return ret
}

func buildStateMachine() *dt.StateMachine {
	sm := dt.NewStateMachine(pluginName)
	sm.SetDBConn(db)
	sm.SetLogger(l)
	sm.SetOnReset(func(in *dt.Msg) {
		sm.SetMemory(in, "city", nil)
	})
	sm.SetStates([]dt.State{
		{
			OnEntry: func(in *dt.Msg) string {
				return "I'll find out for you. What city are you in right now?"
			},
			OnInput: func(in *dt.Msg) {
				cities, err := language.ExtractCities(db, in)
				if err != nil {
					l.Debug(err)
					return
				}
				if len(cities) == 0 {
					l.Debug("found 0 cities")
					return
				}
				sm.SetMemory(in, "city", cities[0])
			},
			Complete: func(in *dt.Msg) (bool, string) {
				return sm.HasMemory(in, "city"), ""
			},
		},
		{
			OnEntry: func(in *dt.Msg) string {
				return kwGetTemp(in, 0)
			},
			OnInput: func(in *dt.Msg) {
			},
			Complete: func(in *dt.Msg) (bool, string) {
				return true, ""
			},
		},
	})
	return sm
}

func e(err error) string {
	l.Debug(err)
	return "Something went wrong, but I'll try to get that fixed right away."
}
