package main

import (
	"io"
	"net/http"
)

func readBody(r *http.Request) ([]byte, error) { return io.ReadAll(r.Body) }
