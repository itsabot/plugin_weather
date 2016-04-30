package weather

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/itsabot/abot/core"
	"github.com/julienschmidt/httprouter"
)

var r *httprouter.Router
var phone string

func TestMain(m *testing.M) {
	if err := os.Setenv("ABOT_ENV", "test"); err != nil {
		log.Fatal(err)
	}
	flag.StringVar(&phone, "phone", "+13105555555", "phone number of test user")
	flag.Parse()
	var err error
	r, err = core.NewServer()
	if err != nil {
		log.Fatal("failed to start Abot server", err)
	}
	os.Exit(m.Run())
}

func TestServer(t *testing.T) {
	req := "what's the weather in LA?"
	u := fmt.Sprintf("http://localhost:%s?flexidtype=2&flexid=%s&cmd=%s",
		os.Getenv("ABOT_PORT"), url.QueryEscape(phone), url.QueryEscape(req))
	c, b := request("POST", u, nil)
	if c != http.StatusOK {
		t.Fatal("expected", http.StatusOK, "got", c, b)
	}
	var matches int
	for _, w := range strings.Fields(b) {
		if w == "It's" {
			matches++
			continue
		}
		if w == "in" {
			matches++
			continue
		}
		if w == "LA." {
			matches++
			break
		}
	}
	if matches != 3 {
		t.Fatalf("expected \"It's...in LA.\" got %q\n", b)
	}
}

func request(method, path string, data []byte) (int, string) {
	req, err := http.NewRequest(method, path, bytes.NewBuffer(data))
	if err != nil {
		return 0, "err completing request: " + err.Error()
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, string(w.Body.Bytes())
}
