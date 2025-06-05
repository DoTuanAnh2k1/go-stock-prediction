package server

import "net/http"

func addHandler() *http.ServeMux {
	mux := http.NewServeMux()

	// add handlers here

	return mux
}
