// Copyright 2017 Steven E. Harris. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

package handler_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/seh/handler"
)

func ensurePanicWithValueOccured(t *testing.T) {
	if p := recover(); p == nil {
		t.Error("panic was not called with a non-nil argument")
	}
}

// fakeSecureCookieError is a securecookie.Error that can impersonate a decoding or validation
// error, depending on whether its underlying value is true or false.
type fakeSecureCookieError bool

func (fakeSecureCookieError) Error() string {
	return "fake"
}

func (fakeSecureCookieError) IsUsage() bool {
	return false
}

func (e fakeSecureCookieError) IsDecode() bool {
	return bool(e)
}

func (fakeSecureCookieError) IsInternal() bool {
	return false
}

func (fakeSecureCookieError) Cause() error {
	return nil
}

type failingSessionSource struct {
	err error
}

func (f failingSessionSource) New(*http.Request, string) (*sessions.Session, error) {
	// The real sessions.Store implementations never return a nil session.
	s := &sessions.Session{
		IsNew: true,
	}
	return s, f.err
}

func TestWithSessionPanicsWithNoSource(t *testing.T) {
	delegate := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("HTTP handler should not have been called")
	})
	onError := func(w http.ResponseWriter, r *http.Request, err error) {
		t.Error("onError handler should not have been called")
	}
	defer ensurePanicWithValueOccured(t)
	handler.WithSession("s", nil, delegate, onError)
}

func TestWithSessionPanicsWithNoHandler(t *testing.T) {
	var source countingSessionSource
	defer func() {
		if got, want := source.callCount(), uint(0); got != want {
			t.Errorf("source call count: got %d, want %d", got, want)
		}
	}()
	onError := func(w http.ResponseWriter, r *http.Request, err error) {
		t.Error("onError handler should not have been called")
	}
	defer ensurePanicWithValueOccured(t)
	handler.WithSession("s", &source, nil, onError)
}

func TestWithSessionSourceFailure(t *testing.T) {
	tests := []struct {
		description   string
		expectedError error
	}{
		{"other", errors.New("")},
		{"non-decode", fakeSecureCookieError(false)},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			source := failingSessionSource{test.expectedError}
			called := false
			onError := func(w http.ResponseWriter, r *http.Request, err error) {
				called = true
				if err != test.expectedError {
					t.Error("onError handler received wrong error")
				}
			}
			delegate := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			handler := handler.WithSession("s", source, delegate, onError)
			if handler == nil {
				t.Fatal("WithSession returned nil")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest("", "/", nil))
			if !called {
				t.Error("onError handler was not called")
			}
		})
	}
}

func TestWithSessionCookieExtractionError(t *testing.T) {
	tests := []struct {
		description   string
		expectedError error
	}{
		{"absent", http.ErrNoCookie},
		{"invalid", fakeSecureCookieError(true)},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			source := failingSessionSource{test.expectedError}
			onError := func(w http.ResponseWriter, r *http.Request, err error) {
				t.Error("onError handler called unexpectedly")
			}
			called := false
			delegate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				session := handler.MustExtractSession(r)
				if session == nil {
					t.Fatal("extracted session was nil")
				}
				if !session.IsNew {
					t.Error("extracted session is not new")
				}
			})
			handler := handler.WithSession("s", source, delegate, onError)
			if handler == nil {
				t.Fatal("WithSession returned nil")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest("", "/", nil))
			if !called {
				t.Error("delegate handler was not called")
			}
		})
	}
}

func ensureResponseIsInternalError(t *testing.T, handler http.Handler) {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("", "/", nil))
	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
	body, err := ioutil.ReadAll(recorder.Body)
	if err != nil {
		t.Fatalf("failed to read the response body: %v", err)
	}
	if got, want := len(body), 0; got != want {
		t.Errorf("response body length: got %d, want %d", got, want)
	}
}

func TestWithSessionSourceFailureWithNoErrorHandler(t *testing.T) {
	source := failingSessionSource{errors.New("")}
	delegate := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	handler := handler.WithSession("s", source, delegate, nil)
	if handler == nil {
		t.Fatal("WithSession returned nil")
	}
	ensureResponseIsInternalError(t, handler)
}

type simpleStore struct{}

func (s simpleStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.NewSession(s, name), nil
}

func (s simpleStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(s, name)
	session.IsNew = true
	return session, nil
}

func (simpleStore) Save(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return nil
}

type countingSessionSource uint

func (s *countingSessionSource) New(r *http.Request, name string) (*sessions.Session, error) {
	*s++
	return simpleStore{}.New(r, name)
}

func (s countingSessionSource) callCount() uint {
	return uint(s)
}

func TestWithSession(t *testing.T) {
	onError := func(w http.ResponseWriter, r *http.Request, err error) {
		t.Error("onError handler called unexpectedly")
	}
	var source countingSessionSource
	called := false
	delegate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		session := handler.MustExtractSession(r)
		if session == nil {
			t.Fatal("extracted session was nil")
		}
		if !session.IsNew {
			t.Error("extracted session is not new")
		}
	})
	handler := handler.WithSession("s", &source, delegate, onError)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest("", "/", nil))
	if !called {
		t.Error("delegate handler was not called")
	}
	if got, want := source.callCount(), uint(1); got != want {
		t.Errorf("source call count: got %d, want %d", got, want)
	}
}

func TestExtractSessionReportsAbsence(t *testing.T) {
	if _, ok := handler.ExtractSession(httptest.NewRequest("", "/", nil)); ok {
		t.Fatal("got true, want false")
	}
}

func TestMustExtractSessionPanics(t *testing.T) {
	r := httptest.NewRequest("", "/", nil)
	defer ensurePanicWithValueOccured(t)
	handler.MustExtractSession(r)
}

func TestWithSessionsNamedPanicsWithNoSource(t *testing.T) {
	delegate := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("HTTP handler should not have been called")
	})
	onError := func(w http.ResponseWriter, r *http.Request, name string, err error) {
		t.Error("onError handler should not have been called")
	}
	defer ensurePanicWithValueOccured(t)
	handler.WithSessionsNamed([]string{"s1", "s2"}, nil, delegate, onError)
}

func TestWithSessionsNamedPanicsWithNoHandler(t *testing.T) {
	var source countingSessionSource
	defer func() {
		if got, want := source.callCount(), uint(0); got != want {
			t.Errorf("source call count: got %d, want %d", got, want)
		}
	}()
	onError := func(w http.ResponseWriter, r *http.Request, name string, err error) {
		t.Error("onError handler should not have been called")
	}
	defer ensurePanicWithValueOccured(t)
	handler.WithSessionsNamed([]string{"s1", "s2"}, &source, nil, onError)
}

func TestWithSessionsNamedSourceFailure(t *testing.T) {
	tests := []struct {
		description   string
		expectedError error
	}{
		{"other", errors.New("")},
		{"non-decode", fakeSecureCookieError(false)},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			expectedError := test.expectedError
			source := failingSessionSource{expectedError}
			delegate := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
			tests := []struct {
				description string
				names       []string
			}{
				{"single", []string{"s"}},
				{"two unique", []string{"s1", "s"}},
			}
			for _, test := range tests {
				t.Run(test.description, func(t *testing.T) {
					called := false
					onError := func(w http.ResponseWriter, r *http.Request, name string, err error) {
						called = true
						if err != expectedError {
							t.Error("onError handler received wrong error")
						}
					}
					handler := handler.WithSessionsNamed(test.names, source, delegate, onError)
					if handler == nil {
						t.Fatal("WithSessionsNamed returned nil")
					}
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, httptest.NewRequest("", "/", nil))
					if !called {
						t.Error("onError handler was not called")
					}
				})
			}
		})
	}
}

func TestWithSessionsNamedCookieExtractionError(t *testing.T) {
	tests := []struct {
		description   string
		expectedError error
	}{
		{"absent", http.ErrNoCookie},
		{"invalid", fakeSecureCookieError(true)},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			source := failingSessionSource{test.expectedError}
			tests := []struct {
				description   string
				names         []string
				distinctCount uint
			}{
				{"single", []string{"s"}, 1},
				{"two unique", []string{"s1", "s2"}, 2},
			}
			for _, test := range tests {
				t.Run(test.description, func(t *testing.T) {
					onError := func(w http.ResponseWriter, r *http.Request, name string, err error) {
						t.Error("onError handler called unexpectedly")
					}
					called := false
					delegate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						called = true
						uniqueSessions := make(map[*sessions.Session]struct{}, len(test.names))
						present := struct{}{}
						for _, name := range test.names {
							session := handler.MustExtractSessionNamed(name, r)
							if session == nil {
								t.Fatalf("extracted session %q was nil", name)
							}
							if !session.IsNew {
								t.Errorf("extracted session %q is not new", name)
							}
							uniqueSessions[session] = present
						}
						if got, want := uint(len(uniqueSessions)), test.distinctCount; got != want {
							t.Errorf("unique sessions: got %d, want %d", got, want)
						}
					})
					handler := handler.WithSessionsNamed(test.names, source, delegate, onError)
					if handler == nil {
						t.Fatal("WithSessionsNamed returned nil")
					}
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, httptest.NewRequest("", "/", nil))
					if !called {
						t.Error("delegate handler was not called")
					}
				})
			}
		})
	}
}

func TestWithSessionsNamedSourceFailureWithNoErrorHandler(t *testing.T) {
	source := failingSessionSource{errors.New("")}
	delegate := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	handler := handler.WithSessionsNamed([]string{"s1", "s2"}, source, delegate, nil)
	if handler == nil {
		t.Fatal("WithSessionsNamed returned nil")
	}
	ensureResponseIsInternalError(t, handler)
}

func TestWithSessionsNamed(t *testing.T) {
	tests := []struct {
		description   string
		names         []string
		distinctCount uint
	}{
		{"none", nil, 0},
		{"one", []string{"s"}, 1},
		{"two duplicates", []string{"s", "s"}, 1},
		{"three duplicates", []string{"s", "s", "s"}, 1},
		{"three with a duplicate", []string{"s1", "s2", "s1"}, 2},
		{"two unique", []string{"s1", "s2"}, 2},
		{"three unique", []string{"s1", "s2", "s3"}, 3},
	}
	request := httptest.NewRequest("", "/", nil)
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			onError := func(w http.ResponseWriter, r *http.Request, name string, err error) {
				t.Errorf("onError handler called unexpectedly for %q", name)
			}
			var source countingSessionSource
			called := false
			delegate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				uniqueSessions := make(map[*sessions.Session]struct{}, len(test.names))
				present := struct{}{}
				for _, name := range test.names {
					session, ok := handler.ExtractSessionNamed(name, r)
					if !ok {
						t.Fatalf("session %q is not available in request", name)
					}
					if session == nil {
						t.Fatalf("extracted session %q was nil", name)
					}
					if !session.IsNew {
						t.Errorf("extracted session %q is not new", name)
					}
					if s := handler.MustExtractSessionNamed(name, r); s != session {
						t.Errorf("sessions extracted for %q don't match", name)
					}
					uniqueSessions[session] = present
				}
				if got, want := uint(len(uniqueSessions)), test.distinctCount; got != want {
					t.Errorf("unique sessions: got %d, want %d", got, want)
				}
			})
			handler := handler.WithSessionsNamed(test.names, &source, delegate, onError)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if !called {
				t.Error("delegate handler was not called")
			}
			if got, want := source.callCount(), test.distinctCount; got != want {
				t.Errorf("source call count: got %d, want %d", got, want)
			}
		})
	}
}

func TestExtractSessionNamedReportsAbsence(t *testing.T) {
	if _, ok := handler.ExtractSessionNamed("nonexistent", httptest.NewRequest("", "/", nil)); ok {
		t.Fatal("got true, want false")
	}
}

func TestMustExtractSessionNamedPanics(t *testing.T) {
	r := httptest.NewRequest("", "/", nil)
	defer ensurePanicWithValueOccured(t)
	handler.MustExtractSessionNamed("nonexistent", r)
}
