# Restgrep: Chromium Examples (GitHub)

// Copyright 2026 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

This file documents examples of `restgrep` running against the **Chromium** repository via the GitHub CLI backend.

## Live Repository Examples (GitHub)

The following examples demonstrate `restgrep` running against the live **Chromium** repository via the GitHub CLI backend.

*Note: The code snippets and file paths below are Copyright (c) The Chromium Authors and are used for demonstration purposes.*

### Substring Search

Search for a class name across the repository:

```bash
restgrep "WebContentsImpl"
```
**Output:**
```text
content/browser/web_contents/web_contents_view_ios.h:class WebContentsImpl;
content/browser/browser_plugin/browser_plugin_embedder.h:class WebContentsImpl;
content/browser/picture_in_picture/picture_in_picture_session.h:class WebContentsImpl;
content/browser/web_contents/aura/gesture_nav_simple.h:class WebContentsImpl;
```

### Case-Insensitive Search (`-i`)

Useful for finding macros or variations of a term.

```bash
restgrep -i "NavigationHandle"
```
**Output:**
```text
chrome/browser/ui/tab_ui_helper.h:class NavigationHandle;
chrome/browser/chromeos/cros_apps/cros_apps_tab_helper.cc:content::NavigationHandle* navigation_handle) {
android_webview/java/src/org/chromium/android_webview/AwContents.java:import org.chromium.content_public.browser.NavigationHandle;
docs/navigation_concepts.md:code is available and `NavigationHandle::IsErrorPage()` is true.
```

### Exact Word Matching (`-w`)

Match only the whole word `WebContents`, ignoring `WebContentsImpl`:

```bash
restgrep -w "WebContents"
```
**Output:**
```text
chrome/browser/web_applications/locks/lock.cc:return "WebContents";
chrome/test/interaction/interactive_browser_test_interactive_uitest.cc:const char kWebContentsName[] = "WebContents";
tools/metrics/histograms/metadata/accessibility/histograms.xml:<variant name="WebContents" summary="The API was called on web contents"/>
```

### Showing Match Counts per File (`-c`)

Find which files reference a core primitive most frequently:

```bash
restgrep -c "RenderFrameHost"
```
**Output:**
```text
android_webview/browser/aw_contents.cc:3
chrome/browser/ui/hid/hid_chooser_controller.cc:3
content/browser/renderer_host/cookie_utils.cc:2
content/public/browser/document_ref.h:2
```

### Showing Only Filenames (`-l`)

List all files that use a specific class:

```bash
restgrep -l "WebContentsImpl"
```
**Output:**
```text
content/public/android/java/src/org/chromium/content/browser/webcontents/WebContentsImpl.java
content/browser/host_zoom_map_impl.h
content/browser/web_contents/web_contents_view_ios.h
content/browser/web_contents/aura/gesture_nav_simple.h
```

## GitHub API Backend Examples

The `github-api` backend uses `gh api` to call the GitHub Search API directly with the `text-match` header.

### Searching for Omnibox (direct API)

```bash
restgrep "OmniboxEd"
```
*(Assuming github-api is configured for chromium/chromium)*

**Output:**
```text
chrome/browser/ui/omnibox/omnibox_edit_model.h:class OmniboxEditModel;
chrome/browser/ui/views/omnibox/omnibox_view_views.h:class OmniboxViewViews;
```

## Simulated Chromium Matches

The following examples demonstrate standard `grep` flags using simulated Chromium code data.

### Showing Line Numbers (`-n`)

```bash
restgrep -n "HttpRequestInfo"
```
**Output:**
```text
src/net/http/http_request_info.h:6:struct HttpRequestInfo {
src/net/http/http_request_info.h:7:  HttpRequestInfo();
src/net/http/http_request_info.h:8:  ~HttpRequestInfo();
```

### Case-Insensitive Search (`-i`) (Simulated)

```bash
restgrep -i "INITRENDErviewHOST"
```
**Output:**
```text
src/content/browser/web_contents/web_contents_impl.cc:  InitRenderViewHost();
src/content/browser/web_contents/web_contents_impl.cc:void WebContentsImpl::InitRenderViewHost() {
```
