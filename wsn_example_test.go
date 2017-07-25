// Copyright 2017 Steven E. Harris. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

package handler_test

import (
	"fmt"
	"net/http"

	"github.com/seh/handler"
)

var sessionNames = []string{"s1", "s2"}

func consumeSessions(w http.ResponseWriter, r *http.Request) {
	converseIn(w, "text/plain")
	for _, name := range sessionNames {
		session := handler.MustExtractSessionNamed(name, r)
		fmt.Fprintf(w, "Session registered with name %q is named %q.\n", name, session.Name())
	}
}

func ExampleWithSessionsNamed() {
	onError := func(w http.ResponseWriter, r *http.Request, name string, err error) {
		w.WriteHeader(http.StatusInternalServerError)
		converseIn(w, "text/plain")
		fmt.Fprintf(w, "Failed to allocate a session named %q for this request: %v\n", name, err)
	}

	wrapped := handler.WithSessionsNamed(sessionNames, makeStore(), http.HandlerFunc(consumeSessions), onError)
	serveRequestAndPrintResponseBody(wrapped)
	// Output:
	// Session registered with name "s1" is named "s1".
	// Session registered with name "s2" is named "s2".
}
