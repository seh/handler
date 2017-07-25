// Copyright 2017 Steven E. Harris. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

package handler_test

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/gorilla/sessions"
)

func randomByteVector(length int) []byte {
	v := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, v); err != nil {
		return nil
	}
	return v
}

func mustRandomByteVector(length int) []byte {
	if v := randomByteVector(length); v != nil {
		return v
	}
	fmt.Fprintf(os.Stderr, "Failed to generate random byte vector of length %d.\n", length)
	os.Exit(1)
	panic("unreachable")
}

func makeStore() sessions.Store {
	authKey := mustRandomByteVector(32)
	encryptionKey := mustRandomByteVector(64)
	return sessions.NewCookieStore(authKey, encryptionKey)
}

func converseIn(w http.ResponseWriter, contentType string) http.Header {
	h := w.Header()
	h.Set("Content-Type", contentType)
	return h
}

func serveRequestAndPrintResponseBody(h http.Handler) {
	ts := httptest.NewServer(h)
	defer ts.Close()

	res, err := http.Get(ts.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to issue request against HTTP server: %v\n", err)
		os.Exit(1)
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read HTTP response body: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(body)
}
