package com.irgo

import android.net.Uri
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import java.io.ByteArrayInputStream

/**
 * Custom WebViewClient that intercepts requests and routes them to Go
 */
open class IrgoWebViewClient : WebViewClient() {

    companion object {
        const val ASSETS_BASE = "https://appassets.androidplatform.net"
        // Chromium's fetch() only supports http(s); custom schemes are
        // rejected client-side BEFORE shouldInterceptRequest. All bridge
        // URLs therefore use the official appassets pseudo-https domain.
        const val SCHEME = "https"
        const val HOST = "appassets.androidplatform.net"
    }

    override fun shouldInterceptRequest(
        view: WebView?,
        request: WebResourceRequest?
    ): WebResourceResponse? {
        val url = request?.url ?: return null

        // Only intercept the appassets https domain
        if (url.scheme != SCHEME || url.host != HOST) {
            return super.shouldInterceptRequest(view, request)
        }

        // Convert https://appassets.androidplatform.net/path?query -> /path?query
        var path = url.path ?: "/"
        if (path.isEmpty()) {
            path = "/"
        }
        url.query?.let { query ->
            if (query.isNotEmpty()) {
                path += "?$query"
            }
        }

        // Get HTTP method
        val method = request.method ?: "GET"

        // Get headers
        val headers = request.requestHeaders ?: emptyMap()

        // Handle request (this runs on WebView thread, which is fine for our use case)
        val response = IrgoBridge.handleRequest(
            method = method,
            url = path,
            headers = headers,
            body = null // WebResourceRequest doesn't provide body access
        )

        // Determine MIME type
        val mimeType = response.headers["Content-Type"] ?: "text/html"

        // Create response
        return WebResourceResponse(
            mimeType.split(";").first().trim(),
            "UTF-8",
            response.status,
            if (response.status < 400) "OK" else "Error",
            response.headers,
            ByteArrayInputStream(response.body)
        )
    }

    override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?): Boolean {
        val url = request?.url ?: return false

        // Allow appassets scheme
        if (url.scheme == SCHEME && url.host == HOST) {
            return false // Let WebView handle it (will be intercepted)
        }

        // For external URLs, could open in browser
        // For now, allow them
        return false
    }

    override fun onPageFinished(view: WebView?, url: String?) {
        super.onPageFinished(view, url)
        // Page loaded successfully
    }
}
