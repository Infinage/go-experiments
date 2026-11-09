package com.irgo.example

import android.os.Bundle
import com.irgo.IrgoActivity
import mobile.Mobile

class MainActivity : IrgoActivity() {
    // IrgoActivity handles everything:
    // - Initializing the Go bridge
    // - Setting up the WebView
    // - Loading the initial page
    // - Handling back navigation

    override fun onCreate(savedInstanceState: Bundle?) {
        // Must run BEFORE IrgoActivity.onCreate initializes the Go bridge:
        // Android cwd is not writable, so the POC db must live in app storage.
        Mobile.setDBPath(filesDir.absolutePath + "/irgo.db")
        super.onCreate(savedInstanceState)
    }
}
