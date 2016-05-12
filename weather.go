package weather

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/itsabot/abot/shared/datatypes"
	"github.com/itsabot/abot/shared/language"
	"github.com/itsabot/abot/shared/nlp"
	"github.com/itsabot/abot/shared/plugin"
)

var p *dt.Plugin

func init() {
	var err error
	p, err = plugin.New("github.com/itsabot/plugin_weather")
	if err != nil {
		log.Fatal(err)
	}
	plugin.SetKeywords(p,
		dt.KeywordHandler{
			Fn: kwGetTemp,
			Trigger: &nlp.StructuredInput{
				Commands: []string{"what", "show", "tell",
					"how"},
				Objects: []string{"weather", "temperature",
					"temp", "outside"},
			},
		},
		dt.KeywordHandler{
			Fn: kwGetRaining,
			Trigger: &nlp.StructuredInput{
				Commands: []string{"tell", "is"},
				Objects:  []string{"rain"},
			},
		},
	)
	plugin.SetStates(p, [][]dt.State{[]dt.State{
		dt.State{
			OnEntry: func(in *dt.Msg) string {
				return "What city are you in?"
			},
			OnInput: func(in *dt.Msg) {
				cities, _ := language.ExtractCities(p.DB, in)
				if len(cities) > 0 {
					p.SetMemory(in, "city", cities[0])
				}
			},
			Complete: func(in *dt.Msg) (bool, string) {
				return p.HasMemory(in, "city"), ""
			},
			SkipIfComplete: true,
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
	}})
	if err = plugin.Register(p); err != nil {
		p.Log.Fatal(err)
	}
}

func kwGetTemp(in *dt.Msg) (resp string) {
	city, err := getCity(in)
	if err == language.ErrNotFound {
		return ""
	}
	if err != nil {
		p.Log.Info("failed to getCity.", err)
		return ""
	}
	p.SetMemory(in, "city", city)
	return getWeather(city)
}

func kwGetRaining(in *dt.Msg) (resp string) {
	city, err := getCity(in)
	if err == language.ErrNotFound {
		return ""
	}
	if err != nil {
		p.Log.Info("failed to getCity.", err)
		return ""
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
	if p.HasMemory(in, "city") {
		mem := p.GetMemory(in, "city")
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
	var req struct {
		Description []string
		Temp        float64
		Humidity    int
	}
	n := url.QueryEscape(city.Name)
	resp, err := http.Get("https://www.itsabot.org/api/weather/" + n)
	if err != nil {
		return ""
	}
	p.Log.Debug("decoding resp")
	if err = json.NewDecoder(resp.Body).Decode(&req); err != nil {
		return ""
	}
	if err = resp.Body.Close(); err != nil {
		return ""
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
