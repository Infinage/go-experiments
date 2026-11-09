//go:build android

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func init() {
	// libwails.so is a c-shared library loaded by the Android app process;
	// main() is not called on its own on Android, so register it with the
	// Wails runtime instead (the equivalent upstream artifact of
	// `wails3 android overlay:gen`, committed to the repo so the build needs
	// no wails3 install).
	application.RegisterAndroidMain(main)
}
