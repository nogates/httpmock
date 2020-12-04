package httpmock_test

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"

	. "github.com/jarcoal/httpmock"
	"github.com/jarcoal/httpmock/internal"
)

func TestResponderFromResponse(t *testing.T) {
	responder := ResponderFromResponse(NewStringResponse(200, "hello world"))

	req, err := http.NewRequest(http.MethodGet, testURL, nil)
	if err != nil {
		t.Fatal("Error creating request")
	}
	response1, err := responder(req)
	if err != nil {
		t.Error("Error should be nil")
	}

	testURLWithQuery := testURL + "?a=1"
	req, err = http.NewRequest(http.MethodGet, testURLWithQuery, nil)
	if err != nil {
		t.Fatal("Error creating request")
	}
	response2, err := responder(req)
	if err != nil {
		t.Error("Error should be nil")
	}

	// Body should be the same for both responses
	assertBody(t, response1, "hello world")
	assertBody(t, response2, "hello world")

	// Request should be non-nil and different for each response
	if response1.Request != nil && response2.Request != nil {
		if response1.Request.URL.String() != testURL {
			t.Errorf("Expected request url %s, got: %s", testURL, response1.Request.URL.String())
		}
		if response2.Request.URL.String() != testURLWithQuery {
			t.Errorf("Expected request url %s, got: %s", testURLWithQuery, response2.Request.URL.String())
		}
	} else {
		t.Error("response.Request should not be nil")
	}
}

func TestNewNotFoundResponder(t *testing.T) {
	responder := NewNotFoundResponder(func(args ...interface{}) {})

	req, err := http.NewRequest("GET", "http://foo.bar/path", nil)
	if err != nil {
		t.Fatal("Error creating request")
	}

	const title = "Responder not found for GET http://foo.bar/path"

	resp, err := responder(req)
	if resp != nil {
		t.Error("resp should be nil")
	}
	if err == nil {
		t.Error("err should be not nil")
	} else if err.Error() != title {
		t.Errorf(`err mismatch, got: "%s", expected: "%s"`,
			err, "Responder not found for: GET http://foo.bar/path")
	} else if ne, ok := err.(internal.StackTracer); !ok {
		t.Errorf(`err type mismatch, got %T, expected httpmock.notFound`, err)
	} else if ne.CustomFn == nil {
		t.Error(`err CustomFn mismatch, got: nil, expected: non-nil`)
	}

	// nil fn
	responder = NewNotFoundResponder(nil)

	resp, err = responder(req)
	if resp != nil {
		t.Error("resp should be nil")
	}
	if err == nil {
		t.Error("err should be not nil")
	} else if err.Error() != title {
		t.Errorf(`err mismatch, got: "%s", expected: "%s"`,
			err, "Responder not found for: GET http://foo.bar/path")
	} else if ne, ok := err.(internal.StackTracer); !ok {
		t.Errorf(`err type mismatch, got %T, expected httpmock.notFound`, err)
	} else if ne.CustomFn != nil {
		t.Errorf(`err CustomFn mismatch, got: %p, expected: nil`, ne.CustomFn)
	}
}

func TestNewStringResponse(t *testing.T) {
	body := "hello world"
	status := 200
	response := NewStringResponse(status, body)

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != body {
		t.FailNow()
	}

	if response.StatusCode != status {
		t.FailNow()
	}
}

func TestNewBytesResponse(t *testing.T) {
	body := []byte("hello world")
	status := 200
	response := NewBytesResponse(status, body)

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != string(body) {
		t.FailNow()
	}

	if response.StatusCode != status {
		t.FailNow()
	}
}

func TestNewJsonResponse(t *testing.T) {
	type schema struct {
		Hello string `json:"hello"`
	}

	body := &schema{"world"}
	status := 200

	response, err := NewJsonResponse(status, body)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != status {
		t.FailNow()
	}

	if response.Header.Get("Content-Type") != "application/json" {
		t.FailNow()
	}

	checkBody := &schema{}
	if err := json.NewDecoder(response.Body).Decode(checkBody); err != nil {
		t.Fatal(err)
	}

	if checkBody.Hello != body.Hello {
		t.FailNow()
	}

	// Error case
	response, err = NewJsonResponse(200, func() {})
	if response != nil {
		t.Fatal("response is not nil")
	}
	if err == nil {
		t.Fatal("no error occurred")
	}
}

func checkResponder(t *testing.T, r Responder, expectedStatus int, expectedBody string) {
	helper(t).Helper()

	req, _ := http.NewRequest(http.MethodGet, "/foo", nil)
	resp, err := r(req)
	if err != nil {
		t.Errorf("An error occurred: %s", err)
		return
	}

	if resp == nil {
		t.Error("Responder returned a nil response")
		return
	}

	if resp.StatusCode != expectedStatus {
		t.Errorf("Status code mismatch: got=%d expected=%d",
			resp.StatusCode, expectedStatus)
	}

	assertBody(t, resp, expectedBody)
}

func TestNewJsonResponder(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		r, err := NewJsonResponder(200, map[string]int{"foo": 42})
		if err != nil {
			t.Error(err)
			return
		}
		checkResponder(t, r, 200, `{"foo":42}`)
	})

	t.Run("Error", func(t *testing.T) {
		r, err := NewJsonResponder(200, func() {})
		if r != nil {
			t.Error("responder is not nil")
		}
		if err == nil {
			t.Error("no error occurred")
		}
	})

	t.Run("OK don't panic", func(t *testing.T) {
		panicked, str := catchPanic(
			func() {
				r := NewJsonResponderOrPanic(200, map[string]int{"foo": 42})
				checkResponder(t, r, 200, `{"foo":42}`)
			},
		)
		if panicked {
			t.Errorf("A panic occurred: <%s>", str)
		}
	})

	t.Run("Panic", func(t *testing.T) {
		panicked, _ := catchPanic(
			func() { NewJsonResponderOrPanic(200, func() {}) },
		)
		if !panicked {
			t.Error("no panic occurred")
		}
	})
}

type schemaXML struct {
	Hello string `xml:"hello"`
}

func TestNewXmlResponse(t *testing.T) {
	body := &schemaXML{"world"}
	status := 200

	response, err := NewXmlResponse(status, body)
	if err != nil {
		t.Fatal(err)
	}

	if response.StatusCode != status {
		t.FailNow()
	}

	if response.Header.Get("Content-Type") != "application/xml" {
		t.FailNow()
	}

	checkBody := &schemaXML{}
	if err := xml.NewDecoder(response.Body).Decode(checkBody); err != nil {
		t.Fatal(err)
	}

	if checkBody.Hello != body.Hello {
		t.FailNow()
	}

	// Error case
	response, err = NewXmlResponse(200, func() {})
	if response != nil {
		t.Fatal("response is not nil")
	}
	if err == nil {
		t.Fatal("no error occurred")
	}
}

func TestNewXmlResponder(t *testing.T) {
	body := &schemaXML{"world"}

	b, err := xml.Marshal(body)
	if err != nil {
		t.Fatalf("Cannot xml.Marshal expected body: %s", err)
	}
	expectedBody := string(b)

	t.Run("OK", func(t *testing.T) {
		r, err := NewXmlResponder(200, body)
		if err != nil {
			t.Error(err)
			return
		}
		checkResponder(t, r, 200, expectedBody)
	})

	t.Run("Error", func(t *testing.T) {
		r, err := NewXmlResponder(200, func() {})
		if r != nil {
			t.Error("responder is not nil")
		}
		if err == nil {
			t.Error("no error occurred")
		}
	})

	t.Run("OK don't panic", func(t *testing.T) {
		panicked, str := catchPanic(
			func() {
				r := NewXmlResponderOrPanic(200, body)
				checkResponder(t, r, 200, expectedBody)
			},
		)
		if panicked {
			t.Errorf("A panic occurred: <%s>", str)
		}
	})

	t.Run("Panic", func(t *testing.T) {
		panicked, _ := catchPanic(
			func() { NewXmlResponderOrPanic(200, func() {}) },
		)
		if !panicked {
			t.Error("no panic occurred")
		}
	})
}

func TestNewErrorResponder(t *testing.T) {
	// From go1.13, a stack frame is stored into errors issued by errors.New()
	origError := errors.New("oh no")
	responder := NewErrorResponder(origError)
	req, err := http.NewRequest(http.MethodGet, testURL, nil)
	if err != nil {
		t.Fatal("Error creating request")
	}
	response, err := responder(req)
	if response != nil {
		t.Error("Response should be nil")
	}
	if err != origError {
		t.Errorf("Expected error %#v, got: %#v", origError, err)
	}
}

func TestRewindResponse(t *testing.T) {
	body := []byte("hello world")
	status := 200
	responses := []*http.Response{
		NewBytesResponse(status, body),
		NewStringResponse(status, string(body)),
	}

	for _, response := range responses {
		data, err := ioutil.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}

		if string(data) != string(body) {
			t.FailNow()
		}

		if response.StatusCode != status {
			t.FailNow()
		}

		data, err = ioutil.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}

		if string(data) != string(body) {
			t.FailNow()
		}

		if response.StatusCode != status {
			t.FailNow()
		}
	}
}

func TestResponder(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://foo.bar", nil)
	if err != nil {
		t.Fatal("Error creating request")
	}
	resp := &http.Response{}

	chk := func(r Responder, expectedResp *http.Response, expectedErr string) {
		helper(t).Helper()
		gotResp, gotErr := r(req)
		if gotResp != expectedResp {
			t.Errorf(`Response mismatch, expected: %v, got: %v`, expectedResp, gotResp)
		}
		var gotErrStr string
		if gotErr != nil {
			gotErrStr = gotErr.Error()
		}
		if gotErrStr != expectedErr {
			t.Errorf(`Error mismatch, expected: %v, got: %v`, expectedErr, gotErrStr)
		}
	}
	called := false
	chkNotCalled := func() {
		if called {
			helper(t).Helper()
			t.Errorf("Original responder should not be called")
			called = false
		}
	}
	chkCalled := func() {
		if !called {
			helper(t).Helper()
			t.Errorf("Original responder should be called")
		}
		called = false
	}

	r := Responder(func(*http.Request) (*http.Response, error) {
		called = true
		return resp, nil
	})
	chk(r, resp, "")
	chkCalled()

	//
	// Once
	ro := r.Once()
	chk(ro, resp, "")
	chkCalled()

	chk(ro, nil, "Responder not found for GET http://foo.bar (coz Once and already called 2 times)")
	chkNotCalled()

	chk(ro, nil, "Responder not found for GET http://foo.bar (coz Once and already called 3 times)")
	chkNotCalled()

	ro = r.Once(func(args ...interface{}) {})
	chk(ro, resp, "")
	chkCalled()

	chk(ro, nil, "Responder not found for GET http://foo.bar (coz Once and already called 2 times)")
	chkNotCalled()

	//
	// Times
	rt := r.Times(2)
	chk(rt, resp, "")
	chkCalled()

	chk(rt, resp, "")
	chkCalled()

	chk(rt, nil, "Responder not found for GET http://foo.bar (coz Times and already called 3 times)")
	chkNotCalled()

	chk(rt, nil, "Responder not found for GET http://foo.bar (coz Times and already called 4 times)")
	chkNotCalled()

	rt = r.Times(1, func(args ...interface{}) {})
	chk(rt, resp, "")
	chkCalled()

	chk(rt, nil, "Responder not found for GET http://foo.bar (coz Times and already called 2 times)")
	chkNotCalled()

	//
	// Trace
	rt = r.Trace(func(args ...interface{}) {})
	chk(rt, resp, "")
	chkCalled()

	chk(rt, resp, "")
	chkCalled()
}

func TestParallelResponder(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://foo.bar", nil)
	if err != nil {
		t.Fatal("Error creating request")
	}

	body := strings.Repeat("ABC-", 1000)

	for _, r := range []Responder{
		NewStringResponder(200, body),
		NewBytesResponder(200, []byte(body)),
	} {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, _ := r(req)
				b, err := ioutil.ReadAll(resp.Body)
				switch {
				case err != nil:
					t.Errorf("ReadAll error: %s", err)
				case len(b) != 4000:
					t.Errorf("ReadAll read only %d bytes", len(b))
				case !strings.HasPrefix(string(b), "ABC-"):
					t.Errorf("ReadAll does not read the right prefix: %s", string(b)[0:4])
				}
			}()
		}
		wg.Wait()
	}
}
