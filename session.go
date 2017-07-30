// Copyright 2017 Steven E. Harris. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

package handler

import (
	"context"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// SessionSource supplies fresh sessions on demand.
//
// Note that it's an intentional subset of the gorilla/sessions.Store interface; any Store is a
// SessionSource.
type SessionSource interface {
	// New creates a new session, or returns an error if unable to do so successfully.
	New(r *http.Request, name string) (*sessions.Session, error)
}

func getValidOrNewSessionFrom(name string, s SessionSource, r *http.Request) (*sessions.Session, error) {
	session, err := s.New(r, name)
	if err != nil && err != http.ErrNoCookie {
		if serr, ok := err.(securecookie.Error); !ok || !serr.IsDecode() {
			return session, err
		}
	}
	return session, nil
}

func makeSingleKeyHandler(name string, contextKey interface{}, s SessionSource, h http.Handler, onError func(w http.ResponseWriter, r *http.Request, err error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := getValidOrNewSessionFrom(name, s, r)
		if err != nil {
			onError(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), contextKey, session)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractSession(contextKey interface{}, r *http.Request) (s *sessions.Session, ok bool) {
	if v := r.Context().Value(contextKey); v != nil {
		s = v.(*sessions.Session)
		if s != nil {
			ok = true
		}
	}
	return
}

func sendDefaultResponse(w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
}

type sessionContextKey struct{}

// WithSession returns an HTTP handler that binds a session with the given name to each submitted
// request, delegating further request processing to the supplied HTTP handler, which can then
// retrieve this bound session with either ExtractSession or MustExtractSession. It panics if either
// the supplied SessionSource or handler is nil. If the SessionSource yields an error instead of a
// session, it delegates further request processing to the onError handler. If no such onError
// handler is supplied and an error arises acquiring a session, it will respond with HTTP status
// code 500 with no body.
//
// Note that even though this bound session has a name, supplied for consumption by the
// SessionSource, WithSession binds at most one session to a given request (as an anonymous
// singleton, shadowing any prior sessions bound similarly), making storage and later retrieval of
// the session more efficient than the multiple sessions that the similar WithSessionsNamed
// binds. To bind multiple sessions with different names to a given request, use WithSessionsNamed
// instead.
func WithSession(name string, s SessionSource, h http.Handler, onError func(w http.ResponseWriter, r *http.Request, err error)) http.Handler {
	if s == nil {
		panic("no session source supplied")
	}
	if h == nil {
		panic("no consuming HTTP handler supplied")
	}
	if onError == nil {
		onError = func(w http.ResponseWriter, _ *http.Request, _ error) { sendDefaultResponse(w) }
	}
	return makeSingleKeyHandler(name, sessionContextKey{}, s, h, onError)
}

// ExtractSession retrieves the singular session most recently bound to this request via
// WithSession, together with a boolean indicating whether such a session is available.
func ExtractSession(r *http.Request) (s *sessions.Session, ok bool) {
	return extractSession(sessionContextKey{}, r)
}

// MustExtractSession retrieves the singular session most recently bound to this request via
// WithSession, or panics if no such session is available.
func MustExtractSession(r *http.Request) *sessions.Session {
	if s, ok := ExtractSession(r); ok {
		return s
	}
	panic("no session available")
}

type namedSessionContextKey string

// WithSessionsNamed returns an HTTP handler that binds any number of sessions with the given set of
// names to each submitted request, delegating further request processing to the supplied HTTP
// handler, which can then retrieve these bound sessions with either ExtractSessionNamed or
// MustExtractSessionNamed. It panics if either the supplied SessionSource or handler is nil. If the
// SessionSource yields an error instead of a session, it delegates further request processing to
// the onError handler. If no such onError handler is supplied and an error arises acquiring a
// session, it will respond with HTTP status code 500 with no body.
//
// It reduces the sequence of names supplied to a set, with no duplicate entries, but it does not
// mutate the supplied slice in place. If no names are supplied, it returns the supplied HTTP
// handler.
//
// To bind only a single session to a given request, consider using WithSession instead.
func WithSessionsNamed(names []string, s SessionSource, h http.Handler, onError func(w http.ResponseWriter, r *http.Request, name string, err error)) http.Handler {
	if s == nil {
		panic("no session source supplied")
	}
	if h == nil {
		panic("no consuming HTTP handler supplied")
	}
	if onError == nil {
		onError = func(w http.ResponseWriter, _ *http.Request, _ string, _ error) { sendDefaultResponse(w) }
	}
	// If there is more than one name supplied, whittle them down to a set, without bothering to
	// preserve order.
	switch len(names) {
	case 0:
		return h
	case 1:
		goto single
	case 2:
		if names[0] == names[1] {
			names = names[:1]
		}
	default:
		// Assume that we can't mutate "names" in place, or we could sort it and then eliminate
		// adjacent duplicates.
		m := make(map[string]struct{}, len(names))
		present := struct{}{}
		for _, n := range names {
			m[n] = present
		}
		switch n := len(m); {
		case n == 1: // All duplicates.
			goto single
		case n != len(names): // At least one duplicate.
			names = make([]string, n)
			i := 0
			for k := range m {
				names[i] = k
				i++
			}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		for _, name := range names {
			session, err := getValidOrNewSessionFrom(name, s, r)
			if err != nil {
				onError(w, r, name, err)
				return
			}
			ctx = context.WithValue(ctx, namedSessionContextKey(name), session)
		}
		h.ServeHTTP(w, r.WithContext(ctx))
	})

single:
	name := names[0]
	return makeSingleKeyHandler(name, namedSessionContextKey(name), s, h,
		func(w http.ResponseWriter, r *http.Request, err error) { onError(w, r, name, err) })
}

// ExtractSessionNamed retrieves the session most recently bound to this request with the given name
// via WithSessionsNamed, together with a boolean indicating whether such a session is available.
func ExtractSessionNamed(name string, r *http.Request) (s *sessions.Session, ok bool) {
	return extractSession(namedSessionContextKey(name), r)
}

// MustExtractSessionNamed retrieves the session most recently bound to this request with the given
// name via WithSessionsNamed, or panics if no such session is available.
func MustExtractSessionNamed(name string, r *http.Request) *sessions.Session {
	if s, ok := ExtractSessionNamed(name, r); ok {
		return s
	}
	panic("no session available")
}
