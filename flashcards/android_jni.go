//go:build android

// JNI bridge for full-fidelity asset serving on Android.
//
// Wails' Java_com_wails_app_WailsBridge_nativeServeAsset buffers our handler's
// response into a []byte, drops the headers, and the Java side then guesses
// MIME from the file extension. Extensionless routes (datastar SSE fragments,
// captured media) end up as application/octet-stream and datastar silently
// discards the body. It also strips the query string datastar signals ride on.
//
// This file exports nativeServeAssetFull: it drives the SAME handler
// directly (via the assetHandler var in main.go, no wails internals needed)
// and returns a JSON envelope {status, contentType, body(B64), reason} built
// from our real response, so the Java layer can emit a truthful
// WebResourceResponse. Query strings survive intact. There is no double
// execution — one recorder pass per request.
//
// JNI plumbing follows the pattern of wails' application_android.go.
package main

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

// Copy a jstring into a C string; the caller frees the result.
static char *jniGetUTF(JNIEnv *env, jstring s) {
    if (s == NULL) return NULL;
    const char *c = (*env)->GetStringUTFChars(env, s, NULL);
    if (c == NULL) return NULL;
    char *out = strdup(c);
    (*env)->ReleaseStringUTFChars(env, s, c);
    return out;
}

// Wrap a C string in a jstring; NULL input → NULL.
static jstring jniNewUTF(JNIEnv *env, const char *c) {
    if (c == NULL) return NULL;
    return (*env)->NewStringUTF(env, c);
}
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"unsafe"
)

// ctRecorder captures the Content-Type as of the handler's FIRST body
// write, before httptest's single-chunk sniffing can pollute the header map
// for handlers that never declare one.
type ctRecorder struct {
	*httptest.ResponseRecorder
	declaredCT string
	sawBody    bool
}

func (r *ctRecorder) Write(b []byte) (int, error) {
	if !r.sawBody {
		r.sawBody = true
		r.declaredCT = r.Header().Get("Content-Type")
	}
	return r.ResponseRecorder.Write(b)
}

// serveAssetViaHandler drives assetHandler and returns the JSON envelope
// {status, contentType, body(B64), reason, location}. Body can be http.NoBody
// (GET fragments) or a real reader (staging POSTs from the JS bridge).
func serveAssetViaHandler(method, path string, bodyReader io.Reader,
	contentType string) (string, error) {
	if assetHandler == nil {
		log.Printf("[JNI] serveAsset: handler not initialized for %s", path)
		return ``, fmt.Errorf("handler not initialized")
	}
	req, err := http.NewRequest(method, "https://wails.localhost"+path, bodyReader)
	if err != nil {
		log.Printf("[JNI] serveAsset: bad request %s: %v", path, err)
		return ``, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// Wrap the recorder so we can tell a handler-declared Content-Type apart
	// from httptest's own sniffing. httptest sniffs on the FIRST body write
	// only: a template whose first chunk is only whitespace (e.g.
	// "\n  <!DOCTYPE html>" arrives in later chunks) gets mislabeled
	// text/plain. The real net/http server buffers up to 512 bytes across
	// writes before sniffing, so production behaves correctly; we mirror
	// that here by re-sniffing the accumulated body when the handler never
	// declared a type.
	rec := &ctRecorder{ResponseRecorder: httptest.NewRecorder()}
	assetHandler.ServeHTTP(rec, req)

	body := rec.Body.Bytes()
	ct := rec.declaredCT
	if ct == "" && len(body) > 0 {
		n := len(body)
		if n > 512 {
			n = 512
		}
		ct = http.DetectContentType(body[:n])
	}
	loc := rec.Header().Get("Location")
	if loc != "" {
		absURL, _ := req.URL.Parse(loc)
		loc = pathFromUrl(absURL)
	}
	log.Printf("[JNI] serveAsset: %s %s -> status=%d CT=%q len=%d location=%q",
		method, path, rec.Code, ct, len(body), loc)
	out, err := json.Marshal(map[string]any{
		"status":      rec.Code,
		"contentType": ct,
		"body":        base64.StdEncoding.EncodeToString(body),
		"reason":      fmt.Sprintf("%d %s", rec.Code, http.StatusText(rec.Code)),
		"location":    loc,
	})
	return string(out), err
}

// pathFromUrl strips a resolved redirect URL back down to the app-relative
// path the webview can navigate to (the JS/Java side reattaches the host).
func pathFromUrl(u *url.URL) string {
	if u == nil {
		return "/"
	}
	p := u.Path
	if p == "" {
		p = "/"
	}
	return p
}

//export Java_com_wails_app_WailsBridge_nativeServeAssetFull
func Java_com_wails_app_WailsBridge_nativeServeAssetFull(env *C.JNIEnv, cls C.jobject, jpath C.jstring, jmethod C.jstring) C.jstring {
	cPath := C.jniGetUTF(env, jpath)
	if cPath == nil {
		return C.jniNewUTF(env, C.CString(`{"status":500,"contentType":"text/plain","body":""}`))
	}
	defer C.free(unsafe.Pointer(cPath))
	cMethod := C.jniGetUTF(env, jmethod)
	if cMethod == nil {
		cMethod = C.CString("GET")
	} else {
		defer C.free(unsafe.Pointer(cMethod))
	}
	path := C.GoString(cPath)
	method := C.GoString(cMethod)
	if method == "" {
		method = http.MethodGet
	}

	out, err := serveAssetViaHandler(method, path, http.NoBody, "")
	if err != nil {
		out = `{"status":500,"contentType":"text/plain","body":""}`
	}
	cs := C.CString(out)
	defer C.free(unsafe.Pointer(cs))
	return C.jniNewUTF(env, cs)
}

//export Java_com_wails_app_WailsBridge_nativeServeAssetBody
// nativeServeAssetBody serves one app request WITH the body Android's
// shouldInterceptRequest cannot provide (requests bodies are not passed to
// WebResourceResponse intercepts at all). The Java bridge hands over
// {path, contentType, bodyB64} — mediapick.js stage() uses this on Android —
// and the exact same Go handler runs, so web and device share the route.
func Java_com_wails_app_WailsBridge_nativeServeAssetBody(env *C.JNIEnv, cls C.jobject, jpayload C.jstring) C.jstring {
	resp500 := `{"status":500,"contentType":"text/plain","body":""}`
	cPayload := C.jniGetUTF(env, jpayload)
	if cPayload == nil {
		return C.jniNewUTF(env, C.CString(resp500))
	}
	defer C.free(unsafe.Pointer(cPayload))

	var payload struct {
		Path        string `json:"path"`
		ContentType string `json:"contentType"`
		Head        string `json:"head"`
		BodyB64     string `json:"bodyB64"`
		Tail        string `json:"tail"`
	}
	if err := json.Unmarshal([]byte(C.GoString(cPayload)), &payload); err != nil || payload.Path == "" {
		log.Printf("[JNI] serveAssetBody: bad payload: %v", err)
		return C.jniNewUTF(env, C.CString(resp500))
	}
	fileBytes, err := base64.StdEncoding.DecodeString(payload.BodyB64)
	if err != nil {
		log.Printf("[JNI] serveAssetBody: bad body b64: %v", err)
		return C.jniNewUTF(env, C.CString(resp500))
	}
	// Reassemble the multipart body exactly as the browser would have sent
	// it: framing head, raw file bytes, closing tail.
	body := make([]byte, 0, len(payload.Head)+len(fileBytes)+len(payload.Tail))
	body = append(body, payload.Head...)
	body = append(body, fileBytes...)
	body = append(body, payload.Tail...)

	out, err := serveAssetViaHandler(http.MethodPost, payload.Path,
		bytes.NewReader(body), payload.ContentType)
	if err != nil {
		out = resp500
	}
	cs := C.CString(out)
	defer C.free(unsafe.Pointer(cs))
	return C.jniNewUTF(env, cs)
}
