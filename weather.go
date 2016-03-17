package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
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
var rpcAddr string

const pluginName = "weather"

func main() {
	var addr string
	flag.StringVar(&addr, "coreaddr", "", "Port to communicate with Abot.")
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
	p, err = plugin.New(pluginName, addr, trigger)
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
	if os.Getenv("ABOT_ENV") == "test" {
		p.Config.CoreRPCAddr = rpcAddr
	}
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
	return nil
}

func kwGetTemp(in *dt.Msg, _ int) (resp string) {
	city, err := getCity(in)
	if err != nil {
		return er(err)
	}
	return getWeather(city)
}

func kwGetRaining(in *dt.Msg, _ int) (resp string) {
	city, err := getCity(in)
	if err != nil {
		return er(err)
	}
	resp = getWeather(city)
	for _, w := range strings.Fields(resp) {
		if w == "rain" {
			return fmt.Sprintf("It's raining in %s right now.",
				city.Name)
		}
	}
	return fmt.Sprintf("It's not raining in %s right now.", city.Name)
}

func getCity(in *dt.Msg) (*dt.City, error) {
	cities, err := language.ExtractCities(db, in)
	if err != nil {
		l.Debug("couldn't extract cities")
		return nil, err
	}
	city := &dt.City{}
	sm := buildStateMachine(in)
	if len(cities) >= 1 {
		city = &cities[0]
	} else if sm.HasMemory(in, "city") {
		mem := sm.GetMemory(in, "city")
		l.Debug(mem)
		if err := json.Unmarshal(mem.Val, city); err != nil {
			l.Debug("couldn't unmarshal mem into city", err)
			return nil, err
		}
	}
	if city == nil {
		return nil, errors.New("no cities found")
	}
	return city, nil
}

func getWeather(city *dt.City) string {
	l.Debug("getting weather for city", city.Name)
	req := weatherJSON{}
	n := url.QueryEscape(city.Name)
	resp, err := http.Get("https://www.itsabot.org/api/weather.json?city=" + n)
	if err != nil {
		return er(err)
	}
	l.Debug("decoding resp")
	if err = json.NewDecoder(resp.Body).Decode(&req); err != nil {
		return er(err)
	}
	l.Debug("closing resp.Body")
	if err = resp.Body.Close(); err != nil {
		return er(err)
	}
	l.Debug("got weather")
	var ret string
	if len(req.Description) == 0 {
		ret = fmt.Sprintf("It's %.f in %s right now.", req.Temp,
			city.Name)
	} else {
		ret = fmt.Sprintf("It's %.0f with %s in %s.", req.Temp,
			req.Description[0], city.Name)
	}
	return ret
}

func buildStateMachine(in *dt.Msg) *dt.StateMachine {
	sm := dt.NewStateMachine(pluginName)
	sm.SetDBConn(db)
	sm.SetLogger(l)
	sm.SetStates([]dt.State{})
	sm.LoadState(in)
	return sm
}

func er(err error) string {
	l.Debug(err)
	return "Something went wrong, but I'll try to get that fixed right away."
}
