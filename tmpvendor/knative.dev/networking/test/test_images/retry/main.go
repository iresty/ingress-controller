/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"net/http"
	"os"

	network "knative.dev/networking/pkg"
	"knative.dev/networking/test"
)

var retries = 0

func handler(w http.ResponseWriter, r *http.Request) {
	if retries == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	fmt.Fprintf(w, "Retry %d", retries)
	retries++
}

func main() {
	h := network.NewProbeHandler(http.HandlerFunc(handler))
	test.ListenAndServeGracefully(":"+os.Getenv("PORT"), h.ServeHTTP)
}
