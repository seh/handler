// Copyright 2017 Steven E. Harris. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

package handler_test

import (
	"fmt"
	"net/http"

	"github.com/seh/handler"
)

func consumeSession(w http.ResponseWriter, r *http.Request) {
	session := handler.MustExtractSession(r)
	converseIn(w, "text/plain")
	fmt.Fprintf(w, "Hello, session %q!\n", session.Name())
}

func ExampleWithSession() {
	onError := func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusInternalServerError)
		converseIn(w, "text/plain")
		fmt.Fprintf(w, "Failed to allocate a session for this request: %v\n", err)
	}

	wrapped := handler.WithSession("s", makeStore(), http.HandlerFunc(consumeSession), onError)
	serveRequestAndPrintResponseBody(wrapped)
	// Output:
	// Hello, session "s"!
}
