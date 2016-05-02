package weather

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/itsabot/abot/shared/datatypes"
	"github.com/itsabot/abot/shared/language"
	"github.com/itsabot/abot/shared/nlp"
	"github.com/itsabot/abot/shared/plugin"
)

type weatherJSON struct {
	Description []string
	Temp        float64
	Humidity    int
}

var p *dt.Plugin

func init() {
	rand.Seed(time.Now().UnixNano())
	trigger := &nlp.StructuredInput{
		Commands: []string{"what", "show", "tell", "is", "how"},
		Objects: []string{"weather", "temperature", "temp", "outside",
			"raining"},
	}
	fns := &dt.PluginFns{Run: Run, FollowUp: FollowUp}
	var err error
	p, err = plugin.New("github.com/itsabot/plugin_weather", trigger, fns)
	if err != nil {
		log.Fatal(err)
	}
	p.Vocab = dt.NewVocab(
		dt.VocabHandler{
			Fn: kwGetTemp,
			Trigger: &nlp.StructuredInput{
				Commands: []string{"what", "show", "tell", "how"},
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
}

func Run(in *dt.Msg) (string, error) {
	sm := buildStateMachine(in)
	sm.Reset(in)
	return FollowUp(in)
}

func FollowUp(in *dt.Msg) (string, error) {
	resp := p.Vocab.HandleKeywords(in)
	if len(resp) == 0 {
		sm := buildStateMachine(in)
		resp = sm.Next(in)
	}
	return resp, nil
}

func kwGetTemp(in *dt.Msg) (resp string) {
	city, err := getCity(in)
	if err == language.ErrNotFound {
		return ""
	}
	if err != nil {
		return er(err)
	}
	sm := buildStateMachine(in)
	sm.SetMemory(in, "city", city)
	return getWeather(city)
}

func kwGetRaining(in *dt.Msg) (resp string) {
	city, err := getCity(in)
	if err == language.ErrNotFound {
		return ""
	}
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
	cities, err := language.ExtractCities(p.DB, in)
	if err != nil && err != language.ErrNotFound {
		p.Log.Debug("couldn't extract cities")
		return nil, err
	}
	if len(cities) >= 1 {
		return &cities[0], nil
	}
	sm := buildStateMachine(in)
	if sm.HasMemory(in, "city") {
		mem := sm.GetMemory(in, "city")
		p.Log.Debug(mem)
		city := &dt.City{}
		if err := json.Unmarshal(mem.Val, city); err != nil {
			p.Log.Info("couldn't unmarshal mem into city.", err)
			return nil, err
		}
		return city, nil
	}
	return nil, language.ErrNotFound
}

func getWeather(city *dt.City) string {
	p.Log.Debug("getting weather for city", city.Name)
	req := weatherJSON{}
	n := url.QueryEscape(city.Name)
	resp, err := http.Get("https://www.itsabot.org/api/weather/" + n)
	if err != nil {
		return er(err)
	}
	p.Log.Debug("decoding resp")
	if err = json.NewDecoder(resp.Body).Decode(&req); err != nil {
		return er(err)
	}
	p.Log.Debug("closing resp.Body")
	if err = resp.Body.Close(); err != nil {
		return er(err)
	}
	p.Log.Debug("got weather")
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
	sm := dt.NewStateMachine(p)
	sm.SetStates([]dt.State{
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				if !sm.HasMemory(in, "city") {
					return "What city are you in?"
				}
				mem := sm.GetMemory(in, "city")
				p.Log.Debug(mem)
				city := &dt.City{}
				if err := json.Unmarshal(mem.Val, city); err != nil {
					return er(err)
				}
				return fmt.Sprintf("Are you still in %s?", city.Name)
			},
			OnInput: func(in *dt.Msg) {
				cities, _ := language.ExtractCities(p.DB, in)
				if len(cities) > 0 {
					sm.SetMemory(in, "city", cities[0])
				}
			},
			Complete: func(in *dt.Msg) (bool, string) {
				return sm.HasMemory(in, "city"), ""
			},
		},
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				return kwGetTemp(in)
			},
			OnInput: func(in *dt.Msg) {},
			Complete: func(in *dt.Msg) (bool, string) {
				return true, ""
			},
		},
	})
	sm.LoadState(in)
	return sm
}

func er(err error) string {
	p.Log.Debug(err)
	return "Something went wrong, but I'll try to get that fixed right away."
}
