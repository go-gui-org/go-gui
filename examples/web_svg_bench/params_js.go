//go:build js && wasm

package main

import "syscall/js"

// queryParam returns the page URL's query parameter name, or "".
func queryParam(name string) string {
	search := js.Global().Get("location").Get("search")
	v := js.Global().Get("URLSearchParams").New(search).Call("get", name)
	if v.Type() != js.TypeString {
		return ""
	}
	return v.String()
}
