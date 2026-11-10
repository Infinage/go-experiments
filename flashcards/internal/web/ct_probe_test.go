package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
)

type loggingHeader struct {
	http.Header
}

func (h loggingHeader) Set(key, val string) {
	if strings.EqualFold(key, "Content-Type") {
		buf := make([]byte, 1<<12)
		buf = buf[:runtime.Stack(buf, false)]
		t := currentProbeTest
		if t != nil {
			t.Logf("CT SET to %q at stack:\n%s", val, buf)
		}
	}
	h.Header.Set(key, val)
}

var currentProbeTest *testing.T

// ctCaptor reports header state and stack at the first write.
type ctCaptor struct {
	*httptest.ResponseRecorder
	t       *testing.T
	wrote   bool
	flipped bool
	n       int
}

func (c *ctCaptor) Write(b []byte) (int, error) {
	c.n++
	ct := c.Header().Get("Content-Type")
	if !c.wrote {
		c.wrote = true
		buf := make([]byte, 1<<13)
		buf = buf[:runtime.Stack(buf, false)]
		c.t.Logf("first Write(#%d), CT=%q fullHeader=%v stack:\n%s", c.n, ct, c.Header(), buf)
	}
	if ct != "" && !c.flipped && c.n > 1 {
		c.flipped = true
		buf := make([]byte, 1<<13)
		buf = buf[:runtime.Stack(buf, false)]
		c.t.Logf("write#%d CT became %q chunk=%q stack:\n%s", c.n, ct, b, buf)
	}
	if c.n < 4 || c.n%5000 == 0 {
		c.t.Logf("write#%d len=%d CT=%q", c.n, len(b), ct)
	}
	return c.ResponseRecorder.Write(b)
}

func TestEditPageContentTypeProbe(t *testing.T) {
	currentProbeTest = t
	defer func() { currentProbeTest = nil }()
	out := fstest.MapFS{}
	entries, _ := os.ReadDir("../../assets/html")
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		data, _ := os.ReadFile("../../assets/html/" + e.Name())
		out["html/"+e.Name()] = &fstest.MapFile{Data: data}
	}
	out["css/index.css"] = &fstest.MapFile{Data: []byte("body{}")}
	h := newHandlerWith(t, out)

	rec := httptest.NewRecorder()
	capt := &ctCaptor{ResponseRecorder: rec, t: t}
	req := httptest.NewRequest("GET", "/cards/new", nil)
	h.ServeHTTP(capt, req)
	t.Logf("final CT=%q code=%d startsDoctype=%v", rec.Header().Get("Content-Type"), rec.Code, strings.HasPrefix(strings.TrimSpace(rec.Body.String()), "<!DOCTYPE"))
}
