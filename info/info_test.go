package info

import (
	"bytes"
	"encoding/json"
	"expvar"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/DataDog/datadog-trace-agent/config"
	"github.com/stretchr/testify/assert"
)

type testServerHandler struct {
	t *testing.T
}

func (h *testServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json, err := ioutil.ReadFile("./test_cases/okay.json")
	if err != nil {
		h.t.Errorf("error loading json file: %v", err)
	}

	switch r.URL.Path {
	case "/debug/vars":
		h.t.Logf("serving fake (static) info data for %s", r.URL.Path)
		_, err := w.Write(json)
		if err != nil {
			h.t.Errorf("error serving %s: %v", r.URL.Path, err)
		}
	default:
		h.t.Logf("answering 404 for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func testServer(t *testing.T) *httptest.Server {
	server := httptest.NewServer(&testServerHandler{t: t})
	t.Logf("test server (serving fake yet valid data) listening on %s", server.URL)
	return server
}

type testServerWarningHandler struct {
	t *testing.T
}

func (h *testServerWarningHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json, err := ioutil.ReadFile("./test_cases/warning.json")
	if err != nil {
		h.t.Errorf("error loading json file: %v", err)
	}

	switch r.URL.Path {
	case "/debug/vars":
		h.t.Logf("serving fake (static) info data for %s", r.URL.Path)
		_, err := w.Write(json)
		if err != nil {
			h.t.Errorf("error serving %s: %v", r.URL.Path, err)
		}
	default:
		h.t.Logf("answering 404 for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func testServerWarning(t *testing.T) *httptest.Server {
	server := httptest.NewServer(&testServerWarningHandler{t: t})
	t.Logf("test server (serving data containing worrying values) listening on %s", server.URL)
	return server
}

type testServerErrorHandler struct {
	t *testing.T
}

func (h *testServerErrorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	switch r.URL.Path {
	case "/debug/vars":
		h.t.Logf("serving fake (static) info data for %s", r.URL.Path)
		_, err := w.Write([]byte(`this is *NOT* a valid JSON, no way...`))
		if err != nil {
			h.t.Errorf("error serving %s: %v", r.URL.Path, err)
		}
	default:
		h.t.Logf("answering 404 for %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func testServerError(t *testing.T) *httptest.Server {
	server := httptest.NewServer(&testServerErrorHandler{t: t})
	t.Logf("test server (serving bad data to trigger errors) listening on %s", server.URL)
	return server
}

// run this at the beginning of each test, this is because we *really*
// need to have InitInfo be called before doing anything
func testInit(t *testing.T) *config.AgentConfig {
	assert := assert.New(t)
	conf := config.NewDefaultAgentConfig()
	assert.NotNil(conf)

	err := InitInfo(conf)
	assert.Nil(err)

	return conf
}

func TestInfo(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServer(t)
	assert.NotNil(server)
	defer server.Close()

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.NoError(err)
	info := buf.String()
	t.Logf("Info:\n%s\n", info)
	expectedInfo, err := ioutil.ReadFile("./test_cases/okay.info")
	assert.NoError(err)
	assert.Equal(string(expectedInfo), info)
}

func TestWarning(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServerWarning(t)
	assert.NotNil(server)
	defer server.Close()

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.Nil(err)
	info := buf.String()

	expectedWarning, err := ioutil.ReadFile("./test_cases/warning.info")
	assert.NoError(err)
	assert.Equal(string(expectedWarning), info)

	t.Logf("Info:\n%s\n", info)
}

func TestNotRunning(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServer(t)
	assert.NotNil(server)

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	server.Close()

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.NotNil(err)
	info := buf.String()

	t.Logf("Info:\n%s\n", info)

	lines := strings.Split(info, "\n")
	assert.Equal(7, len(lines))
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[0])
	assert.Regexp(regexp.MustCompile(`^Trace Agent \(v.*\)$`), lines[1])
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[2])
	assert.Equal(len(lines[1]), len(lines[0]))
	assert.Equal(len(lines[1]), len(lines[2]))
	assert.Equal("", lines[3])
	assert.Equal(fmt.Sprintf("  Not running (port %d)", port), lines[4])
	assert.Equal("", lines[5])
	assert.Equal("", lines[6])
}

func TestError(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	server := testServerError(t)
	assert.NotNil(server)
	defer server.Close()

	url, err := url.Parse(server.URL)
	assert.NotNil(url)
	assert.Nil(err)

	hostPort := strings.Split(url.Host, ":")
	assert.Equal(2, len(hostPort))
	port, err := strconv.Atoi(hostPort[1])
	assert.Nil(err)
	conf.ReceiverPort = port

	var buf bytes.Buffer
	err = Info(&buf, conf)
	assert.NotNil(err)
	info := buf.String()

	t.Logf("Info:\n%s\n", info)

	lines := strings.Split(info, "\n")
	assert.Equal(8, len(lines))
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[0])
	assert.Regexp(regexp.MustCompile(`^Trace Agent \(v.*\)$`), lines[1])
	assert.Regexp(regexp.MustCompile(`^={10,100}$`), lines[2])
	assert.Equal(len(lines[1]), len(lines[0]))
	assert.Equal(len(lines[1]), len(lines[2]))
	assert.Equal("", lines[3])
	assert.Regexp(regexp.MustCompile(`^  Error: .*$`), lines[4])
	assert.Equal(fmt.Sprintf("  URL: http://localhost:%d/debug/vars", port), lines[5])
	assert.Equal("", lines[6])
	assert.Equal("", lines[7])
}

func TestInfoReceiverStats(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	stats := NewReceiverStats()
	t1 := &TagStats{
		Tags{Lang: "python"},
		Stats{TracesReceived: 23, TracesDropped: 2, TracesBytes: 3244, SpansReceived: 213, SpansDropped: 14},
	}
	t2 := &TagStats{
		Tags{Lang: "go"},
		Stats{ServicesReceived: 4, ServicesBytes: 1543},
	}
	stats.Stats = map[Tags]*TagStats{
		t1.Tags: t1,
		t2.Tags: t2,
	}

	// run this with -race flag
	done := make(chan struct{}, 4)
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				UpdateReceiverStats(stats)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 2; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				_ = publishReceiverStats()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 4; i++ {
		<-done
	}
	s := publishReceiverStats()
	switch s := s.(type) {
	case []TagStats:
		for _, tagStats := range s {
			assert.Equal(*stats.Stats[tagStats.Tags], tagStats)
		}
	default:
		t.Errorf("bad stats type: %v", s)
	}
	stats.Stats[t1.Tags].TracesReceived++
	UpdateReceiverStats(stats)
	s = publishReceiverStats()
	switch s := s.(type) {
	case []TagStats:
		for _, tagStats := range s {
			if tagStats.Tags == t1.Tags {
				assert.Equal(t1.Stats.TracesReceived, tagStats.Stats.TracesReceived)
			}
		}
	default:
		t.Errorf("bad stats type: %v", s)
	}
}

func TestInfoConfig(t *testing.T) {
	assert := assert.New(t)
	conf := testInit(t)
	assert.NotNil(conf)

	js := expvar.Get("config").String() // this is what expvar will call
	assert.NotEqual("", js)
	var confCopy config.AgentConfig
	err := json.Unmarshal([]byte(js), &confCopy)
	assert.Nil(err)
	assert.Equal("", confCopy.APIKey, "API Keys should *NEVER* be exported")
	conf.APIKey = ""              // patch upstream source so that we can use equality testing
	assert.Equal(*conf, confCopy) // ensure all fields have been exported then parsed correctly
}
